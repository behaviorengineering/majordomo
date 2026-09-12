package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/majordomo/internal/llmusage"
	stropdspy "github.com/behaviorengineering/strop/dspy"
	"github.com/behaviorengineering/strop/dspy/factory"
	"github.com/behaviorengineering/typology/catalog"
	typroles "github.com/behaviorengineering/typology/roles"
)

const (
	maxLedgerSlices  = 24
	ledgerRLMWorkers = 2
	ledgerRLMTimeout = 15 * time.Minute
)

var objectiveVerdictRE = regexp.MustCompile(`(?i)\bverdict\s*[:=]\s*(grounded|overclaim)\b`)

// sliceObjectiveLedgerBuilder writes evidence-first objectives before refine assemble.
type sliceObjectiveLedgerBuilder interface {
	BuildSliceLedger(ctx context.Context, req sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error)
}

type sliceLedgerBuildRequest struct {
	AnalysisDir   string
	EvidenceDir   string
	DraftTypo     catalog.Typology
	Constraints   packageCapabilityConstraintsDoc
	ClusterMD     string
	DigestCache   *cache.DigestStore
	DigestSkips   bool
	DigestModelID string
}

type stropSliceObjectiveLedgerRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations int, err error)
	}
}

func newStropSliceObjectiveLedgerRLM(ctx context.Context, cfg config.RepoConfig) (sliceObjectiveLedgerBuilder, error) {
	provider, ok, err := cfg.ResolveTaskProvider(jmodules.TaskTypologyObjectiveGrounding)
	if err != nil || !ok {
		// Fall back to typology_inspect when the dedicated task is unset or unknown.
		provider, ok, err = cfg.ResolveTaskProvider(jmodules.TaskTypologyInspect)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("typology_objective_grounding provider not configured")
		}
	}
	stropProvider := provider.ToStrop()
	llmFactory := factory.NewLLMFactory(nil, ledgerRLMTimeout)
	llm, err := llmFactory.CreateLLM(ctx, stropProvider)
	if err != nil {
		return nil, fmt.Errorf("typology_objective_grounding RLM LLM: %w", err)
	}
	llm = judge.WrapLLMWithRetry(llm, judge.DefaultModuleRetryConfig())
	rlmCfg := stropdspy.RLMDefaults()
	rlmCfg.MaxFullContextQueryChars = 24_000
	timeout := provider.GetTimeout(ledgerRLMTimeout)
	if timeout < ledgerRLMTimeout {
		timeout = ledgerRLMTimeout
	}
	rlmCfg.Timeout = timeout
	module, err := stropdspy.CreateRLMModule(llm, rlmCfg)
	if err != nil {
		return nil, err
	}
	return stropSliceObjectiveLedgerRLM{
		module: rlmCompleteAdapter{complete: func(ctx context.Context, contextPayload any, query string) (string, int, error) {
			answer, result, err := stropdspy.RLMComplete(ctx, module, contextPayload, query)
			if err != nil {
				return "", 0, err
			}
			iters := 0
			if result != nil {
				iters = result.Iterations
				llmusage.FromContext(ctx).AddTokenUsageValue(jmodules.TaskTypologyObjectiveGrounding, result.Usage)
			} else {
				llmusage.FromContext(ctx).Add(jmodules.TaskTypologyObjectiveGrounding, 0, 0, 0)
			}
			return answer, iters, nil
		}},
	}, nil
}

func (v stropSliceObjectiveLedgerRLM) BuildSliceLedger(ctx context.Context, req sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error) {
	return buildSliceObjectiveLedger(ctx, v, req)
}

type sliceLedgerRLMCaller interface {
	Complete(ctx context.Context, contextPayload any, query string) (response string, iterations int, err error)
}

func (v stropSliceObjectiveLedgerRLM) Complete(ctx context.Context, contextPayload any, query string) (string, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}

