package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/majordomo/internal/llmusage"
	"github.com/behaviorengineering/majordomo/internal/observability"
	stropdspy "github.com/behaviorengineering/strop/pkg/dspy"
	"github.com/behaviorengineering/strop/pkg/dspy/factory"
	"github.com/behaviorengineering/typology/pkg/catalog"
	"gopkg.in/yaml.v3"
)

const catalogRLMWorkers = ledgerRLMWorkers

// sliceCatalogAssembler builds per-slice catalog fragments via strop RLM.
type sliceCatalogAssembler interface {
	AssembleSlices(ctx context.Context, req sliceCatalogAssembleRequest) (sliceCatalogAssembleResult, error)
}

type sliceCatalogAssembleRequest struct {
	FoldedTypo         catalog.Typology
	LedgerDoc          sliceObjectiveLedgerDoc
	EvidenceDir        string
	RolesYAML          string
	ConstraintsYAML    string
	ReadmeSnapshot     string
	ValidationFeedback string
	Kept               map[string]catalog.Slice
}

type sliceCatalogAssembleResult struct {
	Fragments     []catalog.Slice
	Issues        []string
	PromptTokens  int
	CompletionTok int
	TotalTokens   int
	RLMIterations int
	Duration      time.Duration
}

type stropSliceCatalogRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
	traceDir string
}

type sliceCatalogRLMCaller interface {
	Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
}

func newStropSliceCatalogRLM(ctx context.Context, cfg config.RepoConfig, traceDir string) (sliceCatalogAssembler, error) {
	provider, ok, err := cfg.ResolveTaskProvider(jmodules.TaskTypologySliceCatalog)
	if err != nil || !ok {
		provider, ok, err = cfg.ResolveTaskProvider(jmodules.TaskTypologyInspect)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("typology_slice_catalog provider not configured")
		}
	}
	stropProvider := provider.ToStrop()
	llmFactory := factory.NewLLMFactory(nil, ledgerRLMTimeout)
	llmFactory.SetInstrumentHTTP(observability.InstrumentHTTPClient)
	llm, err := llmFactory.CreateLLM(ctx, stropProvider)
	if err != nil {
		return nil, fmt.Errorf("typology_slice_catalog RLM LLM: %w", err)
	}
	llm = judge.WrapLLMWithRetry(llm, judge.DefaultModuleRetryConfig())
	rlmCfg := stropdspy.RLMDefaults()
	rlmCfg.LLM = llm
	rlmCfg.MaxFullContextQueryChars = 24_000
	timeout := provider.GetTimeout(ledgerRLMTimeout)
	if timeout < ledgerRLMTimeout {
		timeout = ledgerRLMTimeout
	}
	rlmCfg.Timeout = timeout
	rlmCfg.TraceDir = strings.TrimSpace(traceDir)
	module, err := rlmCfg.CreateModule()
	if err != nil {
		return nil, err
	}
	return stropSliceCatalogRLM{
		traceDir: rlmCfg.TraceDir,
		module: rlmCompleteAdapter{complete: func(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
			answer, result, err := stropdspy.RLMComplete(ctx, module, contextPayload, query)
			if err != nil {
				return "", 0, 0, 0, 0, err
			}
			iters := 0
			prompt, completion, total := 0, 0, 0
			if result != nil {
				iters = result.Iterations
				prompt = result.Usage.PromptTokens
				completion = result.Usage.CompletionTokens
				total = result.Usage.TotalTokens
				llmusage.FromContext(ctx).AddTokenUsageValue(jmodules.TaskTypologySliceCatalog, result.Usage)
			} else {
				llmusage.FromContext(ctx).Add(jmodules.TaskTypologySliceCatalog, 0, 0, 0)
			}
			return answer, iters, prompt, completion, total, nil
		}},
	}, nil
}

func (v stropSliceCatalogRLM) AssembleSlices(ctx context.Context, req sliceCatalogAssembleRequest) (sliceCatalogAssembleResult, error) {
	return assembleSliceCatalogFragments(ctx, v, req)
}

func (v stropSliceCatalogRLM) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}

