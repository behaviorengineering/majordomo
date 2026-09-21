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

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	dspymod "github.com/XiaoConstantine/dspy-go/pkg/modules"
	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/majordomo/internal/llmusage"
	"github.com/behaviorengineering/majordomo/internal/observability"
	"github.com/behaviorengineering/strop/pkg/dspy/factory"
	"github.com/behaviorengineering/typology/pkg/catalog"
	typroles "github.com/behaviorengineering/typology/pkg/roles"
	"gopkg.in/yaml.v3"
)

const (
	maxLedgerSlices        = 24
	ledgerRLMWorkers       = 2
	ledgerRLMTimeout       = 15 * time.Minute
	ledgerEvidenceTimeout  = 90 * time.Second
	ledgerSynthesisTimeout = 5 * time.Minute
	ledgerMaxContextChars  = 24_000
	ledgerPlanRelDir       = "tmp/typology/ledger_plans"
)

var objectiveVerdictRE = regexp.MustCompile(`(?i)\bverdict\s*[:=]\s*(grounded|overclaim)\b`)

// sliceObjectiveLedgerBuilder writes evidence-first objectives before refine assemble.
type sliceObjectiveLedgerBuilder interface {
	BuildSliceLedger(ctx context.Context, req sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error)
}

type sliceLedgerBuildRequest struct {
	AnalysisDir     string
	EvidenceDir     string
	DraftTypo       catalog.Typology
	Constraints     packageCapabilityConstraintsDoc
	ClusterHintYAML string
	DigestCache     *cache.DigestStore
	DigestSkips     bool
	DigestModelID   string
}

type stropSliceObjectiveLedgerRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
}

func newObjectiveLedgerPredictModule(llm core.LLM) *dspymod.Predict {
	sig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("context", core.WithDescription("Distilled per-package evidence notes for one teaching slice"))},
			{Field: core.NewField("query", core.WithDescription("Grounding instructions, constraints, and claim policy"))},
		},
		[]core.OutputField{
			{Field: core.NewField("answer", core.WithDescription("YAML ledger object: verdict, evidence, claims, objective"))},
		},
	).WithInstruction(`You write one grounded Typology teaching-slice ledger entry from distilled evidence.
Follow the query exactly. Emit only the YAML object described there (no markdown fences, no REPL, no tool use).`)
	predict := dspymod.NewPredict(sig).WithName(jmodules.TaskTypologySliceMeaning)
	predict.SetLLM(llm)
	return predict
}

func newStropSliceObjectiveLedgerRLM(ctx context.Context, cfg config.RepoConfig, workStoryDir string) (sliceObjectiveLedgerBuilder, error) {
	provider, ok, err := cfg.ResolveTaskProvider(jmodules.TaskTypologySliceMeaning)
	if err != nil || !ok {
		// Fall back to typology_inspect when the dedicated task is unset or unknown.
		provider, ok, err = cfg.ResolveTaskProvider(jmodules.TaskTypologyInspect)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("typology_slice_meaning provider not configured")
		}
	}
	stropProvider := provider.ToStrop()
	timeout := provider.GetTimeout(ledgerSynthesisTimeout)
	if timeout < ledgerSynthesisTimeout {
		timeout = ledgerSynthesisTimeout
	}
	llmFactory := factory.NewLLMFactory(nil, timeout)
	llmFactory.SetInstrumentHTTP(observability.InstrumentHTTPClient)
	llm, err := llmFactory.CreateLLM(ctx, stropProvider)
	if err != nil {
		return nil, fmt.Errorf("typology_slice_meaning Predict LLM: %w", err)
	}
	llm = judge.WrapLLMWithRetry(llm, judge.DefaultModuleRetryConfig())
	module := newObjectiveLedgerPredictModule(llm)
	_ = workStoryDir
	return stropSliceObjectiveLedgerRLM{
		module: rlmCompleteAdapter{complete: func(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
			ctxStr, _ := contextPayload.(string)
			if ctxStr == "" && contextPayload != nil {
				ctxStr = fmt.Sprint(contextPayload)
			}
			stepCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			out, err := module.Process(stepCtx, map[string]any{
				"context": ctxStr,
				"query":   query,
			})
			if err != nil {
				return "", 0, 0, 0, 0, err
			}
			answer := ""
			if out != nil {
				if s, ok := out["answer"].(string); ok {
					answer = s
				} else if s, ok := out["Answer"].(string); ok {
					answer = s
				} else if s, ok := out["completion"].(string); ok {
					answer = s
				}
			}
			llmusage.FromContext(ctx).Add(jmodules.TaskTypologySliceMeaning, 0, 0, 0)
			return strings.TrimSpace(answer), 1, 0, 0, 0, nil
		}},
	}, nil
}

