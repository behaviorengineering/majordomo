package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/behaviorengineering/typology/pkg/catalog"
	"gopkg.in/yaml.v3"
)

// catchUpSliceDelta is the catalog after a catch-up package diff.
// Changed lists slice ids whose package set differs from the previous map.
type catchUpSliceDelta struct {
	Catalog catalog.Typology
	Changed []string
}

// applyCatchUpSliceDelta drops packages the current tree no longer has and
// places new packages on the slice that already owns their sole importer.
// A new package with no single owning slice becomes its own slice.
// Slices whose package set is unchanged are copied through.
func applyCatchUpSliceDelta(prev catalog.Typology, roles map[string]packageRoleNode, importers map[string][]string) catchUpSliceDelta {
	live := map[string]struct{}{}
	for path := range roles {
		if n := normalizeRolePath(path); n != "" {
			live[n] = struct{}{}
		}
	}
	before := map[string]string{}
	next := prev
	next.Slices = append([]catalog.Slice(nil), prev.Slices...)
	for i := range next.Slices {
		s := next.Slices[i]
		before[strings.TrimSpace(s.ID)] = pathSetKey(slicePackagePaths(s))
		next.Slices[i] = dropSlicePackagesAbsent(s, live)
	}
	next.Libraries = append([]catalog.Library(nil), prev.Libraries...)
	for i := range next.Libraries {
		next.Libraries[i] = dropLibraryPackagesAbsent(next.Libraries[i], live)
	}

	owned := map[string]string{}
	for _, s := range next.Slices {
		id := strings.TrimSpace(s.ID)
		for _, p := range slicePackagePaths(s) {
			owned[p] = id
		}
	}
	var fresh []string
	for path := range live {
		if _, ok := owned[path]; ok {
			continue
		}
		fresh = append(fresh, path)
	}
	sort.Strings(fresh)
	for _, path := range fresh {
		if sid, ok := sliceOwningNeighborhood(path, owned, importers); ok {
			for i := range next.Slices {
				if strings.TrimSpace(next.Slices[i].ID) != sid {
					continue
				}
				placePackageOnSlice(&next.Slices[i], path, roles)
				owned[path] = sid
				break
			}
			continue
		}
		sid := uniqueCatchUpSliceID(next, sliceIDFromPath(path))
		created := catalog.Slice{
			ID:        sid,
			Objective: fmt.Sprintf("The %s slice holds %s.", sid, path),
		}
		placePackageOnSlice(&created, path, roles)
		next.Slices = append(next.Slices, created)
		owned[path] = sid
	}

	var kept []catalog.Slice
	var changed []string
	for _, s := range next.Slices {
		id := strings.TrimSpace(s.ID)
		if id == "" || len(slicePackagePaths(s)) == 0 {
			continue
		}
		kept = append(kept, s)
		if before[id] != pathSetKey(slicePackagePaths(s)) {
			changed = append(changed, id)
		}
	}
	next.Slices = kept
	next.SliceBindings = dropBindingsToMissingSlices(next)
	sort.Strings(changed)
	return catchUpSliceDelta{Catalog: next, Changed: changed}
}

func dropSlicePackagesAbsent(s catalog.Slice, live map[string]struct{}) catalog.Slice {
	var owns []catalog.Component
	for _, c := range s.Owns {
		if _, ok := live[normalizeCatalogPath(c.Path)]; ok {
			owns = append(owns, c)
		}
	}
	s.Owns = owns
	var surfaces []catalog.Surface
	for _, surf := range s.Surfaces {
		var comps []catalog.Component
		for _, c := range surf.Components {
			if _, ok := live[normalizeCatalogPath(c.Path)]; ok {
				comps = append(comps, c)
			}
		}
		if len(comps) == 0 {
			continue
		}
		surf.Components = comps
		surfaces = append(surfaces, surf)
	}
	s.Surfaces = surfaces
	return s
}

func dropLibraryPackagesAbsent(lib catalog.Library, live map[string]struct{}) catalog.Library {
	var owns []catalog.Component
	for _, c := range lib.Owns {
		if _, ok := live[normalizeCatalogPath(c.Path)]; ok {
			owns = append(owns, c)
		}
	}
	lib.Owns = owns
	return lib
}

func sliceOwningNeighborhood(path string, owned map[string]string, importers map[string][]string) (string, bool) {
	imps := importers[normalizeRolePath(path)]
	if len(imps) == 0 {
		return "", false
	}
	sliceID := ""
	for _, imp := range imps {
		sid, ok := owned[normalizeRolePath(imp)]
		if !ok || sid == "" {
			return "", false
		}
		if sliceID == "" {
			sliceID = sid
			continue
		}
		if sliceID != sid {
			return "", false
		}
	}
	return sliceID, true
}