func newCatalogAssemblerFromOpts(ctx context.Context, opts Options, analysisDir string) (sliceCatalogAssembler, error) {
	if strings.TrimSpace(opts.ConfigDir) == "" || strings.TrimSpace(opts.RepoID) == "" {
		return nil, fmt.Errorf("config-dir and repo-id required for slice catalog RLM")
	}
	defaults, err := config.LoadDefaults(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadRepoFile(opts.ConfigDir, opts.RepoID, defaults)
	if err != nil {
		return nil, err
	}
	traceDir := rlmTraceDir(inferenceWorkRoot(opts, analysisDir), jmodules.TaskTypologySliceCatalog)
	return newStropSliceCatalogRLM(ctx, cfg, traceDir)
}

func assembleSliceCatalogFragments(ctx context.Context, caller sliceCatalogRLMCaller, req sliceCatalogAssembleRequest) (sliceCatalogAssembleResult, error) {
	out := sliceCatalogAssembleResult{}
	start := time.Now()
	defer func() { out.Duration = time.Since(start) }()

	targets := catalogAssembleTargets(req.FoldedTypo)
	if len(targets) == 0 {
		return out, nil
	}

	wholeContext := ""
	if req.EvidenceDir != "" {
		raw, err := os.ReadFile(filepath.Join(req.EvidenceDir, "package_rlm_context.md"))
		if err != nil && !os.IsNotExist(err) {
			return out, fmt.Errorf("slice catalog RLM read package_rlm_context: %w", err)
		}
		wholeContext = string(raw)
	}

	ledgerByID := map[string]sliceObjectiveLedgerEntry{}
	for _, e := range req.LedgerDoc.Slices {
		if id := strings.TrimSpace(e.ID); id != "" {
			ledgerByID[id] = e
		}
	}

	kept := map[string]catalog.Slice{}
	for id, s := range req.Kept {
		if strings.TrimSpace(id) != "" {
			kept[id] = s
		}
	}

	var lastIssues []string
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		var pending []catalogAssembleTarget
		for _, t := range targets {
			if _, ok := kept[t.id]; ok {
				continue
			}
			pending = append(pending, t)
		}
		if len(pending) == 0 {
			break
		}

		type result struct {
			frag  catalog.Slice
			issue string
			err   error
			pt    int
			ct    int
			tt    int
			iters int
		}
		results := make([]result, len(pending))
		sem := make(chan struct{}, catalogRLMWorkers)
		var wg sync.WaitGroup
		for i, t := range pending {
			wg.Add(1)
			go func(i int, t catalogAssembleTarget) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				ledger := resolveLedgerEntryForTarget(t, req.LedgerDoc, ledgerByID)
				frag, issue, pt, ct, tt, iters, err := groundOneSliceCatalog(
					ctx, caller, req, t, wholeContext, ledger, filterIssuesForSlice(lastIssues, t.id),
				)
				results[i] = result{frag: frag, issue: issue, err: err, pt: pt, ct: ct, tt: tt, iters: iters}
			}(i, t)
		}
		wg.Wait()

		var issues []string
		for _, r := range results {
			out.PromptTokens += r.pt
			out.CompletionTok += r.ct
			out.TotalTokens += r.tt
			out.RLMIterations += r.iters
			if r.err != nil {
				return out, fmt.Errorf("typology_slice_catalog RLM assemble: %w", r.err)
			}
			if strings.TrimSpace(r.issue) != "" {
				issues = append(issues, r.issue)
				continue
			}
			if id := strings.TrimSpace(r.frag.ID); id != "" {
				kept[id] = r.frag
			}
		}
		lastIssues = issues
		if len(issues) == 0 {
			break
		}
		if attempt == maxTypologyRefineAttempts {
			out.Issues = issues
			break
		}
	}

	ids := make([]string, 0, len(kept))
	for id := range kept {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		out.Fragments = append(out.Fragments, kept[id])
	}
	return out, nil
}

type catalogAssembleTarget struct {
	id    string
	slice catalog.Slice
	paths []string
}