func (v stropSliceObjectiveLedgerRLM) BuildSliceLedger(ctx context.Context, req sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error) {
	return buildSliceObjectiveLedger(ctx, v, req)
}

type sliceLedgerRLMCaller interface {
	Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
}

func (v stropSliceObjectiveLedgerRLM) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}

func newLedgerBuilderFromOpts(ctx context.Context, opts Options, analysisDir string) (sliceObjectiveLedgerBuilder, error) {
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
	return newStropSliceObjectiveLedgerRLM(ctx, cfg, inferenceWorkRoot(opts, analysisDir))
}

type stubSliceLedgerCaller struct {
	answer string
	err    error
}

func (s stubSliceLedgerCaller) Complete(_ context.Context, _ any, query string) (string, int, int, int, int, error) {
	if strings.Contains(query, "extract grounding evidence for one package") {
		return "evidence:\n  - StubSymbol\nnotes: stub package notes\n", 1, 0, 0, 0, s.err
	}
	return s.answer, 1, 0, 0, 0, s.err
}

type ledgerSliceTarget struct {
	id    string
	paths []string
}

// sliceLedgerStepPlan is the mechanical per-slice visit order (no LLM planning).
type sliceLedgerStepPlan struct {
	SliceID  string   `yaml:"slice_id"`
	Packages []string `yaml:"packages"`
	Steps    []string `yaml:"steps"`
}

