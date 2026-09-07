package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/majordomo/internal/judge"
	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/typology/catalog"
)

const (
	analysisDraftCatalogRel   = "tmp/typology/typology.yaml"
	analysisDraftArchRel      = "tmp/typology/architecture_draft.md"
	maxTypologyRefineAttempts = 3
)

// TypologyRefineGenerator runs the unattended cluster + refine LLM loop.
type TypologyRefineGenerator interface {
	Refine(ctx context.Context, input TypologyRefineInput) (TypologyRefineOutput, error)
}

// TypologyRefineInput is the evidence pack for typology_cluster / typology_refine.
type TypologyRefineInput struct {
	RepoID             string
	ModuleScope        string
	DraftCatalogYAML   string
	GraphText          string
	PackageContracts   string
	ArchitectureDraft  string
	RepoLayout         string
	ValidationFeedback string
	ClusterProposalMD  string
}

// TypologyRefineOutput is the refined catalog proposal and journey notes.
type TypologyRefineOutput struct {
	ClusterProposalMD  string
	RefinedCatalogYAML string
	JourneyMD          string
}

// JudgeTypologyRefineGenerator uses typology_cluster then typology_refine tasks.
type JudgeTypologyRefineGenerator struct {
	Gen judge.Generator
}

// Refine runs cluster-pass then generate→sanitize→deterministic gate→LLM eval refine attempts.
func (g JudgeTypologyRefineGenerator) Refine(ctx context.Context, input TypologyRefineInput) (TypologyRefineOutput, error) {
	gen := g.Gen
	if gen == nil {
		if !judge.StoryLLMAvailable() {
			return TypologyRefineOutput{}, fmt.Errorf("LLM typology refine unavailable")
		}
		gen = packageJudgeGenerator{}
	}

	clusterFields := map[string]interface{}{
		"repo_id":             input.RepoID,
		"module_scope":        input.ModuleScope,
		"draft_catalog_yaml":  input.DraftCatalogYAML,
		"graph_text":          input.GraphText,
		"package_contracts":   input.PackageContracts,
		"architecture_draft":  input.ArchitectureDraft,
		"repo_layout":         input.RepoLayout,
		"validation_feedback": input.ValidationFeedback,
	}
	clusterOut, err := gen.Generate(ctx, jmodules.TaskTypologyCluster, clusterFields, 1)
	if err != nil {
		return TypologyRefineOutput{}, fmt.Errorf("typology cluster: %w", err)
	}
	clusterMD := strings.TrimSpace(stringField(clusterOut, "cluster_proposal_md"))
	if clusterMD == "" {
		return TypologyRefineOutput{}, fmt.Errorf("typology cluster: cluster_proposal_md is required")
	}

	var refined string
	var journey string
	feedback := input.ValidationFeedback
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		refineFields := map[string]interface{}{
			"repo_id":             input.RepoID,
			"module_scope":        input.ModuleScope,
			"draft_catalog_yaml":  input.DraftCatalogYAML,
			"cluster_proposal_md": clusterMD,
			"package_contracts":   input.PackageContracts,
			"architecture_draft":  input.ArchitectureDraft,
			"repo_layout":         input.RepoLayout,
			"validation_feedback": feedback,
		}
		out, err := gen.Generate(ctx, jmodules.TaskTypologyRefine, refineFields, attempt)
		if err != nil {
			return TypologyRefineOutput{}, fmt.Errorf("typology refine: %w", err)
		}
		refined = stripCodeFence(stringField(out, "refined_catalog_yaml"))
		journey = strings.TrimSpace(stringField(out, "journey_md"))
		if refined == "" {
			return TypologyRefineOutput{}, fmt.Errorf("typology refine: refined_catalog_yaml is required")
		}
		if journey == "" {
			return TypologyRefineOutput{}, fmt.Errorf("typology refine: journey_md is required")
		}
		sanitized, err := validateRefinedCatalogYAML(refined, input.DraftCatalogYAML, input.RepoID)
		if err != nil {
			if attempt == maxTypologyRefineAttempts {
				return TypologyRefineOutput{}, err
			}
			feedback = err.Error()
			continue
		}
		refined = sanitized
		if ok, evalFeedback := evaluateTypologyBoundaries(refined, journey, input.ArchitectureDraft); !ok {
			if attempt == maxTypologyRefineAttempts {
				return TypologyRefineOutput{}, fmt.Errorf("typology refine evaluation failed after %d attempts:\n%s", maxTypologyRefineAttempts, evalFeedback)
			}
			feedback = evalFeedback
			continue
		}
		evalOut := map[string]interface{}{
			"refined_catalog_yaml": refined,
			"journey_md":           journey,
		}
		agg, err := gen.Evaluate(ctx, jmodules.TaskTypologyRefine, refineFields, evalOut, attempt)
		if err != nil {
			if attempt == maxTypologyRefineAttempts {
				return TypologyRefineOutput{}, fmt.Errorf("typology refine LLM evaluation: %w", err)
			}
			feedback = err.Error()
			continue
		}
		if !judge.EvalPassed(agg) {
			if attempt == maxTypologyRefineAttempts {
				return TypologyRefineOutput{}, fmt.Errorf("typology refine LLM evaluation failed after %d attempts:\n%s", maxTypologyRefineAttempts, judge.EvalFeedback(agg))
			}
			feedback = judge.EvalFeedback(agg)
			continue
		}
		break
	}

	return TypologyRefineOutput{
		ClusterProposalMD:  clusterMD,
		RefinedCatalogYAML: refined,
		JourneyMD:          journey,
	}, nil
}