func newLedgerBuilderFromOpts(ctx context.Context, opts Options) (sliceObjectiveLedgerBuilder, error) {
	if strings.TrimSpace(opts.ConfigDir) == "" || strings.TrimSpace(opts.RepoID) == "" {
		return nil, fmt.Errorf("config-dir and repo-id required for objective ledger RLM")
	}
	defaults, err := config.LoadDefaults(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadRepoFile(opts.ConfigDir, opts.RepoID, defaults)
	if err != nil {
		return nil, err
	}
	return newStropSliceObjectiveLedgerRLM(ctx, cfg)
}

type stubSliceLedgerCaller struct {
	answer string
	err    error
}

func (s stubSliceLedgerCaller) Complete(context.Context, any, string) (string, int, error) {
	return s.answer, 1, s.err
}

func buildSliceObjectiveLedger(
	ctx context.Context,
	caller sliceLedgerRLMCaller,
	req sliceLedgerBuildRequest,
) (sliceObjectiveLedgerDoc, []string, error) {
	if caller == nil {
		return sliceObjectiveLedgerDoc{}, []string{fmt.Sprintf(
			"%s: slice_objective_ledger RLM is required for owned packages",
			typologypack.CriterionIDRoleGrounding,
		)}, nil
	}
	byPath := constraintsByPath(req.Constraints)
	type target struct {
		id    string
		paths []string
	}
	var allTargets []target
	for _, s := range req.DraftTypo.Slices {
		id := strings.TrimSpace(s.ID)
		paths := slicePackagePaths(s)
		if id == "" || len(paths) == 0 {
			continue
		}
		allTargets = append(allTargets, target{id: id, paths: paths})
	}
	sort.Slice(allTargets, func(i, j int) bool { return allTargets[i].id < allTargets[j].id })
	if len(allTargets) == 0 {
		return sliceObjectiveLedgerDoc{}, nil, nil
	}
	if len(allTargets) > maxLedgerSlices {
		allTargets = allTargets[:maxLedgerSlices]
	}

	rolesDoc, err := loadPackageRolesFromEvidenceDir(req.EvidenceDir)
	if err != nil {
		return sliceObjectiveLedgerDoc{}, nil, fmt.Errorf("%s: %w", typologypack.CriterionIDRoleGrounding, err)
	}

	rlmContextPath := filepath.Join(req.EvidenceDir, "package_rlm_context.md")
	wholeContext, readErr := os.ReadFile(rlmContextPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return sliceObjectiveLedgerDoc{}, nil, fmt.Errorf("%s: read package_rlm_context.md: %w", typologypack.CriterionIDRoleGrounding, readErr)
	}

	kept := map[string]sliceObjectiveLedgerEntry{}
	var lastIssues []string
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		var pending []target
		for _, t := range allTargets {
			if _, ok := kept[t.id]; ok {
				continue
			}
			pending = append(pending, t)
		}
		if len(pending) == 0 {
			break
		}

		type result struct {
			entry sliceObjectiveLedgerEntry
			issue string
			err   error
		}
		results := make([]result, len(pending))
		sem := make(chan struct{}, ledgerRLMWorkers)
		var wg sync.WaitGroup
		for i, t := range pending {
			wg.Add(1)
			go func(i int, t target) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				var parts []string
				for _, p := range t.paths {
					ctxMD := packageRLMContextSnippet(string(wholeContext), p)
					if strings.TrimSpace(ctxMD) == "" {
						built, err := typroles.FormatPackageRLMContextForPath(req.AnalysisDir, p, nil)
						if err == nil {
							ctxMD = built
						}
					}
					if strings.TrimSpace(ctxMD) != "" {
						parts = append(parts, ctxMD)
					}
				}
				if len(parts) == 0 {
					results[i] = result{issue: fmt.Sprintf(
						"%s: slice %q owned packages have empty RLM context; cannot ground objective",
						typologypack.CriterionIDRoleGrounding, t.id,
					)}
					return
				}
				constraintBlock := formatConstraintRowsForPaths(t.paths, byPath)
				joinedCtx := strings.Join(parts, "\n\n")
				fp := cache.LedgerFingerprint{
					SliceID:         t.id,
					OwnedPathsHash:  cache.OwnedPathsHash(t.paths),
					ContextSHA:      cache.ContentSHA(joinedCtx),
					ConstraintsHash: cache.ContentSHA(constraintBlock),
					ClusterHash:     cache.ContentSHA(req.ClusterMD),
					ModelID:         req.DigestModelID,
					PromptVersion:   cache.DigestLedgerPromptV1,
					SchemaVersion:   cache.DigestLedgerSchemaV1,
				}
				if req.DigestSkips && req.DigestCache != nil {
					if hit, ok, err := req.DigestCache.LookupLedger(fp); err == nil && ok && hit.Verdict == ledgerVerdictGrounded {
						logf("INFO", "digest cache hit ledger slice=%s", t.id)
						results[i] = result{entry: sliceObjectiveLedgerEntry{
							ID:         hit.ID,
							OwnedPaths: append([]string(nil), hit.OwnedPaths...),
							Evidence:   append([]string(nil), hit.Evidence...),
							Claims:     append([]string(nil), hit.Claims...),
							Objective:  hit.Objective,
							Verdict:    hit.Verdict,
							Source:     firstNonEmpty(hit.Source, "digest_cache"),
						}}
						return
					}
				}
				sliceFeedback := filterIssuesForSlice(lastIssues, t.id)
				query := formatSliceObjectiveLedgerQuery(t.id, t.paths, constraintBlock, req.ClusterMD, sliceFeedback)
				answer, _, err := caller.Complete(ctx, joinedCtx, query)
				if err != nil {
					if ctx.Err() != nil {
						results[i] = result{err: fmt.Errorf("%s: slice %q objective ledger RLM failed: %w",
							typologypack.CriterionIDRoleGrounding, t.id, err)}
						return
					}
					// Provider timeouts / 502s are per-slice retry fuel, not a full abort.
					results[i] = result{issue: fmt.Sprintf(
						"%s: slice %q objective ledger RLM failed: %v",
						typologypack.CriterionIDRoleGrounding, t.id, err,
					)}
					return
				}
				evidence, claims, objective, verdict, err := parseSliceObjectiveLedgerAnswer(answer)
				if err != nil {
					results[i] = result{issue: fmt.Sprintf(
						"%s: slice %q objective ledger parse failed: %v",
						typologypack.CriterionIDRoleGrounding, t.id, err,
					)}
					return
				}
				if verdict == ledgerVerdictOverclaim {
					results[i] = result{issue: fmt.Sprintf(
						"%s: slice %q objective overclaims; cite package evidence or simplify the meaning",
						typologypack.CriterionIDRoleGrounding, t.id,
					)}
					return
				}
				entry := sliceObjectiveLedgerEntry{
					ID:         t.id,
					OwnedPaths: append([]string(nil), t.paths...),
					Evidence:   evidence,
					Claims:     claims,
					Objective:  objective,
					Verdict:    verdict,
					Source:     "slice_objective_rlm",
				}
				if hit := intersectStrings(claims, sliceMustNotUnion(t.paths, byPath)); len(hit) > 0 {
					results[i] = result{issue: fmt.Sprintf(
						"%s: slice %q ledger claims %v intersect must_not %v",
						typologypack.CriterionIDRoleGrounding, t.id, claims, hit,
					)}
					return
				}
				if entailIssues := rejectUnentailedClaims(t.id, claims, t.paths, req.Constraints, rolesDoc); len(entailIssues) > 0 {
					results[i] = result{issue: strings.Join(entailIssues, "\n")}
					return
				}
				if req.DigestCache != nil && entry.Verdict == ledgerVerdictGrounded {
					if err := req.DigestCache.StoreLedger(fp, cache.LedgerCachedEntry{
						ID:         entry.ID,
						OwnedPaths: append([]string(nil), entry.OwnedPaths...),
						Evidence:   append([]string(nil), entry.Evidence...),
						Claims:     append([]string(nil), entry.Claims...),
						Objective:  entry.Objective,
						Verdict:    entry.Verdict,
						Source:     entry.Source,
					}); err != nil {
						results[i] = result{err: fmt.Errorf("store slice %q objective ledger cache: %w", t.id, err)}
						return
					}
				}
				results[i] = result{entry: entry}
			}(i, t)
		}
		wg.Wait()

		var issues []string
		for _, r := range results {
			if r.err != nil {
				return sliceObjectiveLedgerDoc{}, nil, r.err
			}
			if strings.TrimSpace(r.issue) != "" {
				issues = append(issues, r.issue)
				continue
			}
			if id := strings.TrimSpace(r.entry.ID); id != "" {
				kept[id] = r.entry
			}
		}
		if len(issues) == 0 {
			break
		}
		lastIssues = issues
		if attempt == maxTypologyRefineAttempts {
			return sliceObjectiveLedgerDoc{}, lastIssues, nil
		}
	}

	slices := make([]sliceObjectiveLedgerEntry, 0, len(kept))
	for _, t := range allTargets {
		if e, ok := kept[t.id]; ok {
			slices = append(slices, e)
		}
	}
	doc := sliceObjectiveLedgerDoc{Slices: slices}
	if err := validateObjectiveLedgerDoc(doc); err != nil {
		return sliceObjectiveLedgerDoc{}, nil, err
	}
	if consIssues := validateLedgerAgainstConstraints(doc, req.Constraints, rolesDoc); len(consIssues) > 0 {
		return sliceObjectiveLedgerDoc{}, consIssues, nil
	}
	return doc, nil, nil
}