// packageEvidenceNote is distilled evidence from one package visit.
type packageEvidenceNote struct {
	Path     string   `yaml:"path" json:"path"`
	Evidence []string `yaml:"evidence" json:"evidence"`
	Notes    string   `yaml:"notes" json:"notes"`
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
	var allTargets []ledgerSliceTarget
	for _, s := range req.DraftTypo.Slices {
		id := strings.TrimSpace(s.ID)
		paths := slicePackagePaths(s)
		if id == "" || len(paths) == 0 {
			continue
		}
		allTargets = append(allTargets, ledgerSliceTarget{id: id, paths: paths})
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
		var pending []ledgerSliceTarget
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
			go func(i int, t ledgerSliceTarget) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				entry, issue, err := groundOneSliceLedger(ctx, caller, req, t, string(wholeContext), byPath, rolesDoc, filterIssuesForSlice(lastIssues, t.id))
				results[i] = result{entry: entry, issue: issue, err: err}
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
	// Soft-drop claim codes outside owned is=[] (alignLedger does the same). Keeps seed
	// from failing closed on a single over-wide RLM claim after evidence already grounded.
	for i := range doc.Slices {
		doc.Slices[i].Claims = filterClaimsToOwnedIs(doc.Slices[i].Claims, doc.Slices[i].OwnedPaths, byPath)
	}
	if consIssues := validateLedgerAgainstConstraints(doc, req.Constraints, rolesDoc); len(consIssues) > 0 {
		return sliceObjectiveLedgerDoc{}, consIssues, nil
	}
	if req.DigestCache != nil {
		logf("INFO", "%s", cache.FormatStatsLine(req.DigestCache.Stats()))
	}
	return doc, nil, nil
}

// groundOneSliceLedger runs the mechanical package-visit plan via strop RunStepPlan,
// then one synthesis step. Evidence steps are sequential within the slice; callers
// may still parallelize across slices. Filesystem checkpoints allow mid-slice resume.
func groundOneSliceLedger(
	ctx context.Context,
	caller sliceLedgerRLMCaller,
	req sliceLedgerBuildRequest,
	t ledgerSliceTarget,
	wholeContext string,
	byPath map[string]packageCapabilityConstraint,
	rolesDoc packageRolesDoc,
	sliceFeedback string,
) (sliceObjectiveLedgerEntry, string, error) {
	plan := buildSliceLedgerStepPlan(t.id, t.paths)
	if err := persistSliceLedgerStepPlan(req.AnalysisDir, plan); err != nil {
		logf("WARN", "ledger plan persist slice=%s: %v", t.id, err)
	}

	constraintBlock := formatConstraintRowsForPaths(t.paths, byPath)
	ownedSrc, srcErr := cache.OwnedPackagesSourceHash(req.AnalysisDir, t.paths)
	if srcErr != nil {
		return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
			"%s: slice %q owned package source hash failed: %v",
			typologypack.CriterionIDRoleGrounding, t.id, srcErr,
		), nil
	}
	fp := cache.LedgerFingerprint{
		SliceID:         t.id,
		OwnedPathsHash:  cache.OwnedPathsHash(t.paths),
		ContextSHA:      ownedSrc,
		ConstraintsHash: cache.ContentSHA(constraintBlock),
		ModelID:         req.DigestModelID,
		PromptVersion:   cache.DigestLedgerPromptV1,
		SchemaVersion:   cache.DigestLedgerSchemaV2,
	}
	if req.DigestSkips && req.DigestCache != nil {
		if hit, ok, err := req.DigestCache.LookupLedger(fp); err == nil && ok && hit.Verdict == ledgerVerdictGrounded {
			req.DigestCache.RecordLedgerHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
			logf("INFO", "digest cache hit ledger slice=%s", t.id)
			return sliceObjectiveLedgerEntry{
				ID:         hit.ID,
				OwnedPaths: append([]string(nil), hit.OwnedPaths...),
				Evidence:   append([]string(nil), hit.Evidence...),
				Claims:     append([]string(nil), hit.Claims...),
				Objective:  hit.Objective,
				Verdict:    hit.Verdict,
				Source:     firstNonEmpty(hit.Source, "digest_cache"),
			}, "", nil
		}
	}

	if req.DigestCache != nil {
		req.DigestCache.RecordLedgerMiss()
	}
	return runSliceLedgerViaStepPlan(ctx, caller, req, t, wholeContext, byPath, rolesDoc, sliceFeedback)
}

func buildSliceLedgerStepPlan(sliceID string, paths []string) sliceLedgerStepPlan {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	steps := make([]string, 0, len(ordered)+1)
	for _, p := range ordered {
		steps = append(steps, "evidence:"+p)
	}
	steps = append(steps, "synthesis")
	return sliceLedgerStepPlan{
		SliceID:  sliceID,
		Packages: ordered,
		Steps:    steps,
	}
}

func persistSliceLedgerStepPlan(analysisDir string, plan sliceLedgerStepPlan) error {
	if strings.TrimSpace(analysisDir) == "" || strings.TrimSpace(plan.SliceID) == "" {
		return nil
	}
	dir := filepath.Join(analysisDir, filepath.FromSlash(ledgerPlanRelDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body, err := yamlMarshalLedgerPlan(plan)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, plan.SliceID+".yaml"), body, 0o644)
}

