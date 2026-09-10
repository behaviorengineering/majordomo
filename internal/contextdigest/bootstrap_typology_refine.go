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
	"gopkg.in/yaml.v3"
)

const (
	analysisDraftCatalogRel   = "tmp/typology/typology.yaml"
	analysisDraftArchRel      = "tmp/typology/architecture_draft.md"
	maxTypologyRefineAttempts = 3
	maxReadmeSnapshotRunes    = 12000
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
	PackageRoles       string
	ArchitectureDraft  string
	RepoLayout         string
	ReadmeSnapshot     string
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
	rolesDoc := mustParseRoles(input.PackageRoles)
	mechanicalGroupingMD := mechanicalPreCluster(rolesDoc)

	clusterFields := map[string]interface{}{
		"repo_id":                input.RepoID,
		"module_scope":           input.ModuleScope,
		"draft_catalog_yaml":     input.DraftCatalogYAML,
		"graph_text":             input.GraphText,
		"package_contracts":      input.PackageContracts,
		"package_roles":          input.PackageRoles,
		"mechanical_grouping_md": mechanicalGroupingMD,
		"architecture_draft":     input.ArchitectureDraft,
		"repo_layout":            input.RepoLayout,
		"readme_snapshot":        input.ReadmeSnapshot,
		"validation_feedback":    input.ValidationFeedback,
	}
	var clusterMD string
	clusterFeedback := input.ValidationFeedback
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		if attempt > 1 {
			clusterFields["validation_feedback"] = clusterFeedback
		}
		clusterOut, err := gen.Generate(ctx, jmodules.TaskTypologyCluster, clusterFields, attempt)
		if err != nil {
			return TypologyRefineOutput{}, fmt.Errorf("typology cluster: %w", err)
		}
		clusterMD = strings.TrimSpace(stringField(clusterOut, "cluster_proposal_md"))
		if clusterMD == "" {
			return TypologyRefineOutput{}, fmt.Errorf("typology cluster: cluster_proposal_md is required")
		}
		if fixed, note := scrubForbiddenHTTPEntrypointMerges(clusterMD, input.PackageRoles); note != "" {
			// Deterministic role gate: do not keep asking the LLM to unlearn sole-importer folds.
			clusterMD = fixed
			_ = note
			break
		}
		agg, err := gen.Evaluate(ctx, jmodules.TaskTypologyCluster, clusterFields, map[string]interface{}{
			"cluster_proposal_md": clusterMD,
		}, attempt)
		if err != nil {
			if attempt == maxTypologyRefineAttempts {
				return TypologyRefineOutput{}, fmt.Errorf("typology cluster LLM evaluation: %w", err)
			}
			clusterFeedback = err.Error()
			continue
		}
		if !judge.EvalPassed(agg) {
			if attempt == maxTypologyRefineAttempts {
				return TypologyRefineOutput{}, fmt.Errorf("typology cluster LLM evaluation failed after %d attempts:\n%s", maxTypologyRefineAttempts, judge.EvalFeedback(agg))
			}
			clusterFeedback = judge.EvalFeedback(agg)
			continue
		}
		break
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
			"package_roles":       input.PackageRoles,
			"architecture_draft":  input.ArchitectureDraft,
			"repo_layout":         input.RepoLayout,
			"readme_snapshot":     input.ReadmeSnapshot,
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
		sanitized, err := validateRefinedCatalogYAML(refined, input.DraftCatalogYAML, input.RepoID, input.PackageRoles)
		if err != nil {
			if attempt == maxTypologyRefineAttempts {
				return TypologyRefineOutput{}, err
			}
			feedback = err.Error()
			continue
		}
		refined = sanitized
		if ok, evalFeedback := evaluateTypologyBoundaries(refined, journey, input.ArchitectureDraft, input.PackageRoles); !ok {
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
func evaluateTypologyBoundaries(refinedYAML, journeyMD, architectureDraft, rolesYAML string) (bool, string) {
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
	roles := roleByPath(mustParseRoles(rolesYAML))

	for _, s := range typo.Slices {
		obj := strings.TrimSpace(s.Objective)
		if obj == "" {
			issues = append(issues, fmt.Sprintf("%s: slice %q missing objective", typologypack.CriterionIDObjectives, s.ID))
		} else if isHollowObjective(obj) {
			issues = append(issues, fmt.Sprintf("%s: slice %q has hollow template objective %q", typologypack.CriterionIDObjectives, s.ID, obj))
		}
		for _, c := range s.Owns {
			if looksLikeInteractionPath(c.Path, roles) {
				issues = append(issues, fmt.Sprintf("%s: package %q on slice %q has observed role %s and must sit under surfaces[]", typologypack.CriterionIDSurfaces, c.Path, s.ID, roles[normalizeRolePath(c.Path)].Role))
			}
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				if isExecAdapterPath(c.Path, roles) && surf.Kind == catalog.InteractionCLI {
					issues = append(issues, fmt.Sprintf("%s: package %q is an exec_runner and must not sit under kind: cli surface %q", typologypack.CriterionIDAdapterSurfaces, c.Path, surf.ID))
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

func mustParseRoles(raw string) packageRolesDoc {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return packageRolesDoc{}
	}
	var doc packageRolesDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return packageRolesDoc{}
	}
	return doc
}

func isExecAdapterPath(path string, roles map[string]packageRoleNode) bool {
	n, ok := roles[normalizeRolePath(path)]
	if !ok {
		return false
	}
	return isExecRunnerRole(n.Role)
}

func looksLikeInteractionPath(path string, roles map[string]packageRoleNode) bool {
	n, ok := roles[normalizeRolePath(path)]
	if !ok {
		return false
	}
	return isInteractionRole(n.Role)
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

func validateRefinedCatalogYAML(raw, draftYAML, repoID, rolesYAML string) (string, error) {
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
	draft, allowed, err := loadDraftCatalog(draftYAML)
	if err != nil {
		return "", err
	}
	roles := roleByPath(mustParseRoles(rolesYAML))
	typo = sanitizeRefinedCatalog(typo, draft, roles)
	if id := strings.TrimSpace(repoID); id != "" {
		cur := strings.TrimSpace(typo.ID)
		if cur == "" || strings.HasPrefix(cur, "majordomo-typology-") || strings.Contains(cur, "/") {
			typo.ID = id
		}
	}
	if len(allowed) > 0 {
		typo = remapInventedCatalogPaths(typo, allowed)
	}
	typo = restoreMissingDraftPackages(typo, draft, roles)
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
	for i := range t.Libraries {
		lib := &t.Libraries[i]
		for j := range lib.Owns {
			if near := uniqueNearestDraftPath(lib.Owns[j].Path, allowedList); near != "" {
				if normalizeCatalogPath(lib.Owns[j].Path) != near {
					lib.Owns[j].Path = near
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
	for _, lib := range t.Libraries {
		for _, c := range lib.Owns {
			if n := normalizeCatalogPath(c.Path); n != "" {
				paths = append(paths, n)
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
// Draft library packages reattach to the matching refined library when present.
func restoreMissingDraftPackages(refined, draft catalog.Typology, roles map[string]packageRoleNode) catalog.Typology {
	if len(refined.Slices) == 0 {
		return refined
	}
	claimed := collectCatalogPaths(refined)
	type missingComp struct {
		draftSliceID string
		draftLibID   string
		comp         catalog.Component
	}
	var missing []missingComp
	add := func(draftSliceID, draftLibID string, c catalog.Component) {
		n := normalizeCatalogPath(c.Path)
		if n == "" {
			return
		}
		if _, ok := claimed[n]; ok {
			return
		}
		missing = append(missing, missingComp{draftSliceID: draftSliceID, draftLibID: draftLibID, comp: c})
		claimed[n] = struct{}{}
	}
	for _, s := range draft.Slices {
		for _, c := range s.Owns {
			add(s.ID, "", c)
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				add(s.ID, "", c)
			}
		}
	}
	for _, lib := range draft.Libraries {
		for _, c := range lib.Owns {
			add("", lib.ID, c)
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
	libIdx := make(map[string]int, len(refined.Libraries))
	for i, lib := range refined.Libraries {
		if id := strings.TrimSpace(lib.ID); id != "" {
			libIdx[id] = i
		}
	}

	for _, m := range missing {
		c := m.comp
		if strings.TrimSpace(c.ID) == "" {
			c.ID = filepath.Base(normalizeCatalogPath(c.Path))
		}
		if libID := strings.TrimSpace(m.draftLibID); libID != "" {
			if i, ok := libIdx[libID]; ok {
				if c.Layer == catalog.LayerInteraction {
					c.Layer = ""
				}
				if c.Layer != "" && c.Layer != catalog.LayerDomain {
					c.Layer = catalog.LayerDomain
				}
				refined.Libraries[i].Owns = append(refined.Libraries[i].Owns, c)
				continue
			}
		}
		idx := 0
		if i, ok := sliceIdx[strings.TrimSpace(m.draftSliceID)]; ok {
			idx = i
		}
		if isExecAdapterPath(c.Path, roles) || !looksLikeInteractionPath(c.Path, roles) {
			c.Layer = catalog.LayerDomain
			c.Kind = ""
			refined.Slices[idx].Owns = append(refined.Slices[idx].Owns, c)
			continue
		}
		kind := inferInteractionKind(c.Path, roles)
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

func inferInteractionKind(path string, roles map[string]packageRoleNode) catalog.InteractionKind {
	n := roles[normalizeRolePath(path)]
	switch n.Role {
	case roleEntrypoint:
		return catalog.InteractionCLI
	case roleHTTPSurface:
		for _, e := range n.Evidence {
			if e == "embeds_static" {
				return catalog.InteractionUI
			}
		}
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
	b.WriteString("unmapped draft packages (must remain under owns[], surfaces[], or libraries[].owns[]; demote exec adapters into owns[], do not drop them):")
	for _, p := range missing {
		fmt.Fprintf(&b, "\n- %s", p)
	}
	return fmt.Errorf("%s", b.String())
}

// sanitizeRefinedCatalog drops dangling bindings, moves likely interaction packages
// onto surfaces, demotes exec adapters into owns[] (never drops them), strips invented
// DocPages, keeps only draft-backed libraries with a purpose, prefers slice claims over
// library duplicates, and normalizes common LLM mistakes.
func sanitizeRefinedCatalog(t, draft catalog.Typology, roles map[string]packageRoleNode) catalog.Typology {
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
			if looksLikeInteractionPath(c.Path, roles) {
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
		seenSurfaceID := make(map[string]struct{})
		uniqueSurfaceID := func(id string, kind catalog.InteractionKind) string {
			base := strings.TrimSpace(id)
			if base == "" {
				base = s.ID + "-" + string(kind)
			}
			candidate := base
			if _, ok := seenSurfaceID[candidate]; !ok {
				seenSurfaceID[candidate] = struct{}{}
				return candidate
			}
			candidate = s.ID + "-" + string(kind)
			if _, ok := seenSurfaceID[candidate]; !ok {
				seenSurfaceID[candidate] = struct{}{}
				return candidate
			}
			for n := 2; ; n++ {
				candidate = fmt.Sprintf("%s-%s-%d", s.ID, kind, n)
				if _, ok := seenSurfaceID[candidate]; !ok {
					seenSurfaceID[candidate] = struct{}{}
					return candidate
				}
			}
		}
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
				if surf.Kind == catalog.InteractionCLI && isExecAdapterPath(c.Path, roles) {
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
			surf.ID = uniqueSurfaceID(surf.ID, surf.Kind)
			surfaces = append(surfaces, surf)
		}
		if len(moved) > 0 {
			kind := catalog.InteractionCLI
			for _, c := range moved {
				kind = inferInteractionKind(c.Path, roles)
				break
			}
			if _, ok := seenKind[kind]; !ok {
				surfaces = append(surfaces, catalog.Surface{
					ID:         uniqueSurfaceID(s.ID+"-"+string(kind), kind),
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

	draftLibs := make(map[string]catalog.Library, len(draft.Libraries))
	for _, lib := range draft.Libraries {
		id := strings.TrimSpace(lib.ID)
		if id == "" {
			continue
		}
		draftLibs[id] = lib
	}
	var libraries []catalog.Library
	keptLib := make(map[string]struct{})
	appendLibraryOwns := func(id string, candidates []catalog.Component) []catalog.Component {
		var owns []catalog.Component
		for _, c := range candidates {
			if strings.TrimSpace(c.ID) == "" {
				continue
			}
			if _, ok := seenComp[c.ID]; ok {
				continue
			}
			if c.Layer == catalog.LayerInteraction {
				continue
			}
			if c.Layer != "" && c.Layer != catalog.LayerDomain {
				c.Layer = catalog.LayerDomain
			}
			seenComp[c.ID] = id
			owns = append(owns, c)
		}
		return owns
	}
	for _, lib := range t.Libraries {
		id := strings.TrimSpace(lib.ID)
		if id == "" {
			continue
		}
		draftLib, ok := draftLibs[id]
		if !ok {
			continue // drop invented libraries not present in the discover draft
		}
		purpose := strings.TrimSpace(lib.Purpose)
		if purpose == "" {
			purpose = strings.TrimSpace(draftLib.Purpose)
		}
		if purpose == "" {
			continue
		}
		owns := appendLibraryOwns(id, lib.Owns)
		if len(owns) == 0 {
			owns = appendLibraryOwns(id, draftLib.Owns)
		}
		libraries = append(libraries, catalog.Library{ID: id, Purpose: purpose, Owns: owns})
		keptLib[id] = struct{}{}
	}
	var missingDraftLibIDs []string
	for id := range draftLibs {
		if _, ok := keptLib[id]; ok {
			continue
		}
		missingDraftLibIDs = append(missingDraftLibIDs, id)
	}
	sort.Strings(missingDraftLibIDs)
	for _, id := range missingDraftLibIDs {
		draftLib := draftLibs[id]
		purpose := strings.TrimSpace(draftLib.Purpose)
		if purpose == "" {
			continue
		}
		owns := appendLibraryOwns(id, draftLib.Owns)
		libraries = append(libraries, catalog.Library{ID: id, Purpose: purpose, Owns: owns})
		keptLib[id] = struct{}{}
	}
	t.Libraries = libraries

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
		_, toSlice := slices[b.To]
		_, toLibrary := keptLib[b.To]
		if !toSlice && !toLibrary {
			continue
		}
		sliceBindings = append(sliceBindings, b)
	}
	t.SliceBindings = sliceBindings
	var compBindings []catalog.ComponentBinding
	for _, b := range t.ComponentBindings {
		fromOwner, okFrom := seenComp[b.From]
		toOwner, okTo := seenComp[b.To]
		if !okFrom || !okTo {
			continue
		}
		if _, fromIsLib := keptLib[fromOwner]; fromIsLib {
			// Libraries must not bind outward to slice packages or other libraries.
			if _, toIsLib := keptLib[toOwner]; toIsLib {
				if fromOwner != toOwner {
					continue
				}
			} else if _, toIsSlice := slices[toOwner]; toIsSlice {
				continue
			}
		}
		if fromOwner != toOwner {
			has := false
			for _, sb := range t.SliceBindings {
				if (sb.From == fromOwner && sb.To == toOwner) || (sb.From == toOwner && sb.To == fromOwner) {
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
	return separateHTTPSurfacesFromEntrypoint(t, roles)
}

// separateHTTPSurfacesFromEntrypoint moves server packages off any slice that
// also claims an entrypoint. Sole-importer wiring must not park delivery under the CLI drawer.
func separateHTTPSurfacesFromEntrypoint(t catalog.Typology, roles map[string]packageRoleNode) catalog.Typology {
	if len(roles) == 0 {
		return t
	}
	usedSliceIDs := make(map[string]struct{}, len(t.Slices))
	for _, s := range t.Slices {
		if id := strings.TrimSpace(s.ID); id != "" {
			usedSliceIDs[id] = struct{}{}
		}
	}
	uniqueSliceID := func(base string) string {
		base = strings.TrimSpace(base)
		if base == "" {
			base = "server"
		}
		if _, ok := usedSliceIDs[base]; !ok {
			usedSliceIDs[base] = struct{}{}
			return base
		}
		for n := 2; ; n++ {
			candidate := fmt.Sprintf("%s-%d", base, n)
			if _, ok := usedSliceIDs[candidate]; !ok {
				usedSliceIDs[candidate] = struct{}{}
				return candidate
			}
		}
	}

	var extra []catalog.Slice
	for i := range t.Slices {
		s := &t.Slices[i]
		hasEntrypoint := false
		pathRole := func(path string) string {
			return roles[normalizeRolePath(path)].Role
		}
		for _, c := range s.Owns {
			if pathRole(c.Path) == roleEntrypoint {
				hasEntrypoint = true
			}
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				if pathRole(c.Path) == roleEntrypoint {
					hasEntrypoint = true
				}
			}
		}
		if !hasEntrypoint {
			continue
		}

		var httpOwns []catalog.Component
		var keepOwns []catalog.Component
		for _, c := range s.Owns {
			if pathRole(c.Path) == roleHTTPSurface {
				httpOwns = append(httpOwns, c)
				continue
			}
			keepOwns = append(keepOwns, c)
		}
		s.Owns = keepOwns

		var keepSurfaces []catalog.Surface
		var httpSurfaceComps []catalog.Component
		httpKind := catalog.InteractionAPI
		for _, surf := range s.Surfaces {
			var keepComps []catalog.Component
			for _, c := range surf.Components {
				if pathRole(c.Path) == roleHTTPSurface {
					httpSurfaceComps = append(httpSurfaceComps, c)
					httpKind = inferInteractionKind(c.Path, roles)
					continue
				}
				keepComps = append(keepComps, c)
			}
			surf.Components = keepComps
			if len(keepComps) == 0 {
				continue
			}
			keepSurfaces = append(keepSurfaces, surf)
		}
		s.Surfaces = keepSurfaces

		moved := append(httpOwns, httpSurfaceComps...)
		if len(moved) == 0 {
			continue
		}
		sid := uniqueSliceID(strings.TrimSpace(s.ID) + "-http")
		extra = append(extra, catalog.Slice{
			ID:        sid,
			Objective: "Delivery surface separated from the CLI entrypoint.",
			Surfaces: []catalog.Surface{{
				ID:         sid + "-" + string(httpKind),
				Kind:       httpKind,
				Components: moved,
			}},
		})
	}
	if len(extra) > 0 {
		t.Slices = append(t.Slices, extra...)
	}
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

// scrubForbiddenHTTPEntrypointMerges rewrites cluster proposals that fold server
// packages into an entrypoint slice. Returns a feedback note when a forbidden merge was found.
func scrubForbiddenHTTPEntrypointMerges(proposalMD, rolesYAML string) (string, string) {
	roles := roleByPath(mustParseRoles(rolesYAML))
	if len(roles) == 0 {
		return proposalMD, ""
	}
	var httpPaths, entryPaths []string
	for path, n := range roles {
		switch n.Role {
		case roleHTTPSurface:
			httpPaths = append(httpPaths, path)
		case roleEntrypoint:
			entryPaths = append(entryPaths, path)
		}
	}
	if len(httpPaths) == 0 || len(entryPaths) == 0 {
		return proposalMD, ""
	}
	sort.Strings(httpPaths)
	sort.Strings(entryPaths)

	lower := strings.ToLower(proposalMD)
	found := false
	for _, httpPath := range httpPaths {
		hp := strings.ToLower(httpPath)
		base := strings.ToLower(filepath.Base(httpPath))
		if !strings.Contains(lower, hp) && !strings.Contains(lower, base) {
			continue
		}
		for _, entryPath := range entryPaths {
			ep := strings.ToLower(entryPath)
			entryBase := strings.ToLower(filepath.Base(entryPath))
			// Merge language tying the http package into the entrypoint.
			mentionsEntry := strings.Contains(lower, ep) || strings.Contains(lower, entryBase) ||
				strings.Contains(lower, "entrypoint") || strings.Contains(lower, "cmd/")
			mergeCue := strings.Contains(lower, "merge") || strings.Contains(lower, "→") ||
				strings.Contains(lower, "->") || strings.Contains(lower, "into")
			if mentionsEntry && mergeCue {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return proposalMD, ""
	}

	var b strings.Builder
	b.WriteString(strings.TrimSpace(proposalMD))
	b.WriteString("\n\n## Mechanical override (Majordomo)\n\n")
	b.WriteString("Rejected folding server into entrypoint. Keep delivery on its own slice/surface.\n\n")
	for _, httpPath := range httpPaths {
		fmt.Fprintf(&b, "- MUST NOT merge `%s` (server) into an entrypoint package.\n", httpPath)
	}
	for _, entryPath := range entryPaths {
		fmt.Fprintf(&b, "- Entrypoint `%s` may import HTTP packages as wiring only.\n", entryPath)
	}
	note := "MUST NOT merge server packages into an entrypoint. Keep them on a separate delivery slice/surface; sole importer is a wiring note only."
	return b.String(), note
}

// completeEvidencedLibraryBindings loads the refined catalog and architecture brief,
// appends missing slice→library SliceBindings, and writes the local catalog when changed.
func completeEvidencedLibraryBindings(localCatalogPath, architecturePath string) (bool, error) {
	archMD, err := os.ReadFile(architecturePath)
	if err != nil {
		return false, fmt.Errorf("typology library bindings read architecture: %w", err)
	}
	findings := extractArchitectureFindings(string(archMD))
	if len(findings) == 0 {
		return false, nil
	}
	typo, err := catalog.LoadYAML(localCatalogPath)
	if err != nil {
		return false, fmt.Errorf("typology library bindings load catalog: %w", err)
	}
	updated, changed := applyEvidencedLibraryBindings(typo, findings)
	if !changed {
		return false, nil
	}
	if err := catalog.SaveYAML(localCatalogPath, updated); err != nil {
		return false, fmt.Errorf("typology library bindings save catalog: %w", err)
	}
	return true, nil
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
	rolesRel := strings.TrimSpace(manifest.PackageRolesPath)
	if rolesRel == "" {
		rolesRel = "package_roles.yaml"
	}
	rolesPath := filepath.Join(evidenceDir, rolesRel)
	rolesText, err := os.ReadFile(rolesPath)
	if err != nil {
		return fmt.Errorf("typology refine read package roles: %w", err)
	}
	archDraft, err := os.ReadFile(filepath.Join(analysisDir, analysisDraftArchRel))
	if err != nil {
		return fmt.Errorf("typology refine read architecture draft: %w", err)
	}

	if gen == nil {
		gen = JudgeTypologyRefineGenerator{Gen: judgeGen}
	}
	rolesUpdated, err := inspectLowConfidencePackages(ctx, judgeGen, analysisDir, rolesPath, string(rolesText), string(contractsText))
	if err != nil {
		return err
	}
	if rolesUpdated != "" {
		rolesText = []byte(rolesUpdated)
	}

	out, err := gen.Refine(ctx, TypologyRefineInput{
		RepoID:            manifest.RepoID,
		ModuleScope:       manifest.ModuleScope,
		DraftCatalogYAML:  string(draftYAML),
		GraphText:         string(graphText),
		PackageContracts:  string(contractsText),
		PackageRoles:      string(rolesText),
		ArchitectureDraft: string(archDraft),
		RepoLayout:        strings.Join(collectTopLevelEntries(analysisDir), "\n"),
		ReadmeSnapshot:    capReadmeSnapshot(readText(filepath.Join(analysisDir, "README.md"))),
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

	if changed, err := completeEvidencedLibraryBindings(refinedLocal, archOut); err != nil {
		return err
	} else if changed {
		if err := copyFile(refinedLocal, refinedPath); err != nil {
			return fmt.Errorf("typology refine rewrite catalog after library bindings: %w", err)
		}
		if err := copyFile(refinedPath, snapshotPath); err != nil {
			return fmt.Errorf("typology refine rewrite snapshot after library bindings: %w", err)
		}
		if err := runTypology(ctx, opts.TypologyBinary, analysisDir, "architecture", manifest.ModuleScope,
			"--catalog", refinedLocal, "--out", archOut); err != nil {
			return fmt.Errorf("typology refine architecture after library bindings: %w", err)
		}
		if _, err := os.Stat(archOut); err != nil {
			fallbackArch := filepath.Join(analysisDir, "docs", "architecture", "typology.md")
			if copyErr := copyFile(fallbackArch, archOut); copyErr != nil {
				return fmt.Errorf("typology refine architecture missing after library bindings: %w", err)
			}
		}
		if err := polishTypologyArchitectureBrief(archOut); err != nil {
			return err
		}
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

func capReadmeSnapshot(text string) string {
	runes := []rune(text)
	if len(runes) <= maxReadmeSnapshotRunes {
		return text
	}
	return string(runes[:maxReadmeSnapshotRunes]) + "\n[... README truncated for typology cluster/refine ...]\n"
}

// inspectLowConfidencePackages asks the LLM to classify thin packages from source only.
// Folder names are forbidden as evidence. LLM confidence is capped below mechanical facts.
func inspectLowConfidencePackages(ctx context.Context, gen judge.Generator, analysisDir, rolesPath, rolesYAML, contractsYAML string) (string, error) {
	if gen == nil || !gen.Ready() {
		return rolesYAML, nil
	}
	doc := mustParseRoles(rolesYAML)
	need := packagesNeedingInspect(doc)
	if len(need) == 0 {
		return rolesYAML, nil
	}
	byPath := roleByPath(doc)
	changed := false
	for i, n := range need {
		src := readPackageSources(analysisDir, n.Path)
		if strings.TrimSpace(src) == "" {
			continue
		}
		fields := map[string]interface{}{
			"package_path":      n.Path,
			"package_contracts": contractsSnippetFor(n.Path, contractsYAML),
			"package_source":    src,
			"candidate_role":    n.CandidateRole,
			"current_evidence":  strings.Join(n.Evidence, ", "),
		}
		out, err := gen.Generate(ctx, jmodules.TaskTypologyInspect, fields, i+1)
		if err != nil {
			continue
		}
		role := strings.TrimSpace(stringField(out, "role"))
		evidence := strings.TrimSpace(stringField(out, "evidence"))
		if role == "" || role == roleUnknown {
			continue
		}
		if rejectInspectRoleContradiction(role, src) {
			continue
		}
		node := byPath[normalizeRolePath(n.Path)]
		node.Role = role
		node.Confidence = llmInspectConfidence
		node.InspectedStage = 3
		if evidence != "" {
			node.Evidence = appendUnique(node.Evidence, "llm_inspect:"+evidence)
		} else {
			node.Evidence = appendUnique(node.Evidence, "llm_inspect")
		}
		node.CandidateRole = ""
		byPath[normalizeRolePath(n.Path)] = node
		changed = true
	}
	if !changed {
		return rolesYAML, nil
	}
	var packages []packageRoleNode
	for _, n := range doc.Packages {
		if updated, ok := byPath[normalizeRolePath(n.Path)]; ok {
			packages = append(packages, updated)
			continue
		}
		packages = append(packages, n)
	}
	doc.Packages = packages
	if err := writePackageRoles(rolesPath, doc); err != nil {
		return "", err
	}
	data, err := os.ReadFile(rolesPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func contractsSnippetFor(pkgPath, contractsYAML string) string {
	want := "## ./" + normalizeRolePath(pkgPath)
	alt := "## " + normalizeRolePath(pkgPath)
	idx := strings.Index(contractsYAML, want)
	if idx < 0 {
		idx = strings.Index(contractsYAML, alt)
	}
	if idx < 0 {
		return ""
	}
	rest := contractsYAML[idx:]
	next := strings.Index(rest[3:], "\n## ")
	if next >= 0 {
		return strings.TrimSpace(rest[:next+3])
	}
	return strings.TrimSpace(rest)
}

func appendUnique(list []string, item string) []string {
	for _, v := range list {
		if v == item {
			return list
		}
	}
	return append(list, item)
}