func catalogAssembleTargets(typo catalog.Typology) []catalogAssembleTarget {
	var out []catalogAssembleTarget
	for _, s := range typo.Slices {
		id := strings.TrimSpace(s.ID)
		paths := normalizePackageList(slicePackagePaths(s))
		if id == "" || len(paths) == 0 {
			continue
		}
		out = append(out, catalogAssembleTarget{id: id, slice: s, paths: paths})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	if len(out) > maxLedgerSlices {
		out = out[:maxLedgerSlices]
	}
	return out
}

// resolveLedgerEntryForTarget finds meaning for a folded slice.
// Fold accept verdicts may rename draft ids (review → judge-eval); match by owned path overlap.
func resolveLedgerEntryForTarget(
	t catalogAssembleTarget,
	doc sliceObjectiveLedgerDoc,
	byID map[string]sliceObjectiveLedgerEntry,
) sliceObjectiveLedgerEntry {
	if e, ok := byID[t.id]; ok {
		return e
	}
	target := map[string]struct{}{}
	for _, p := range t.paths {
		if np := normalizeRolePath(p); np != "" {
			target[np] = struct{}{}
		}
	}
	if len(target) == 0 {
		return sliceObjectiveLedgerEntry{}
	}
	var best sliceObjectiveLedgerEntry
	bestOverlap := 0
	for _, e := range doc.Slices {
		overlap := 0
		for _, p := range e.OwnedPaths {
			if _, ok := target[normalizeRolePath(p)]; ok {
				overlap++
			}
		}
		if overlap > bestOverlap {
			bestOverlap = overlap
			best = e
		}
	}
	return best
}

// stampLedgerObjectivesOntoCatalog copies contributing ledger objectives onto slices by path overlap.
// Draft-keyed ledger rows survive fold renames; ValidateStructure-adjacent gates require verbatim copy.
func stampLedgerObjectivesOntoCatalog(typo catalog.Typology, ledger sliceObjectiveLedgerDoc) catalog.Typology {
	byID := map[string]sliceObjectiveLedgerEntry{}
	for _, e := range ledger.Slices {
		if id := strings.TrimSpace(e.ID); id != "" {
			byID[id] = e
		}
	}
	for i := range typo.Slices {
		t := catalogAssembleTarget{
			id:    strings.TrimSpace(typo.Slices[i].ID),
			paths: normalizePackageList(slicePackagePaths(typo.Slices[i])),
			slice: typo.Slices[i],
		}
		if t.id == "" || len(t.paths) == 0 {
			continue
		}
		e := resolveLedgerEntryForTarget(t, ledger, byID)
		if obj := strings.TrimSpace(e.Objective); obj != "" {
			typo.Slices[i].Objective = obj
		}
	}
	return typo
}

func groundOneSliceCatalog(
	ctx context.Context,
	caller sliceCatalogRLMCaller,
	req sliceCatalogAssembleRequest,
	t catalogAssembleTarget,
	wholeContext string,
	ledger sliceObjectiveLedgerEntry,
	sliceFeedback string,
) (catalog.Slice, string, int, int, int, int, error) {
	ctxPayload := buildSliceCatalogContext(t, wholeContext, req.RolesYAML, req.ConstraintsYAML, req.ReadmeSnapshot)
	query := formatSliceCatalogQuery(t, ledger, sliceFeedback, req.ValidationFeedback)
	answer, iters, pt, ct, tt, err := caller.Complete(ctx, ctxPayload, query)
	if err != nil {
		return catalog.Slice{}, "", pt, ct, tt, iters, fmt.Errorf("typology_slice_catalog RLM slice %q: %w", t.id, err)
	}
	frag, issue := parseAndValidateSliceCatalogFragment(answer, t, ledger)
	return frag, issue, pt, ct, tt, iters, nil
}

func buildSliceCatalogContext(
	t catalogAssembleTarget,
	wholeContext, rolesYAML, constraintsYAML, readme string,
) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Slice id: %s\nAllowed owned paths: %s\n\n", t.id, strings.Join(t.paths, ", "))
	if draftYAML, err := yaml.Marshal(t.slice); err == nil {
		b.WriteString("## Folded draft slice (membership fixed; improve surfaces/owns detail)\n")
		b.Write(draftYAML)
		b.WriteString("\n")
	} else {
		fmt.Fprintf(&b, "## Folded draft slice unavailable (%v)\n\n", err)
	}
	if rs := strings.TrimSpace(readme); rs != "" {
		if len(rs) > 4000 {
			rs = rs[:4000] + "\n[... readme truncated ...]\n"
		}
		b.WriteString("## README snapshot\n")
		b.WriteString(rs)
		b.WriteString("\n\n")
	}
	if roles := strings.TrimSpace(rolesYAML); roles != "" {
		if len(roles) > 8000 {
			roles = roles[:8000] + "\n[... roles truncated ...]\n"
		}
		b.WriteString("## Package roles\n")
		b.WriteString(roles)
		b.WriteString("\n\n")
	}
	if cons := strings.TrimSpace(constraintsYAML); cons != "" {
		if len(cons) > 6000 {
			cons = cons[:6000] + "\n[... constraints truncated ...]\n"
		}
		b.WriteString("## Capability constraints\n")
		b.WriteString(cons)
		b.WriteString("\n\n")
	}
	for _, p := range t.paths {
		snip := packageRLMContextSnippet(wholeContext, p)
		if strings.TrimSpace(snip) == "" {
			continue
		}
		if len(snip) > ledgerMaxContextChars/4 {
			snip = snip[:ledgerMaxContextChars/4] + "\n[... package context truncated ...]\n"
		}
		fmt.Fprintf(&b, "## Package evidence: %s\n%s\n\n", p, snip)
	}
	return b.String()
}