// evaluateTypologyBoundaries applies deterministic typology hard-fails before LLM EvaluateWorkflow.
func evaluateTypologyBoundaries(refinedYAML, journeyMD, architectureDraft string) (bool, string) {
	var issues []string

	tmp, err := os.CreateTemp("", "majordomo-eval-*.yaml")
	if err != nil {
		return false, fmt.Sprintf("%s: temp file: %v", typologypack.CriterionIDObjectives, err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(refinedYAML); err != nil {
		_ = tmp.Close()
		return false, fmt.Sprintf("%s: write temp: %v", typologypack.CriterionIDObjectives, err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Sprintf("%s: close temp: %v", typologypack.CriterionIDObjectives, err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		return false, fmt.Sprintf("%s: load catalog: %v", typologypack.CriterionIDObjectives, err)
	}

	for _, s := range typo.Slices {
		obj := strings.TrimSpace(s.Objective)
		if obj == "" {
			issues = append(issues, fmt.Sprintf("%s: slice %q missing objective", typologypack.CriterionIDObjectives, s.ID))
		} else if isHollowObjective(obj) {
			issues = append(issues, fmt.Sprintf("%s: slice %q has hollow template objective %q", typologypack.CriterionIDObjectives, s.ID, obj))
		}
		for _, c := range s.Owns {
			if looksLikeInteractionPath(c.Path) {
				issues = append(issues, fmt.Sprintf("%s: package %q on slice %q looks like interaction and must sit under surfaces[]", typologypack.CriterionIDSurfaces, c.Path, s.ID))
			}
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				if isExecAdapterPath(c.Path) && surf.Kind == catalog.InteractionCLI {
					issues = append(issues, fmt.Sprintf("%s: package %q is an exec adapter and must not sit under kind: cli surface %q", typologypack.CriterionIDAdapterSurfaces, c.Path, surf.ID))
				}
			}
		}
	}

	if architectureHasFindings(architectureDraft) && !journeyHasDebtTable(journeyMD) {
		issues = append(issues, fmt.Sprintf("%s: architecture findings remain but journey has no technical debt / boundary violations table", typologypack.CriterionIDDebtWhenFindings))
	}
	if journeyStatusClaimsComplete(journeyMD) && journeyDebtStillSaysMerge(journeyMD) {
		issues = append(issues, fmt.Sprintf("%s: journey Status claims complete but debt still lists Merge into actions", typologypack.CriterionIDJourneyConsistent))
	}

	if len(issues) == 0 {
		return true, ""
	}
	return false, strings.Join(issues, "\n")
}

var hollowObjectiveRe = regexp.MustCompile(`(?i)^provide\s+.+\s+(functionality|capabilities|services)\.?$`)

func isHollowObjective(objective string) bool {
	return hollowObjectiveRe.MatchString(strings.TrimSpace(objective))
}

func isExecAdapterPath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if p == "" {
		return false
	}
	base := filepath.Base(p)
	return base == "cliexec" || strings.Contains(p, "/cliexec") || strings.HasSuffix(p, "cliexec")
}

func looksLikeInteractionPath(path string) bool {
	// Exec adapters must stay under owns[]; never treat them as delivery surfaces.
	if isExecAdapterPath(path) {
		return false
	}
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if p == "" {
		return false
	}
	base := filepath.Base(p)
	switch {
	case strings.Contains(p, "/cmd/"), strings.HasPrefix(p, "cmd/"):
		return true
	case base == "cli" || strings.HasSuffix(p, "/cli") || strings.Contains(p, "/cli/"):
		return true
	case strings.Contains(p, "/http") || strings.Contains(p, "httpapi") || strings.Contains(p, "/api/"):
		return true
	case base == "ui" || strings.HasSuffix(p, "/ui") || strings.Contains(p, "/ui/"):
		return true
	case base == "dashboard" || strings.HasSuffix(p, "/dashboard") || strings.Contains(p, "/dashboard/"):
		return true
	case base == "server" || strings.HasSuffix(p, "/server") || strings.Contains(p, "/server/"):
		return true
	default:
		return false
	}
}

func journeyStatusClaimsComplete(journey string) bool {
	lower := strings.ToLower(journey)
	if !(strings.Contains(lower, "completed") || strings.Contains(lower, "complete")) {
		return false
	}
	if strings.Contains(lower, "status:") {
		return true
	}
	// Markdown ## Status section claiming refinement/merge complete.
	lines := strings.Split(lower, "\n")
	inStatus := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") && strings.Contains(trim, "status") {
			inStatus = true
			continue
		}
		if inStatus && strings.HasPrefix(trim, "#") {
			break
		}
		if !inStatus || trim == "" {
			continue
		}
		if strings.Contains(trim, "complete") || strings.Contains(trim, "completed") {
			return true
		}
	}
	return false
}