// filterIssuesForSlice returns prior-attempt issues that mention this slice id.
func filterIssuesForSlice(issues []string, sliceID string) string {
	sliceID = strings.TrimSpace(sliceID)
	if sliceID == "" || len(issues) == 0 {
		return ""
	}
	needle := fmt.Sprintf("slice %q", sliceID)
	var out []string
	for _, issue := range issues {
		if strings.Contains(issue, needle) {
			out = append(out, strings.TrimSpace(issue))
		}
	}
	return strings.Join(out, "\n")
}

func formatSliceObjectiveLedgerQuery(sliceID string, paths []string, constraintBlock, clusterMD, validationFeedback string) string {
	clusterNote := strings.TrimSpace(clusterMD)
	if len(clusterNote) > 4000 {
		clusterNote = clusterNote[:4000] + "\n[... cluster proposal truncated ...]\n"
	}
	feedbackBlock := ""
	if fb := strings.TrimSpace(validationFeedback); fb != "" {
		feedbackBlock = fmt.Sprintf(`
validation_feedback (from a prior failed attempt; MUST fix before emitting):
%s

`, fb)
	}
	return fmt.Sprintf(`You write the grounded meaning for one Typology teaching slice.
Cite evidence BEFORE claims BEFORE the objective. Path basenames are never evidence.

Slice id: %s
Owned package paths: %s

%s
%s
Cluster proposal (membership hint only; MUST NOT invent prestige meaning from it):
%s

Explore the package AST context. Quote symbols, delivery flags, json/yaml tags, or filled_by facts.

End with these lines in order:
evidence: <comma-separated symbol or flag quotes>
claims: <comma-separated portable codes only from: data_shape, synchronize_state, merge_adapters, serve_http, wire_handlers, orchestrate, own_domain_rules, fill_dto, run_cli, aggregate_views, exec_process, observability, adapt_external, config>
objective: <one plain sentence matching the evidence; no prestige overclaim>
verdict: grounded|overclaim

If you cannot support a runtime claim with symbols, either drop that claim or set verdict: overclaim.
Claims MUST NOT intersect owned must_not codes in the constraint rows.
When validation_feedback is present, drop or replace every claim it rejects; do not repeat the same overclaim.`,
		sliceID, strings.Join(paths, ", "), constraintBlock, feedbackBlock, clusterNote)
}