func formatSliceCatalogQuery(
	t catalogAssembleTarget,
	ledger sliceObjectiveLedgerEntry,
	sliceFeedback, globalFeedback string,
) string {
	objRule := "Copy objective from the meaning ledger verbatim when provided."
	ledgerObj := strings.TrimSpace(ledger.Objective)
	if ledgerObj != "" {
		objRule = fmt.Sprintf("Objective MUST be exactly this ledger sentence (verbatim):\n%s", ledgerObj)
	}
	fb := strings.TrimSpace(sliceFeedback)
	if g := strings.TrimSpace(globalFeedback); g != "" {
		if fb != "" {
			fb = fb + "\n" + g
		} else {
			fb = g
		}
	}
	feedbackBlock := ""
	if fb != "" {
		feedbackBlock = fmt.Sprintf(`
validation_feedback (fix before emitting):
%s

`, fb)
	}
	examplePath := "internal/example"
	if len(t.paths) > 0 {
		examplePath = t.paths[0]
	}
	return fmt.Sprintf(`You assemble ONE Typology teaching-slice YAML fragment for catalog join.
Membership is already fixed by Go. You MUST NOT invent sibling slices, libraries, or new slice ids.
You MUST NOT move packages outside the allowed owned path set.
You MAY add/adjust surfaces and owns component ids within allowed paths.
%s
%s
Emit ONLY a YAML object for this slice (no markdown fences, no root typology wrapper, no libraries, no prose).
CRITICAL: owns and surfaces MUST be YAML lists with "- " dashes. NEVER emit flat sibling id:/path: keys under owns.
Surface kind MUST be exactly one of: cli, api, ui (never service, http, or other synonyms).

CORRECT shape:
id: %s
objective: <sentence>
owns:
  - id: pkg-a
    path: %s
surfaces:
  - id: %s-cli
    kind: cli
    components:
      - path: %s

Allowed owned paths: %s
FINAL with that YAML only.`,
		objRule, feedbackBlock, t.id, examplePath, t.id, examplePath, strings.Join(t.paths, ", "))
}