func yamlMarshalLedgerPlan(plan sliceLedgerStepPlan) ([]byte, error) {
	var b strings.Builder
	b.WriteString("slice_id: ")
	b.WriteString(plan.SliceID)
	b.WriteByte('\n')
	b.WriteString("packages:\n")
	for _, p := range plan.Packages {
		b.WriteString("  - ")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	b.WriteString("steps:\n")
	for _, s := range plan.Steps {
		b.WriteString("  - ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func gatherSlicePackageEvidence(
	ctx context.Context,
	caller sliceLedgerRLMCaller,
	req sliceLedgerBuildRequest,
	t ledgerSliceTarget,
	wholeContext string,
	byPath map[string]packageCapabilityConstraint,
) ([]packageEvidenceNote, string) {
	ordered := append([]string(nil), t.paths...)
	sort.Strings(ordered)
	notes := make([]packageEvidenceNote, 0, len(ordered))
	for _, p := range ordered {
		if ctx.Err() != nil {
			return nil, fmt.Sprintf(
				"%s: slice %q evidence cancelled: %v",
				typologypack.CriterionIDRoleGrounding, t.id, ctx.Err(),
			)
		}
		ctxMD := packageRLMContextSnippet(wholeContext, p)
		if strings.TrimSpace(ctxMD) == "" {
			built, err := typroles.FormatPackageRLMContextForPath(req.AnalysisDir, p, nil)
			if err == nil {
				ctxMD = built
			}
		}
		if strings.TrimSpace(ctxMD) == "" {
			continue
		}
		// Prefer deterministic distillation from the package snippet. RLM exploration
		// on these already-small contexts burns iterations without emitting YAML.
		note := mechanicalPackageEvidenceNote(p, ctxMD)
		if len(note.Evidence) == 0 {
			pkgConstraints := formatConstraintRowsForPaths([]string{p}, byPath)
			payload := strings.TrimSpace(ctxMD)
			if pkgConstraints != "" {
				payload = payload + "\n\n" + pkgConstraints
			}
			if len(payload) > ledgerMaxContextChars {
				payload = truncateToLedgerBudget(payload, "\n[... package context truncated ...]\n")
			}
			query := formatPackageEvidenceQuery(t.id, p)
			stepCtx, cancel := context.WithTimeout(ctx, ledgerEvidenceTimeout)
			answer, _, _, _, _, err := caller.Complete(stepCtx, payload, query)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return nil, fmt.Sprintf(
						"%s: slice %q package %q evidence cancelled: %v",
						typologypack.CriterionIDRoleGrounding, t.id, p, err,
					)
				}
				logf("WARN", "ledger evidence RLM soft-fail slice=%s pkg=%s: %v; using empty mechanical note", t.id, p, err)
			} else if parsed, parseErr := parsePackageEvidenceAnswer(p, answer); parseErr == nil {
				note = parsed
			} else {
				logf("WARN", "ledger evidence parse soft-fail slice=%s pkg=%s: %v", t.id, p, parseErr)
			}
		}
		if len(note.Evidence) == 0 && strings.TrimSpace(note.Notes) == "" {
			note = packageEvidenceNote{
				Path:     p,
				Evidence: []string{"package:" + filepath.Base(p)},
				Notes:    "Package present in owned slice; symbols not extracted.",
			}
		}
		notes = append(notes, note)
	}
	return notes, ""
}

func mechanicalPackageEvidenceNote(pkgPath, ctxMD string) packageEvidenceNote {
	var evidence []string
	notes := ""
	for _, line := range strings.Split(ctxMD, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		lower := strings.ToLower(trim)
		switch {
		case strings.HasPrefix(lower, "- packagedoc:"):
			notes = strings.TrimSpace(trim[strings.Index(trim, ":")+1:])
			notes = strings.Trim(notes, "`\" ")
		case strings.HasPrefix(lower, "- mechanicalrole:"):
			role := strings.TrimSpace(trim[strings.Index(trim, ":")+1:])
			if role != "" && !strings.EqualFold(role, "unknown") {
				evidence = append(evidence, "mechanicalRole:"+role)
			}
		case strings.HasPrefix(lower, "- mechanicalevidence:"):
			ev := strings.TrimSpace(trim[strings.Index(trim, ":")+1:])
			if ev != "" && !strings.EqualFold(ev, "(none)") {
				evidence = append(evidence, "mechanicalEvidence:"+ev)
			}
		case strings.HasPrefix(lower, "- deliveryhint:"):
			hint := strings.TrimSpace(trim[strings.Index(trim, ":")+1:])
			if hint != "" {
				evidence = append(evidence, "deliveryHint:"+hint)
			}
		case strings.HasPrefix(lower, "- jsontags:") && strings.Contains(lower, "true"):
			evidence = append(evidence, "jsonTags:true")
		case strings.HasPrefix(lower, "- hasmain:") && strings.Contains(lower, "true"):
			evidence = append(evidence, "hasMain:true")
		case strings.HasPrefix(lower, "- importsosexec:") && strings.Contains(lower, "true"):
			evidence = append(evidence, "importsOsExec:true")
		case strings.HasPrefix(lower, "- importsnethttp:") && strings.Contains(lower, "true"):
			evidence = append(evidence, "importsNetHTTP:true")
		case strings.HasPrefix(lower, "- importsotel:") && strings.Contains(lower, "true"):
			evidence = append(evidence, "importsOtel:true")
		case strings.HasPrefix(lower, "- exportedfuncs:"):
			rest := strings.TrimSpace(trim[strings.Index(trim, ":")+1:])
			if rest != "" && !strings.EqualFold(rest, "(none)") {
				for _, part := range strings.Split(rest, ",") {
					part = strings.TrimSpace(part)
					if part != "" {
						evidence = append(evidence, part)
					}
				}
			}
		case strings.HasPrefix(lower, "- exportedmethods:"):
			rest := strings.TrimSpace(trim[strings.Index(trim, ":")+1:])
			if rest != "" && !strings.EqualFold(rest, "(none)") {
				for _, part := range strings.Split(rest, ",") {
					part = strings.TrimSpace(part)
					if part != "" {
						evidence = append(evidence, part)
					}
				}
			}
		case strings.HasPrefix(lower, "- exporteddecls:"):
			rest := strings.TrimSpace(trim[strings.Index(trim, ":")+1:])
			if rest != "" && !strings.EqualFold(rest, "(none)") {
				parts := strings.Split(rest, ",")
				limit := 8
				if len(parts) < limit {
					limit = len(parts)
				}
				for _, part := range parts[:limit] {
					part = strings.TrimSpace(part)
					if part != "" {
						evidence = append(evidence, part)
					}
				}
			}
		}
	}
	evidence = normalizeEvidenceList(evidence)
	if len(evidence) > 12 {
		evidence = evidence[:12]
	}
	return packageEvidenceNote{Path: pkgPath, Evidence: evidence, Notes: notes}
}

func formatPackageEvidenceQuery(sliceID, pkgPath string) string {
	return fmt.Sprintf(`You extract grounding evidence for one package that belongs to Typology teaching slice %q.
The context payload IS already this package's AST snippet. Do NOT explore, FindRelevant, Query, SubRLM, or write REPL loops.
Immediately FINAL with the YAML object below. Path basenames are never evidence.

Package path: %s

YAML object (no markdown fences):
evidence:
  - <symbol or flag quote>
notes: <one plain sentence about what this package does>`, sliceID, pkgPath)
}

func parsePackageEvidenceAnswer(pkgPath, text string) (packageEvidenceNote, error) {
	body := strings.TrimSpace(stripCodeFence(text))
	if body == "" {
		return packageEvidenceNote{}, fmt.Errorf("empty evidence answer")
	}
	var doc struct {
		Evidence []string `yaml:"evidence"`
		Notes    string   `yaml:"notes"`
	}
	if err := yaml.Unmarshal([]byte(body), &doc); err == nil {
		ev := normalizeEvidenceList(doc.Evidence)
		if len(ev) == 0 && strings.TrimSpace(doc.Notes) == "" {
			return packageEvidenceNote{}, fmt.Errorf("evidence answer missing evidence and notes")
		}
		return packageEvidenceNote{Path: pkgPath, Evidence: ev, Notes: strings.TrimSpace(doc.Notes)}, nil
	}
	// Line fallback: evidence: ... and notes: ...
	var evidence []string
	notes := ""
	section := ""
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "evidence:") {
			rest := strings.TrimSpace(trim[len("evidence")+1:])
			if rest != "" {
				evidence = append(evidence, splitLedgerList(rest)...)
			}
			section = "evidence"
			continue
		}
		if strings.HasPrefix(lower, "notes:") {
			notes = strings.TrimSpace(trim[len("notes")+1:])
			section = ""
			continue
		}
		if section == "evidence" {
			item := strings.Trim(trim, "`\"'- ")
			if item != "" && !strings.EqualFold(item, "none") {
				evidence = append(evidence, item)
			}
		}
	}
	evidence = normalizeEvidenceList(evidence)
	if len(evidence) == 0 && notes == "" {
		return packageEvidenceNote{}, fmt.Errorf("could not parse package evidence answer")
	}
	return packageEvidenceNote{Path: pkgPath, Evidence: evidence, Notes: notes}, nil
}