func journeyDebtStillSaysMerge(journey string) bool {
	return strings.Contains(strings.ToLower(journey), "merge into")
}

func architectureHasFindings(arch string) bool {
	lines := strings.Split(arch, "\n")
	inFindings := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if isArchitectureFindingsHeading(trim) {
			inFindings = true
			continue
		}
		if inFindings && strings.HasPrefix(trim, "#") {
			inFindings = false
		}
		if !inFindings || trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "*") || strings.HasPrefix(trim, "|") {
			return true
		}
		if strings.Contains(low, "`") {
			return true
		}
	}
	return false
}

func journeyHasDebtTable(journey string) bool {
	lower := strings.ToLower(journey)
	if !strings.Contains(lower, "debt") && !strings.Contains(lower, "boundary violation") {
		return false
	}
	// Require at least one markdown table row or bullet under a debt heading.
	lines := strings.Split(journey, "\n")
	inDebt := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if strings.HasPrefix(low, "#") && (strings.Contains(low, "debt") || strings.Contains(low, "boundary")) {
			inDebt = true
			continue
		}
		if inDebt && strings.HasPrefix(trim, "#") {
			inDebt = false
		}
		if !inDebt {
			continue
		}
		if strings.HasPrefix(trim, "|") && !strings.Contains(trim, "---") && !strings.Contains(strings.ToLower(trim), "violation") {
			cells := strings.Split(trim, "|")
			content := 0
			for _, c := range cells {
				if strings.TrimSpace(c) != "" {
					content++
				}
			}
			if content >= 2 {
				return true
			}
		}
		if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") {
			return true
		}
	}
	return false
}