func placePackageOnSlice(s *catalog.Slice, path string, roles map[string]packageRoleNode) {
	if s == nil {
		return
	}
	comp := catalog.Component{ID: filepath.Base(path), Path: path}
	if !looksLikeInteractionPath(path, roles) {
		comp.Layer = catalog.LayerDomain
		s.Owns = append(s.Owns, comp)
		return
	}
	appendHTTPSurfaceComponents(s, inferInteractionKind(path, roles), []catalog.Component{comp})
}

func sliceIDFromPath(path string) string {
	base := filepath.Base(normalizeRolePath(path))
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		return "slice"
	}
	return base
}

func uniqueCatchUpSliceID(typo catalog.Typology, base string) string {
	used := map[string]struct{}{}
	for _, s := range typo.Slices {
		if id := strings.TrimSpace(s.ID); id != "" {
			used[id] = struct{}{}
		}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func pathSetKey(paths []string) string {
	norm := normalizePackageList(paths)
	sort.Strings(norm)
	return strings.Join(norm, "\n")
}

func dropBindingsToMissingSlices(typo catalog.Typology) []catalog.SliceBinding {
	ids := map[string]struct{}{}
	for _, s := range typo.Slices {
		if id := strings.TrimSpace(s.ID); id != "" {
			ids[id] = struct{}{}
		}
	}
	var out []catalog.SliceBinding
	for _, b := range typo.SliceBindings {
		if _, ok := ids[strings.TrimSpace(b.From)]; !ok {
			continue
		}
		if _, ok := ids[strings.TrimSpace(b.To)]; !ok {
			continue
		}
		out = append(out, b)
	}
	return out
}

func loadRefinedCatalogIfPresent(path string) (catalog.Typology, bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return catalog.Typology{}, false, nil
		}
		return catalog.Typology{}, false, fmt.Errorf("typology catch-up stat refined catalog: %w", err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		return catalog.Typology{}, false, fmt.Errorf("typology catch-up load refined catalog: %w", err)
	}
	if len(typo.Slices) == 0 {
		return catalog.Typology{}, false, nil
	}
	return typo, true, nil
}

// rewriteChangedSlices grounds ledger and catalog text for slices whose package
// set changed. Unchanged slices stay on catalog. Missing RLM configuration keeps
// the mechanical membership and leaves the previous wording in place.
func rewriteChangedSlices(ctx context.Context, opts Options, analysisDir, evidenceDir string, current catalog.Typology, changed []string) (catalog.Typology, error) {
	if len(changed) == 0 {
		return current, nil
	}
	if ctx == nil {
		return catalog.Typology{}, fmt.Errorf("typology catch-up: context is required")
	}
	subset := catalog.Typology{ID: current.ID, Slices: slicesByID(current, changed)}
	if len(subset.Slices) == 0 {
		return current, nil
	}
	ledgerBuilder, err := newLedgerBuilderFromOpts(ctx, opts, analysisDir)
	if err != nil {
		logf("WARN", "typology catch-up slice rewrite skipped: %v", err)
		return current, nil
	}
	rolesDoc, err := loadPackageRolesFromEvidenceDir(evidenceDir)
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("typology catch-up roles for slice rewrite: %w", err)
	}
	constraints := buildCapabilityConstraints(rolesDoc)
	ledgerDoc, issues, err := ledgerBuilder.BuildSliceLedger(ctx, sliceLedgerBuildRequest{
		AnalysisDir:   analysisDir,
		EvidenceDir:   evidenceDir,
		DraftTypo:     subset,
		Constraints:   constraints,
		DigestCache:   opts.DigestCache,
		DigestSkips:   opts.DigestSkips,
		DigestModelID: opts.DigestModelID,
	})
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("typology catch-up ledger: %w", err)
	}
	if len(issues) > 0 {
		return catalog.Typology{}, fmt.Errorf("typology catch-up ledger: %s", strings.Join(issues, "\n"))
	}
	assembler, err := newCatalogAssemblerFromOpts(ctx, opts, analysisDir)
	if err != nil {
		logf("WARN", "typology catch-up catalog rewrite skipped: %v", err)
		return stampChangedSliceObjectives(current, ledgerDoc, changed), nil
	}
	rolesYAML, err := marshalRoles(rolesDoc)
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("typology catch-up marshal roles: %w", err)
	}
	constraintsYAML, err := marshalConstraints(constraints)
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("typology catch-up marshal constraints: %w", err)
	}
	assembled, err := assembler.AssembleSlices(ctx, sliceCatalogAssembleRequest{
		FoldedTypo:      subset,
		LedgerDoc:       ledgerDoc,
		EvidenceDir:     evidenceDir,
		RolesYAML:       rolesYAML,
		ConstraintsYAML: constraintsYAML,
		ReadmeSnapshot:  capReadmeSnapshot(readText(filepath.Join(analysisDir, "README.md"))),
	})
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("typology catch-up slice catalog: %w", err)
	}
	if len(assembled.Issues) > 0 {
		return catalog.Typology{}, fmt.Errorf("typology catch-up slice catalog: %s", strings.Join(assembled.Issues, "\n"))
	}
	merged := replaceSlices(current, assembled.Fragments)
	merged = stampChangedSliceObjectives(merged, ledgerDoc, changed)
	if err := mergeChangedLedger(evidenceDir, ledgerDoc, changed); err != nil {
		return catalog.Typology{}, err
	}
	return merged, nil
}