func parseAndValidateSliceCatalogFragment(
	answer string,
	t catalogAssembleTarget,
	ledger sliceObjectiveLedgerEntry,
) (catalog.Slice, string) {
	raw := extractSliceCatalogYAMLDocument(answer)
	if raw == "" {
		return catalog.Slice{}, fmt.Sprintf("slice %q: empty catalog fragment", t.id)
	}
	raw = repairFlatSliceCatalogYAML(raw)
	var frag catalog.Slice
	if err := yaml.Unmarshal([]byte(raw), &frag); err != nil {
		var wrap struct {
			Slice catalog.Slice `yaml:"slice"`
		}
		if err2 := yaml.Unmarshal([]byte(raw), &wrap); err2 != nil || strings.TrimSpace(wrap.Slice.ID) == "" {
			return catalog.Slice{}, fmt.Sprintf("slice %q: fragment YAML decode failed: %v", t.id, err)
		}
		frag = wrap.Slice
	}
	id := strings.TrimSpace(frag.ID)
	if id == "" {
		frag.ID = t.id
		id = t.id
	}
	if id != t.id {
		return catalog.Slice{}, fmt.Sprintf("slice %q: fragment id %q does not match fixed membership", t.id, id)
	}
	// Meaning ledger owns the objective sentence; force it so RLM prestige wording cannot thrash.
	if ledgerObj := strings.TrimSpace(ledger.Objective); ledgerObj != "" {
		frag.Objective = ledgerObj
	} else if strings.TrimSpace(frag.Objective) == "" {
		return catalog.Slice{}, fmt.Sprintf("slice %q: missing objective", t.id)
	}
	allowed := map[string]struct{}{}
	for _, p := range t.paths {
		allowed[p] = struct{}{}
	}
	for _, c := range frag.Owns {
		p := normalizeRolePath(normalizeCatalogPath(c.Path))
		if p == "" {
			continue
		}
		if _, ok := allowed[p]; !ok {
			return catalog.Slice{}, fmt.Sprintf("slice %q: owns path %q outside allowed membership", t.id, p)
		}
	}
	for _, surf := range frag.Surfaces {
		for _, c := range surf.Components {
			p := normalizeRolePath(normalizeCatalogPath(c.Path))
			if p == "" {
				continue
			}
			if _, ok := allowed[p]; !ok {
				return catalog.Slice{}, fmt.Sprintf("slice %q: surface path %q outside allowed membership", t.id, p)
			}
		}
	}
	var surfaces []catalog.Surface
	for _, surf := range frag.Surfaces {
		kind, ok := normalizeCatalogSurfaceKind(surf.Kind)
		if !ok {
			continue
		}
		surf.Kind = kind
		surfaces = append(surfaces, surf)
	}
	frag.Surfaces = surfaces
	// Ensure every allowed package remains owned (RLM may detail but must not drop membership).
	owned := map[string]struct{}{}
	for _, p := range normalizePackageList(slicePackagePaths(frag)) {
		owned[p] = struct{}{}
	}
	for _, p := range t.paths {
		if _, ok := owned[p]; ok {
			continue
		}
		frag.Owns = append(frag.Owns, catalog.Component{Path: p})
	}
	return frag, ""
}

// extractSliceCatalogYAMLDocument keeps the first YAML object that starts with id:.
func extractSliceCatalogYAMLDocument(answer string) string {
	body := strings.TrimSpace(stripCodeFence(answer))
	if body == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "id:") {
			start = i
			break
		}
	}
	if start < 0 {
		return body
	}
	var out []string
	for _, line := range lines[start:] {
		trim := strings.TrimSpace(line)
		// Stop on REPL/tool chatter after the document has begun.
		if len(out) > 2 && (strings.HasPrefix(trim, "FINAL") ||
			strings.HasPrefix(trim, "```") ||
			strings.HasPrefix(trim, "Thought") ||
			strings.HasPrefix(trim, "I will") ||
			strings.HasPrefix(trim, "Here is")) {
			break
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// repairFlatSliceCatalogYAML rewrites flat sibling id:/path: blocks under owns/surfaces into lists.
// Models often emit that shape; yaml.v3 then reports duplicate mapping keys.
func repairFlatSliceCatalogYAML(raw string) string {
	lines := strings.Split(raw, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		trim := strings.TrimSpace(lines[i])
		switch {
		case trim == "owns:" || strings.HasPrefix(trim, "owns:"):
			out = append(out, "owns:")
			i++
			items, next := collectFlatOwnsItems(lines, i)
			out = append(out, items...)
			i = next
		case trim == "surfaces:" || strings.HasPrefix(trim, "surfaces:"):
			out = append(out, "surfaces:")
			i++
			items, next := collectFlatSurfaceItems(lines, i)
			out = append(out, items...)
			i = next
		default:
			out = append(out, lines[i])
			i++
		}
	}
	return strings.Join(out, "\n")
}

func collectFlatOwnsItems(lines []string, start int) ([]string, int) {
	// Already a proper list: leave unchanged.
	if start < len(lines) {
		trim := strings.TrimSpace(lines[start])
		if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "-id:") {
			var keep []string
			i := start
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				if t == "" {
					keep = append(keep, lines[i])
					i++
					continue
				}
				if isSliceSectionBoundary(t) {
					break
				}
				keep = append(keep, lines[i])
				i++
			}
			return keep, i
		}
	}
	var out []string
	i := start
	var curID, curPath, curLayer string
	flush := func() {
		if curID == "" && curPath == "" {
			return
		}
		out = append(out, "  - id: "+firstNonEmpty(curID, filepath.Base(curPath)))
		if curPath != "" {
			out = append(out, "    path: "+curPath)
		}
		if curLayer != "" {
			out = append(out, "    layer: "+curLayer)
		}
		curID, curPath, curLayer = "", "", ""
	}
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			continue
		}
		if isSliceSectionBoundary(t) {
			break
		}
		key, val := splitYAMLSimpleKV(t)
		switch key {
		case "id":
			flush()
			curID = val
		case "path":
			curPath = val
		case "layer":
			curLayer = val
		default:
			// Unknown line inside owns: keep raw indented under last item if any.
			if len(out) > 0 {
				out = append(out, "    "+t)
			}
		}
		i++
	}
	flush()
	return out, i
}