func truncateToLedgerBudget(payload, marker string) string {
	if len(payload) <= ledgerMaxContextChars {
		return payload
	}
	cut := ledgerMaxContextChars - len(marker)
	if cut < 0 {
		cut = 0
	}
	if cut > len(payload) {
		cut = len(payload)
	}
	return payload[:cut] + marker
}

func formatDistilledPackageEvidence(notes []packageEvidenceNote) string {
	var b strings.Builder
	b.WriteString("# Distilled per-package evidence for slice synthesis\n")
	for _, n := range notes {
		b.WriteString("\n## ")
		b.WriteString(n.Path)
		b.WriteByte('\n')
		if strings.TrimSpace(n.Notes) != "" {
			b.WriteString("notes: ")
			b.WriteString(strings.TrimSpace(n.Notes))
			b.WriteByte('\n')
		}
		if len(n.Evidence) == 0 {
			b.WriteString("evidence: []\n")
			continue
		}
		b.WriteString("evidence:\n")
		for _, e := range n.Evidence {
			b.WriteString("  - ")
			b.WriteString(e)
			b.WriteByte('\n')
		}
	}
	return b.String()
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

func formatSliceObjectiveLedgerQuery(sliceID string, paths []string, constraintBlock, clusterHintYAML, validationFeedback string) string {
	clusterNote := strings.TrimSpace(clusterHintYAML)
	if len(clusterNote) > 4000 {
		clusterNote = clusterNote[:4000] + "\n[... cluster merge context truncated ...]\n"
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
Context is distilled per-package evidence notes (not the full AST); cite those notes.
Do NOT explore, FindRelevant, Query, SubRLM, or write REPL loops. Immediately FINAL with the YAML object.

Slice id: %s
Owned package paths: %s

%s
%s
%s
Cluster proposal (membership hint only; MUST NOT invent prestige meaning from it):
%s

YAML object (no markdown fences):
verdict: grounded|overclaim
evidence:
  - <symbol or flag quote>
claims:
  - <portable code>
objective: <one plain sentence matching the evidence; no prestige overclaim>

Allowed claim codes only from: data_shape, synchronize_state, merge_adapters, serve_http, wire_handlers, orchestrate, own_domain_rules, fill_dto, run_cli, aggregate_views, exec_process, observability, adapt_external, config
Prefer claim codes that already appear in owned package is=[] rows.
If you cannot support a runtime claim with symbols, either drop that claim or set verdict: overclaim.
When validation_feedback is present, drop or replace every claim it rejects; do not repeat the same overclaim.`,
		sliceID, strings.Join(paths, ", "), constraintBlock, feedbackBlock, claimPolicyPromptRules(), clusterNote)
}