func validateRefinedCatalogYAML(raw, draftYAML, repoID string) (string, error) {
	tmp, err := os.CreateTemp("", "majordomo-refined-*.yaml")
	if err != nil {
		return "", fmt.Errorf("typology refine temp file: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(raw); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("typology refine write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("typology refine close temp: %w", err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		return "", fmt.Errorf("typology refine load catalog: %w", err)
	}
	typo = sanitizeRefinedCatalog(typo)
	if id := strings.TrimSpace(repoID); id != "" {
		cur := strings.TrimSpace(typo.ID)
		if cur == "" || strings.HasPrefix(cur, "majordomo-typology-") || strings.Contains(cur, "/") {
			typo.ID = id
		}
	}
	draft, allowed, err := loadDraftCatalog(draftYAML)
	if err != nil {
		return "", err
	}
	if len(allowed) > 0 {
		typo = remapInventedCatalogPaths(typo, allowed)
	}
	typo = restoreMissingDraftPackages(typo, draft)
	if err := catalog.SaveYAML(path, typo); err != nil {
		return "", fmt.Errorf("typology refine save sanitized catalog: %w", err)
	}
	sanitized, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("typology refine read sanitized catalog: %w", err)
	}
	if issues := typo.ValidateStructure(); len(issues) > 0 {
		var b strings.Builder
		b.WriteString("catalog.ValidateStructure failed:")
		for _, issue := range issues {
			if strings.TrimSpace(issue.Slice) != "" {
				fmt.Fprintf(&b, "\n- %s: %s", issue.Slice, issue.Message)
				continue
			}
			fmt.Fprintf(&b, "\n- %s", issue.Message)
		}
		return "", fmt.Errorf("%s", b.String())
	}
	if err := rejectInventedCatalogPaths(typo, allowed); err != nil {
		return "", err
	}
	if err := rejectMissingDraftPackages(typo, allowed); err != nil {
		return "", err
	}
	return string(sanitized), nil
}

func loadDraftCatalog(draftYAML string) (catalog.Typology, map[string]struct{}, error) {
	draftYAML = strings.TrimSpace(draftYAML)
	if draftYAML == "" {
		return catalog.Typology{}, nil, nil
	}
	tmp, err := os.CreateTemp("", "majordomo-draft-*.yaml")
	if err != nil {
		return catalog.Typology{}, nil, fmt.Errorf("typology refine draft temp: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(draftYAML); err != nil {
		_ = tmp.Close()
		return catalog.Typology{}, nil, fmt.Errorf("typology refine write draft temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return catalog.Typology{}, nil, fmt.Errorf("typology refine close draft temp: %w", err)
	}
	draft, err := catalog.LoadYAML(path)
	if err != nil {
		return catalog.Typology{}, nil, fmt.Errorf("typology refine load draft catalog: %w", err)
	}
	return draft, collectCatalogPaths(draft), nil
}

func remapInventedCatalogPaths(t catalog.Typology, allowed map[string]struct{}) catalog.Typology {
	if len(allowed) == 0 {
		return t
	}
	allowedList := make([]string, 0, len(allowed))
	for p := range allowed {
		allowedList = append(allowedList, p)
	}
	sort.Strings(allowedList)
	for i := range t.Slices {
		s := &t.Slices[i]
		for j := range s.Owns {
			if near := uniqueNearestDraftPath(s.Owns[j].Path, allowedList); near != "" {
				if normalizeCatalogPath(s.Owns[j].Path) != near {
					s.Owns[j].Path = near
				}
			}
		}
		for k := range s.Surfaces {
			for j := range s.Surfaces[k].Components {
				c := &s.Surfaces[k].Components[j]
				if near := uniqueNearestDraftPath(c.Path, allowedList); near != "" {
					if normalizeCatalogPath(c.Path) != near {
						c.Path = near
					}
				}
			}
		}
	}
	return t
}

func rejectInventedCatalogPaths(refined catalog.Typology, allowed map[string]struct{}) error {
	if len(allowed) == 0 {
		return nil
	}
	var invented []string
	for _, p := range collectCatalogPathList(refined) {
		if _, ok := allowed[p]; ok {
			continue
		}
		invented = append(invented, p)
	}
	if len(invented) == 0 {
		return nil
	}
	allowedList := make([]string, 0, len(allowed))
	for p := range allowed {
		allowedList = append(allowedList, p)
	}
	sort.Strings(allowedList)
	sort.Strings(invented)
	var b strings.Builder
	b.WriteString("invented package paths (must copy draft/graph paths verbatim; put filesystem renames in journey debt only):")
	for _, p := range invented {
		fmt.Fprintf(&b, "\n- %s", p)
		if near := uniqueNearestDraftPath(p, allowedList); near != "" {
			fmt.Fprintf(&b, " (draft has %s)", near)
		}
	}
	return fmt.Errorf("%s", b.String())
}

func collectCatalogPaths(t catalog.Typology) map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range collectCatalogPathList(t) {
		out[p] = struct{}{}
	}
	return out
}

func collectCatalogPathList(t catalog.Typology) []string {
	var paths []string
	for _, s := range t.Slices {
		for _, c := range s.Owns {
			if n := normalizeCatalogPath(c.Path); n != "" {
				paths = append(paths, n)
			}
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				if n := normalizeCatalogPath(c.Path); n != "" {
					paths = append(paths, n)
				}
			}
		}
	}
	return paths
}

func normalizeCatalogPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/")
	return p
}