func collectFlatSurfaceItems(lines []string, start int) ([]string, int) {
	if start < len(lines) {
		trim := strings.TrimSpace(lines[start])
		if strings.HasPrefix(trim, "- ") {
			var keep []string
			i := start
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				if t == "" {
					keep = append(keep, lines[i])
					i++
					continue
				}
				if isSliceSectionBoundary(t) {
					break
				}
				keep = append(keep, lines[i])
				i++
			}
			return keep, i
		}
	}
	var out []string
	i := start
	var curID, curKind string
	var comps []string
	flush := func() {
		if curID == "" && curKind == "" && len(comps) == 0 {
			return
		}
		out = append(out, "  - id: "+firstNonEmpty(curID, "surface"))
		if curKind != "" {
			out = append(out, "    kind: "+curKind)
		}
		if len(comps) > 0 {
			out = append(out, "    components:")
			for _, p := range comps {
				out = append(out, "      - path: "+p)
			}
		}
		curID, curKind = "", ""
		comps = nil
	}
	inComponents := false
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			continue
		}
		if isSliceSectionBoundary(t) {
			break
		}
		key, val := splitYAMLSimpleKV(t)
		switch {
		case key == "id":
			flush()
			inComponents = false
			curID = val
		case key == "kind":
			inComponents = false
			curKind = val
		case key == "components":
			inComponents = true
		case key == "path" && inComponents:
			comps = append(comps, val)
		case key == "path" && !inComponents:
			// Treat path without components: as component of current surface.
			comps = append(comps, val)
		default:
			if inComponents && key == "" && strings.HasPrefix(t, "- ") {
				_, pval := splitYAMLSimpleKV(strings.TrimSpace(strings.TrimPrefix(t, "- ")))
				if pval != "" {
					comps = append(comps, pval)
				}
			}
		}
		i++
	}
	flush()
	return out, i
}

// isSliceSectionBoundary reports YAML keys that end an owns/surfaces block.
// MUST NOT treat "id:" as a boundary: flat RLM output repeats id:/path: as sibling keys.
func isSliceSectionBoundary(trim string) bool {
	for _, base := range []string{
		"objective", "owns", "surfaces", "must", "mustNot", "must_not",
		"success", "docs", "opRuns", "subprograms", "actuators", "route",
	} {
		if trim == base+":" || strings.HasPrefix(trim, base+":") {
			return true
		}
	}
	return false
}

func splitYAMLSimpleKV(line string) (key, val string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	if strings.HasPrefix(line, "- ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
	}
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return "", ""
	}
	key = strings.TrimSpace(line[:i])
	val = strings.TrimSpace(line[i+1:])
	return key, val
}
