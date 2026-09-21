package contextdigest

import (
	"fmt"
	"strings"

	"github.com/behaviorengineering/typology/pkg/catalog"
	"gopkg.in/yaml.v3"
)

// applyAcceptedMerges folds draft slice ownership for accept cluster verdicts.
// Overlay and reject verdicts leave membership alone. Single-package accepts are no-ops.
// The survivor slice id prefers the verdict id when set; otherwise the first package's draft owner.
func applyAcceptedMerges(draft catalog.Typology, verdicts []clusterMergeVerdict) (catalog.Typology, error) {
	out, err := cloneTypology(draft)
	if err != nil {
		return catalog.Typology{}, err
	}
	for _, v := range verdicts {
		if strings.ToLower(strings.TrimSpace(v.Verdict)) != verdictAccept {
			continue
		}
		pkgs := normalizePackageList(v.Packages)
		if len(pkgs) < 2 {
			continue
		}
		owners := draftPackageOwner(out)
		survivorIdx := findSliceIndexByID(out, strings.TrimSpace(v.ID))
		if survivorIdx < 0 {
			if ownerID := owners[pkgs[0]]; ownerID != "" {
				survivorIdx = findSliceIndexByID(out, ownerID)
			}
		}
		created := false
		if survivorIdx < 0 {
			out.Slices = append(out.Slices, catalog.Slice{
				ID:        strings.TrimSpace(v.ID),
				Objective: "placeholder",
			})
			survivorIdx = len(out.Slices) - 1
			created = true
		}
		if id := strings.TrimSpace(v.ID); id != "" {
			ownerSet := map[string]struct{}{}
			for _, p := range pkgs {
				if o := owners[p]; o != "" {
					ownerSet[o] = struct{}{}
				}
			}
			// Same-neighborhood accept: keep the draft owner id so the meaning ledger still keys.
			// Cross-slice accept: adopt the verdict id as the merged teaching name.
			if created || len(ownerSet) != 1 {
				out.Slices[survivorIdx].ID = id
			}
		}

		var moved []catalog.Component
		seenPath := map[string]struct{}{}
		for si := range out.Slices {
			kept := make([]catalog.Component, 0, len(out.Slices[si].Owns))
			for _, c := range out.Slices[si].Owns {
				p := normalizeRolePath(normalizeCatalogPath(c.Path))
				if p == "" {
					kept = append(kept, c)
					continue
				}
				if !containsNormalizedPath(pkgs, p) {
					kept = append(kept, c)
					continue
				}
				if _, ok := seenPath[p]; !ok {
					seenPath[p] = struct{}{}
					moved = append(moved, c)
				}
			}
			out.Slices[si].Owns = kept
		}
		// Also strip accepted packages from libraries so fold ownership is unambiguous.
		for li := range out.Libraries {
			kept := make([]catalog.Component, 0, len(out.Libraries[li].Owns))
			for _, c := range out.Libraries[li].Owns {
				p := normalizeRolePath(normalizeCatalogPath(c.Path))
				if p == "" || !containsNormalizedPath(pkgs, p) {
					kept = append(kept, c)
					continue
				}
				if _, ok := seenPath[p]; !ok {
					seenPath[p] = struct{}{}
					moved = append(moved, c)
				}
			}
			out.Libraries[li].Owns = kept
		}
		out.Slices[survivorIdx].Owns = append(out.Slices[survivorIdx].Owns, moved...)
	}
	return out, nil
}

// joinSliceCatalog replaces skeleton slices with RLM fragments by id.
// Fragments must reference existing skeleton slice ids. Libraries and bindings stay from the skeleton.
func joinSliceCatalog(skeleton catalog.Typology, fragments []catalog.Slice) (catalog.Typology, error) {
	out, err := cloneTypology(skeleton)
	if err != nil {
		return catalog.Typology{}, err
	}
	byID := map[string]int{}
	for i, s := range out.Slices {
		id := strings.TrimSpace(s.ID)
		if id != "" {
			byID[id] = i
		}
	}
	for _, frag := range fragments {
		id := strings.TrimSpace(frag.ID)
		if id == "" {
			return catalog.Typology{}, fmt.Errorf("slice catalog join: fragment missing id")
		}
		idx, ok := byID[id]
		if !ok {
			return catalog.Typology{}, fmt.Errorf("slice catalog join: unknown fragment id %q", id)
		}
		out.Slices[idx] = frag
	}
	return out, nil
}

func cloneTypology(in catalog.Typology) (catalog.Typology, error) {
	data, err := yaml.Marshal(&in)
	if err != nil {
		return catalog.Typology{}, fmt.Errorf("clone typology encode: %w", err)
	}
	var out catalog.Typology
	if err := yaml.Unmarshal(data, &out); err != nil {
		return catalog.Typology{}, fmt.Errorf("clone typology decode: %w", err)
	}
	return out, nil
}

func findSliceIndexByID(typo catalog.Typology, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return -1
	}
	for i, s := range typo.Slices {
		if strings.TrimSpace(s.ID) == id {
			return i
		}
	}
	return -1
}

func containsNormalizedPath(pkgs []string, path string) bool {
	path = normalizeRolePath(path)
	for _, p := range pkgs {
		if p == path {
			return true
		}
	}
	return false
}