func slicesByID(typo catalog.Typology, ids []string) []catalog.Slice {
	want := map[string]struct{}{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = struct{}{}
		}
	}
	var out []catalog.Slice
	for _, s := range typo.Slices {
		if _, ok := want[strings.TrimSpace(s.ID)]; ok {
			out = append(out, s)
		}
	}
	return out
}

func replaceSlices(current catalog.Typology, fragments []catalog.Slice) catalog.Typology {
	byID := map[string]catalog.Slice{}
	for _, s := range fragments {
		if id := strings.TrimSpace(s.ID); id != "" {
			byID[id] = s
		}
	}
	for i := range current.Slices {
		id := strings.TrimSpace(current.Slices[i].ID)
		if repl, ok := byID[id]; ok {
			current.Slices[i] = repl
			delete(byID, id)
		}
	}
	var extra []string
	for id := range byID {
		extra = append(extra, id)
	}
	sort.Strings(extra)
	for _, id := range extra {
		current.Slices = append(current.Slices, byID[id])
	}
	return current
}

func stampChangedSliceObjectives(typo catalog.Typology, ledger sliceObjectiveLedgerDoc, changed []string) catalog.Typology {
	want := map[string]struct{}{}
	for _, id := range changed {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = struct{}{}
		}
	}
	byID := map[string]sliceObjectiveLedgerEntry{}
	for _, e := range ledger.Slices {
		if id := strings.TrimSpace(e.ID); id != "" {
			byID[id] = e
		}
	}
	for i := range typo.Slices {
		id := strings.TrimSpace(typo.Slices[i].ID)
		if _, ok := want[id]; !ok {
			continue
		}
		if obj := strings.TrimSpace(byID[id].Objective); obj != "" {
			typo.Slices[i].Objective = obj
		}
	}
	return typo
}

func mergeChangedLedger(evidenceDir string, fresh sliceObjectiveLedgerDoc, changed []string) error {
	path := filepath.Join(evidenceDir, sliceObjectiveLedgerRel)
	prev := sliceObjectiveLedgerDoc{}
	if data, err := os.ReadFile(path); err == nil {
		parsed, parseErr := parseObjectiveLedgerYAML(string(data))
		if parseErr != nil {
			return fmt.Errorf("typology catch-up read ledger: %w", parseErr)
		}
		prev = parsed
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("typology catch-up read ledger: %w", err)
	}
	want := map[string]struct{}{}
	for _, id := range changed {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = struct{}{}
		}
	}
	freshByID := map[string]sliceObjectiveLedgerEntry{}
	for _, e := range fresh.Slices {
		if id := strings.TrimSpace(e.ID); id != "" {
			freshByID[id] = e
		}
	}
	var merged []sliceObjectiveLedgerEntry
	seen := map[string]struct{}{}
	for _, e := range prev.Slices {
		id := strings.TrimSpace(e.ID)
		if repl, ok := freshByID[id]; ok {
			merged = append(merged, repl)
			seen[id] = struct{}{}
			continue
		}
		if _, drop := want[id]; drop {
			continue
		}
		merged = append(merged, e)
		seen[id] = struct{}{}
	}
	var extra []string
	for id := range freshByID {
		if _, ok := seen[id]; !ok {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		merged = append(merged, freshByID[id])
	}
	prev.Slices = merged
	if err := writeObjectiveLedger(path, prev); err != nil {
		return fmt.Errorf("typology catch-up write ledger: %w", err)
	}
	return nil
}

func saveRefinedCatalog(path string, typo catalog.Typology) error {
	if err := catalog.SaveYAML(path, typo); err != nil {
		return fmt.Errorf("typology catch-up save refined catalog: %w", err)
	}
	return nil
}

func marshalRoles(doc packageRolesDoc) (string, error) {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