// uniqueNearestDraftPath returns a draft path only when exactly one allowed path
// is an unambiguous match for the invented path.
func uniqueNearestDraftPath(invented string, allowed []string) string {
	inv := normalizeCatalogPath(invented)
	if inv == "" {
		return ""
	}
	if containsString(allowed, inv) {
		return inv
	}
	base := filepath.Base(inv)
	var byBase []string
	for _, a := range allowed {
		if filepath.Base(a) == base {
			byBase = append(byBase, a)
		}
	}
	if len(byBase) == 1 {
		return byBase[0]
	}
	invFlat := strings.ReplaceAll(inv, "/", "")
	var byFlat []string
	for _, a := range allowed {
		if strings.ReplaceAll(a, "/", "") == invFlat {
			byFlat = append(byFlat, a)
		}
	}
	if len(byFlat) == 1 {
		return byFlat[0]
	}
	var byContain []string
	for _, a := range allowed {
		aBase := filepath.Base(a)
		if aBase == "" || base == "" {
			continue
		}
		if strings.Contains(aBase, base) || strings.Contains(base, aBase) {
			byContain = append(byContain, a)
		}
	}
	if len(byContain) == 1 {
		return byContain[0]
	}
	return ""
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// restoreMissingDraftPackages reclaims draft package paths the refine LLM dropped.
// Exec adapters always land under owns[]; interaction paths reattach to surfaces.
func restoreMissingDraftPackages(refined, draft catalog.Typology) catalog.Typology {
	if len(draft.Slices) == 0 || len(refined.Slices) == 0 {
		return refined
	}
	claimed := collectCatalogPaths(refined)
	type missingComp struct {
		draftSliceID string
		comp         catalog.Component
	}
	var missing []missingComp
	for _, s := range draft.Slices {
		add := func(c catalog.Component) {
			n := normalizeCatalogPath(c.Path)
			if n == "" {
				return
			}
			if _, ok := claimed[n]; ok {
				return
			}
			missing = append(missing, missingComp{draftSliceID: s.ID, comp: c})
			claimed[n] = struct{}{}
		}
		for _, c := range s.Owns {
			add(c)
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				add(c)
			}
		}
	}
	if len(missing) == 0 {
		return refined
	}

	sliceIdx := make(map[string]int, len(refined.Slices))
	for i, s := range refined.Slices {
		if id := strings.TrimSpace(s.ID); id != "" {
			sliceIdx[id] = i
		}
	}

	for _, m := range missing {
		idx := 0
		if i, ok := sliceIdx[strings.TrimSpace(m.draftSliceID)]; ok {
			idx = i
		}
		c := m.comp
		if strings.TrimSpace(c.ID) == "" {
			c.ID = filepath.Base(normalizeCatalogPath(c.Path))
		}
		if isExecAdapterPath(c.Path) || !looksLikeInteractionPath(c.Path) {
			c.Layer = catalog.LayerDomain
			c.Kind = ""
			refined.Slices[idx].Owns = append(refined.Slices[idx].Owns, c)
			continue
		}
		kind := inferInteractionKind(c.Path)
		placed := false
		for j := range refined.Slices[idx].Surfaces {
			if refined.Slices[idx].Surfaces[j].Kind == kind {
				refined.Slices[idx].Surfaces[j].Components = append(refined.Slices[idx].Surfaces[j].Components, c)
				placed = true
				break
			}
		}
		if !placed {
			sid := strings.TrimSpace(refined.Slices[idx].ID)
			if sid == "" {
				sid = "slice"
			}
			refined.Slices[idx].Surfaces = append(refined.Slices[idx].Surfaces, catalog.Surface{
				ID:         sid + "-" + string(kind),
				Kind:       kind,
				Components: []catalog.Component{c},
			})
		}
	}
	return refined
}

