package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	"github.com/behaviorengineering/typology/catalog"
)

// appendEvidenceGroundingIssues adds fail-closed catalog checks for ownership shape.
func appendEvidenceGroundingIssues(typo catalog.Typology, issues []string) []string {
	issues = appendHollowSliceOwnershipIssues(typo, issues)
	issues = appendDuplicatePackageOwnerIssues(typo, issues)
	return issues
}

// appendConstraintClaimIssues fail-closes when structured objective claims intersect
// the slice must_not union from capability constraints.
func appendConstraintClaimIssues(
	typo catalog.Typology,
	constraints packageCapabilityConstraintsDoc,
	claims sliceObjectiveClaimsDoc,
	issues []string,
) []string {
	byPath := constraintsByPath(constraints)
	bySlice := claimsBySliceID(claims)
	knownSlices := map[string]struct{}{}
	for _, s := range typo.Slices {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			continue
		}
		knownSlices[id] = struct{}{}
		paths := slicePackagePaths(s)
		mustNot := sliceMustNotUnion(paths, byPath)
		claimed, ok := bySlice[id]
		if !ok || len(claimed) == 0 {
			// Require claims whenever owned packages carry constraints.
			needsClaims := false
			for _, p := range paths {
				if c, ok := byPath[normalizeRolePath(p)]; ok && !packageUnconstrained(c) {
					needsClaims = true
					break
				}
			}
			if needsClaims {
				issues = append(issues, fmt.Sprintf(
					"%s: slice %q is missing objective_claims for constrained packages",
					typologypack.CriterionIDRoleGrounding, id,
				))
			}
			continue
		}
		if hit := intersectStrings(claimed, mustNot); len(hit) > 0 {
			issues = append(issues, fmt.Sprintf(
				"%s: slice %q objective_claims %v intersect must_not %v; rewrite the objective and claims to match owned package capabilities (name who fills DTOs; do not assign sync/merge to data_shape packages)",
				typologypack.CriterionIDRoleGrounding, id, claimed, hit,
			))
		}
	}
	for id := range bySlice {
		if _, ok := knownSlices[id]; !ok {
			issues = append(issues, fmt.Sprintf(
				"%s: objective_claims references unknown slice %q",
				typologypack.CriterionIDRoleGrounding, id,
			))
		}
	}
	return issues
}

func appendHollowSliceOwnershipIssues(typo catalog.Typology, issues []string) []string {
	ownerByBase := packageBaseOwners(typo)
	for _, s := range typo.Slices {
		if len(slicePackagePaths(s)) > 0 {
			continue
		}
		id := strings.TrimSpace(s.ID)
		if id == "" {
			continue
		}
		if owner, ok := ownerByBase[id]; ok && owner != id {
			issues = append(issues, fmt.Sprintf(
				"%s: slice %q owns no packages but %q owns package base %q; remount SliceBindings onto %q and delete the hollow slice (one package, one owner).",
				typologypack.CriterionIDSliceOwnership, id, owner, id, owner,
			))
			continue
		}
		if sliceAppearsInBindings(typo, id) {
			issues = append(issues, fmt.Sprintf(
				"%s: slice %q owns no packages but still appears in SliceBindings; remount those edges onto the slice that owns the packages, or delete the hollow slice.",
				typologypack.CriterionIDSliceOwnership, id,
			))
		}
	}
	return issues
}

func appendDuplicatePackageOwnerIssues(typo catalog.Typology, issues []string) []string {
	owners := map[string][]string{}
	for _, s := range typo.Slices {
		id := strings.TrimSpace(s.ID)
		for _, p := range slicePackagePaths(s) {
			owners[p] = append(owners[p], id)
		}
	}
	for path, ids := range owners {
		uniq := uniqueStrings(ids)
		if len(uniq) < 2 {
			continue
		}
		issues = append(issues, fmt.Sprintf(
			"%s: package %q is owned by multiple slices (%s); keep one owner.",
			typologypack.CriterionIDSliceOwnership, path, strings.Join(uniq, ", "),
		))
	}
	return issues
}

