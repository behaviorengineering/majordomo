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

	"github.com/behaviorengineering/majordomo/internal/config"
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
	ledgerRLMTimeout = 8 * time.Minute
)

var objectiveVerdictRE = regexp.MustCompile(`(?i)\bverdict\s*[:=]\s*(grounded|overclaim)\b`)

// sliceObjectiveLedgerBuilder writes evidence-first objectives before refine assemble.
type sliceObjectiveLedgerBuilder interface {
	BuildSliceLedger(ctx context.Context, req sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error)
}

type sliceLedgerBuildRequest struct {
	AnalysisDir string
	EvidenceDir string
	DraftTypo   catalog.Typology
	Constraints packageCapabilityConstraintsDoc
	ClusterMD   string
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
	var targets []target
	for _, s := range req.DraftTypo.Slices {
		id := strings.TrimSpace(s.ID)
		paths := slicePackagePaths(s)
		if id == "" || len(paths) == 0 {
			continue
		}
		targets = append(targets, target{id: id, paths: paths})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].id < targets[j].id })
	if len(targets) == 0 {
		return sliceObjectiveLedgerDoc{}, nil, nil
	}
	if len(targets) > maxLedgerSlices {
		targets = targets[:maxLedgerSlices]
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

	type result struct {
		entry sliceObjectiveLedgerEntry
		issue string
		err   error
	}
	results := make([]result, len(targets))
	sem := make(chan struct{}, ledgerRLMWorkers)
	var wg sync.WaitGroup
	for i, t := range targets {
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
			query := formatSliceObjectiveLedgerQuery(t.id, t.paths, constraintBlock, req.ClusterMD)
			answer, _, err := caller.Complete(ctx, strings.Join(parts, "\n\n"), query)
			if err != nil {
				results[i] = result{err: fmt.Errorf("%s: slice %q objective ledger RLM failed: %w",
					typologypack.CriterionIDRoleGrounding, t.id, err)}
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
			results[i] = result{entry: entry}
		}(i, t)
	}
	wg.Wait()

	var issues []string
	var slices []sliceObjectiveLedgerEntry
	for _, r := range results {
		if r.err != nil {
			return sliceObjectiveLedgerDoc{}, nil, r.err
		}
		if strings.TrimSpace(r.issue) != "" {
			issues = append(issues, r.issue)
			continue
		}
		if strings.TrimSpace(r.entry.ID) != "" {
			slices = append(slices, r.entry)
		}
	}
	if len(issues) > 0 {
		return sliceObjectiveLedgerDoc{}, issues, nil
	}
	sort.Slice(slices, func(i, j int) bool { return slices[i].ID < slices[j].ID })
	doc := sliceObjectiveLedgerDoc{Slices: slices}
	if err := validateObjectiveLedgerDoc(doc); err != nil {
		return sliceObjectiveLedgerDoc{}, nil, err
	}
	if consIssues := validateLedgerAgainstConstraints(doc, req.Constraints, rolesDoc); len(consIssues) > 0 {
		return sliceObjectiveLedgerDoc{}, consIssues, nil
	}
	return doc, nil, nil
}

func formatSliceObjectiveLedgerQuery(sliceID string, paths []string, constraintBlock, clusterMD string) string {
	clusterNote := strings.TrimSpace(clusterMD)
	if len(clusterNote) > 4000 {
		clusterNote = clusterNote[:4000] + "\n[... cluster proposal truncated ...]\n"
	}
	return fmt.Sprintf(`You write the grounded meaning for one Typology teaching slice.
Cite evidence BEFORE claims BEFORE the objective. Path basenames are never evidence.

Slice id: %s
Owned package paths: %s

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
Claims MUST NOT intersect owned must_not codes in the constraint rows.`,
		sliceID, strings.Join(paths, ", "), constraintBlock, clusterNote)
}