func inferInteractionKind(path string) catalog.InteractionKind {
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	switch {
	case strings.Contains(p, "ui") || strings.Contains(p, "dashboard"):
		return catalog.InteractionUI
	case strings.Contains(p, "http") || strings.Contains(p, "api"):
		return catalog.InteractionAPI
	default:
		return catalog.InteractionCLI
	}
}

func rejectMissingDraftPackages(refined catalog.Typology, allowed map[string]struct{}) error {
	if len(allowed) == 0 {
		return nil
	}
	claimed := collectCatalogPaths(refined)
	var missing []string
	for p := range allowed {
		if _, ok := claimed[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	var b strings.Builder
	b.WriteString("unmapped draft packages (must remain under owns[] or surfaces[]; demote exec adapters into owns[], do not drop them):")
	for _, p := range missing {
		fmt.Fprintf(&b, "\n- %s", p)
	}
	return fmt.Errorf("%s", b.String())
}

// sanitizeRefinedCatalog drops dangling bindings, moves likely interaction packages
// onto surfaces, demotes exec adapters into owns[] (never drops them), strips invented
// DocPages, and normalizes common LLM mistakes.
func sanitizeRefinedCatalog(t catalog.Typology) catalog.Typology {
	seenComp := make(map[string]string)
	for i := range t.Slices {
		s := &t.Slices[i]
		var owns []catalog.Component
		var moved []catalog.Component
		for _, c := range s.Owns {
			if strings.TrimSpace(c.ID) == "" {
				continue
			}
			if owner, ok := seenComp[c.ID]; ok && owner != s.ID {
				continue
			}
			if looksLikeInteractionPath(c.Path) {
				moved = append(moved, catalog.Component{ID: c.ID, Path: c.Path})
				seenComp[c.ID] = s.ID
				continue
			}
			c.Layer = catalog.LayerDomain
			seenComp[c.ID] = s.ID
			owns = append(owns, c)
		}
		s.Owns = owns

		seenKind := make(map[catalog.InteractionKind]struct{})
		var surfaces []catalog.Surface
		for _, surf := range s.Surfaces {
			if _, ok := seenKind[surf.Kind]; ok {
				continue
			}
			seenKind[surf.Kind] = struct{}{}
			var comps []catalog.Component
			for _, c := range surf.Components {
				if strings.TrimSpace(c.ID) == "" {
					continue
				}
				if owner, ok := seenComp[c.ID]; ok && owner != s.ID {
					continue
				}
				// Exec adapters are never CLI delivery surfaces; demote to owns[].
				if surf.Kind == catalog.InteractionCLI && isExecAdapterPath(c.Path) {
					c.Layer = catalog.LayerDomain
					seenComp[c.ID] = s.ID
					s.Owns = append(s.Owns, c)
					continue
				}
				seenComp[c.ID] = s.ID
				comps = append(comps, c)
			}
			surf.Components = comps
			if len(comps) == 0 && surf.Kind == catalog.InteractionCLI {
				// Drop empty CLI surfaces created only for mis-placed adapters.
				delete(seenKind, surf.Kind)
				continue
			}
			surfaces = append(surfaces, surf)
		}
		if len(moved) > 0 {
			kind := catalog.InteractionCLI
			for _, c := range moved {
				p := strings.ToLower(c.Path)
				switch {
				case strings.Contains(p, "ui") || strings.Contains(p, "dashboard"):
					kind = catalog.InteractionUI
				case strings.Contains(p, "http") || strings.Contains(p, "api"):
					kind = catalog.InteractionAPI
				}
			}
			if _, ok := seenKind[kind]; !ok {
				surfaces = append(surfaces, catalog.Surface{
					ID:         s.ID + "-" + string(kind),
					Kind:       kind,
					Components: moved,
				})
				seenKind[kind] = struct{}{}
			} else {
				for j := range surfaces {
					if surfaces[j].Kind == kind {
						surfaces[j].Components = append(surfaces[j].Components, moved...)
						break
					}
				}
			}
		}
		s.Surfaces = surfaces
		// Digest proposals never declare DocPages; those come from human emit later.
		s.Docs = catalog.DocCluster{}
	}

	slices := make(map[string]struct{}, len(t.Slices))
	for _, s := range t.Slices {
		if strings.TrimSpace(s.ID) != "" {
			slices[s.ID] = struct{}{}
		}
	}
	var sliceBindings []catalog.SliceBinding
	for _, b := range t.SliceBindings {
		if _, ok := slices[b.From]; !ok {
			continue
		}
		if _, ok := slices[b.To]; !ok {
			continue
		}
		sliceBindings = append(sliceBindings, b)
	}
	t.SliceBindings = sliceBindings
	var compBindings []catalog.ComponentBinding
	for _, b := range t.ComponentBindings {
		fromSlice, okFrom := seenComp[b.From]
		toSlice, okTo := seenComp[b.To]
		if !okFrom || !okTo {
			continue
		}
		if fromSlice != toSlice {
			has := false
			for _, sb := range t.SliceBindings {
				if (sb.From == fromSlice && sb.To == toSlice) || (sb.From == toSlice && sb.To == fromSlice) {
					has = true
					break
				}
			}
			if !has {
				continue
			}
		}
		compBindings = append(compBindings, b)
	}
	t.ComponentBindings = compBindings
	return t
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func refineTypologyEvidence(ctx context.Context, opts Options, analysisDir, evidenceDir string, gen TypologyRefineGenerator, judgeGen judge.Generator) error {
	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	manifest, err := contextstore.ParseTypologyManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(manifest.RefineStatus, contextstore.TypologyRefineSkipped) {
		return nil
	}
	if strings.TrimSpace(opts.TypologyBinary) == "" {
		return fmt.Errorf("typology refine: Typology binary is required")
	}

	draftPath := filepath.Join(analysisDir, analysisDraftCatalogRel)
	draftYAML, err := os.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("typology refine read draft: %w", err)
	}
	graphText, err := os.ReadFile(filepath.Join(evidenceDir, manifest.GraphPath))
	if err != nil {
		return fmt.Errorf("typology refine read graph: %w", err)
	}
	contractsRel := strings.TrimSpace(manifest.PackageContractsPath)
	if contractsRel == "" {
		contractsRel = "package_contracts.md"
	}
	contractsText, err := os.ReadFile(filepath.Join(evidenceDir, contractsRel))
	if err != nil {
		return fmt.Errorf("typology refine read package contracts: %w", err)
	}
	archDraft, err := os.ReadFile(filepath.Join(analysisDir, analysisDraftArchRel))
	if err != nil {
		return fmt.Errorf("typology refine read architecture draft: %w", err)
	}

	if gen == nil {
		gen = JudgeTypologyRefineGenerator{Gen: judgeGen}
	}
	out, err := gen.Refine(ctx, TypologyRefineInput{
		RepoID:            manifest.RepoID,
		ModuleScope:       manifest.ModuleScope,
		DraftCatalogYAML:  string(draftYAML),
		GraphText:         string(graphText),
		PackageContracts:  string(contractsText),
		ArchitectureDraft: string(archDraft),
		RepoLayout:        strings.Join(collectTopLevelEntries(analysisDir), "\n"),
	})
	if err != nil {
		return err
	}

	clusterPath := filepath.Join(evidenceDir, manifest.ClusterProposalPath)
	if err := writeText(clusterPath, out.ClusterProposalMD); err != nil {
		return err
	}
	refinedPath := filepath.Join(evidenceDir, manifest.RefinedSnapshotPath)
	if err := writeText(refinedPath, out.RefinedCatalogYAML); err != nil {
		return err
	}
	journeyPath := filepath.Join(evidenceDir, manifest.JourneyPath)
	if err := writeText(journeyPath, out.JourneyMD); err != nil {
		return err
	}
	snapshotPath := filepath.Join(evidenceDir, manifest.SnapshotPath)
	if err := copyFile(refinedPath, snapshotPath); err != nil {
		return fmt.Errorf("typology refine copy snapshot: %w", err)
	}

	archOut := filepath.Join(evidenceDir, manifest.ArchitecturePath)
	refinedLocal := filepath.Join(analysisDir, "tmp", "typology", "refined.yaml")
	if err := copyFile(refinedPath, refinedLocal); err != nil {
		return fmt.Errorf("typology refine stage catalog: %w", err)
	}
	if err := runTypology(ctx, opts.TypologyBinary, analysisDir, "architecture", manifest.ModuleScope,
		"--catalog", refinedLocal, "--out", archOut); err != nil {
		return fmt.Errorf("typology refine architecture: %w", err)
	}
	if _, err := os.Stat(archOut); err != nil {
		fallbackArch := filepath.Join(analysisDir, "docs", "architecture", "typology.md")
		if copyErr := copyFile(fallbackArch, archOut); copyErr != nil {
			return fmt.Errorf("typology refine architecture missing: %w", err)
		}
	}
	if err := polishTypologyArchitectureBrief(archOut); err != nil {
		return err
	}

	if err := flagHumanIntervention(ctx, evidenceDir, opts.HumanInterventionGenerator, judgeGen); err != nil {
		return err
	}

	// Preserve human_intervention_path written by the flagger.
	updated, err := contextstore.ParseTypologyManifest(filepath.Join(evidenceDir, "manifest.yaml"))
	if err != nil {
		return err
	}
	manifest.HumanInterventionPath = updated.HumanInterventionPath
	manifest.RefineStatus = contextstore.TypologyRefineComplete
	return writeTypologyManifest(evidenceDir, manifest)
}