func slicePackagePaths(s catalog.Slice) []string {
	var out []string
	for _, c := range s.Owns {
		if p := normalizeCatalogPath(c.Path); p != "" {
			out = append(out, p)
		}
	}
	for _, surf := range s.Surfaces {
		for _, c := range surf.Components {
			if p := normalizeCatalogPath(c.Path); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func packageBaseOwners(typo catalog.Typology) map[string]string {
	out := map[string]string{}
	for _, s := range typo.Slices {
		id := strings.TrimSpace(s.ID)
		for _, p := range slicePackagePaths(s) {
			base := filepath.Base(p)
			if base == "" || base == "." {
				continue
			}
			out[base] = id
		}
	}
	return out
}

func sliceAppearsInBindings(typo catalog.Typology, sliceID string) bool {
	for _, b := range typo.SliceBindings {
		if strings.TrimSpace(b.From) == sliceID || strings.TrimSpace(b.To) == sliceID {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// collapseHollowPackageSlices remounts bindings from package-less slices onto the
// slice that owns the matching package base (unfinished HTTP rename / split).
func collapseHollowPackageSlices(t catalog.Typology) catalog.Typology {
	ownerByBase := packageBaseOwners(t)
	remap := map[string]string{}
	for _, s := range t.Slices {
		if len(slicePackagePaths(s)) > 0 {
			continue
		}
		id := strings.TrimSpace(s.ID)
		if id == "" {
			continue
		}
		if owner, ok := ownerByBase[id]; ok && owner != id {
			remap[id] = owner
		}
	}
	if len(remap) == 0 {
		return t
	}

	var keep []catalog.Slice
	for _, s := range t.Slices {
		id := strings.TrimSpace(s.ID)
		if _, drop := remap[id]; drop {
			continue
		}
		keep = append(keep, s)
	}
	t.Slices = keep

	var bindings []catalog.SliceBinding
	seen := map[string]struct{}{}
	for _, b := range t.SliceBindings {
		from := strings.TrimSpace(b.From)
		to := strings.TrimSpace(b.To)
		if next, ok := remap[from]; ok {
			from = next
		}
		if next, ok := remap[to]; ok {
			to = next
		}
		if from == "" || to == "" || from == to {
			continue
		}
		key := from + "->" + to + ":" + string(b.Kind)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		b.From = from
		b.To = to
		bindings = append(bindings, b)
	}
	t.SliceBindings = bindings
	return t
}

func loadTypologyFromYAML(raw string) (catalog.Typology, error) {
	tmp, err := os.CreateTemp("", "majordomo-ground-*.yaml")
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("evidence grounding create temp: %w", err)
	}
	path := tmp.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := tmp.WriteString(raw); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return catalog.Typology{}, fmt.Errorf("evidence grounding write temp: %w; close temp: %v", err, closeErr)
		}
		return catalog.Typology{}, fmt.Errorf("evidence grounding write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return catalog.Typology{}, fmt.Errorf("evidence grounding close temp: %w", err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("evidence grounding load catalog: %w", err)
	}
	return typo, nil
}

// assertEvidenceGroundingYAML fail-closes when catalog objectives or ownership
// drift from package_roles / one-owner rules.
func assertEvidenceGroundingYAML(refinedYAML, _ string) error {
	typo, err := loadTypologyFromYAML(refinedYAML)
	if err != nil {
		return fmt.Errorf("evidence grounding load catalog: %w", err)
	}
	issues := appendEvidenceGroundingIssues(typo, nil)
	if len(issues) == 0 {
		return nil
	}
	return fmt.Errorf("evidence grounding:\n%s", strings.Join(issues, "\n"))
}

// assertEvidenceGroundingBeforeStory re-checks the on-disk refined catalog before
// the teaching story paraphrases it.
func assertEvidenceGroundingBeforeStory(ctxDir string) error {
	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	if !fileExists(manifestPath) {
		return nil
	}
	manifest, err := contextstore.ParseTypologyManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(manifest.RefineStatus, contextstore.TypologyRefineSkipped) {
		return nil
	}
	catalogRel := strings.TrimSpace(manifest.RefinedSnapshotPath)
	if catalogRel == "" {
		catalogRel = strings.TrimSpace(manifest.SnapshotPath)
	}
	if catalogRel == "" {
		catalogRel = refinedSnapshotRel
	}
	catalogPath := filepath.Join(evidenceDir, catalogRel)
	if !fileExists(catalogPath) {
		catalogPath = filepath.Join(evidenceDir, "snapshot.yaml")
	}
	if !fileExists(catalogPath) {
		return nil
	}
	refined, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("evidence grounding read catalog: %w", err)
	}
	rolesRel := strings.TrimSpace(manifest.PackageRolesPath)
	if rolesRel == "" {
		rolesRel = packageRolesRel
	}
	rolesPath := filepath.Join(evidenceDir, rolesRel)
	rolesYAML := ""
	if fileExists(rolesPath) {
		b, err := os.ReadFile(rolesPath)
		if err != nil {
			return fmt.Errorf("evidence grounding read roles: %w", err)
		}
		rolesYAML = string(b)
	}
	return assertEvidenceGroundingYAML(string(refined), rolesYAML)
}

// assertArchitectureKeepsGroundedObjectives fail-closes catch-up story when
// root architecture.md drops a constrained slice's grounded catalog objective.
func assertArchitectureKeepsGroundedObjectives(ctxDir string) error {
	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	if !fileExists(manifestPath) {
		return nil
	}
	manifest, err := contextstore.ParseTypologyManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(manifest.RefineStatus, contextstore.TypologyRefineSkipped) {
		return nil
	}
	constraintsRel := strings.TrimSpace(manifest.PackageCapabilityConstraintsPath)
	if constraintsRel == "" {
		constraintsRel = packageCapabilityConstraintsRel
	}
	constraintsPath := filepath.Join(evidenceDir, constraintsRel)
	if !fileExists(constraintsPath) {
		return nil
	}
	constraints, err := loadCapabilityConstraints(constraintsPath)
	if err != nil {
		return fmt.Errorf("catch-up grounding read constraints: %w", err)
	}
	catalogRel := strings.TrimSpace(manifest.RefinedSnapshotPath)
	if catalogRel == "" {
		catalogRel = refinedSnapshotRel
	}
	catalogPath := filepath.Join(evidenceDir, catalogRel)
	if !fileExists(catalogPath) {
		return nil
	}
	raw, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("catch-up grounding read catalog: %w", err)
	}
	typo, err := loadTypologyFromYAML(string(raw))
	if err != nil {
		return fmt.Errorf("catch-up grounding load catalog: %w", err)
	}
	archPath := filepath.Join(ctxDir, "architecture.md")
	if !fileExists(archPath) {
		return nil
	}
	arch, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("catch-up grounding read architecture: %w", err)
	}
	archText := string(arch)
	byPath := constraintsByPath(constraints)
	for _, s := range typo.Slices {
		obj := strings.TrimSpace(s.Objective)
		if obj == "" {
			continue
		}
		mustNot := sliceMustNotUnion(slicePackagePaths(s), byPath)
		if len(mustNot) == 0 {
			continue
		}
		if !strings.Contains(archText, obj) {
			return fmt.Errorf(
				"%s: architecture.md dropped grounded objective for constrained slice %q; catch-up must preserve catalog objectives",
				typologypack.CriterionIDRoleGrounding, s.ID,
			)
		}
	}
	return nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
