package contextdigest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
)

// assertAcceptedMembership reports illegal multi-package ownership that is not an accept verdict
// and was not already co-owned in the draft catalog.
func assertAcceptedMembership(refined, draft catalog.Typology, verdicts []clusterMergeVerdict) []string {
	accepted := acceptedPackageSets(verdicts)
	draftSets := multiPackageOwnerSetIndex(draft)
	var issues []string
	for _, set := range multiPackageOwnerSets(refined) {
		key := packageSetKey(set.Packages)
		if key == "" {
			continue
		}
		if _, ok := accepted[key]; ok {
			continue
		}
		if draftSets.contains(key) {
			continue
		}
		issues = append(issues, fmt.Sprintf(
			"refined %s %q co-owns [%s] without an accept cluster_merge verdict; fold only accepted merges (or keep draft co-ownership)",
			set.Kind, set.OwnerID, strings.Join(set.Packages, ", "),
		))
	}
	sort.Strings(issues)
	return issues
}

type ownerSet struct {
	Kind     string // slice | library
	OwnerID  string
	Packages []string
}

type ownerSetIndex map[string]struct{}

func (i ownerSetIndex) contains(key string) bool {
	_, ok := i[key]
	return ok
}

func multiPackageOwnerSets(typo catalog.Typology) []ownerSet {
	var out []ownerSet
	for _, s := range typo.Slices {
		pkgs := normalizePackageList(slicePackagePaths(s))
		if len(pkgs) >= 2 {
			out = append(out, ownerSet{Kind: "slice", OwnerID: strings.TrimSpace(s.ID), Packages: pkgs})
		}
	}
	for _, lib := range typo.Libraries {
		lp := normalizePackageList(libraryPackagePaths(lib))
		if len(lp) >= 2 {
			out = append(out, ownerSet{
				Kind:     "library",
				OwnerID:  strings.TrimSpace(lib.ID),
				Packages: lp,
			})
		}
	}
	return out
}

func multiPackageOwnerSetIndex(typo catalog.Typology) ownerSetIndex {
	out := ownerSetIndex{}
	for _, o := range multiPackageOwnerSets(typo) {
		out[packageSetKey(o.Packages)] = struct{}{}
	}
	return out
}

func libraryPackagePaths(lib catalog.Library) []string {
	var out []string
	for _, c := range lib.Owns {
		if p := normalizeCatalogPath(c.Path); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitIllegalMembershipToDraftOwners moves packages from illegal multi-owners back onto
// their draft slice owners. Does not invent new slice ids.
func splitIllegalMembershipToDraftOwners(refined, draft catalog.Typology, verdicts []clusterMergeVerdict) (catalog.Typology, bool) {
	issues := assertAcceptedMembership(refined, draft, verdicts)
	if len(issues) == 0 {
		return refined, false
	}
	accepted := acceptedPackageSets(verdicts)
	draftOwner := draftPackageOwner(draft)
	changed := false

	for si := range refined.Slices {
		s := &refined.Slices[si]
		if cleaned, ok := filterOwnsToAcceptedOrDraft(s.Owns, accepted, draft, draftOwner, s.ID); ok {
			s.Owns = cleaned
			changed = true
		}
	}
	for li := range refined.Libraries {
		lib := &refined.Libraries[li]
		if cleaned, ok := filterOwnsToAcceptedOrDraft(lib.Owns, accepted, draft, draftOwner, ""); ok {
			lib.Owns = cleaned
			changed = true
		}
	}

	// Remount packages that lost ownership onto their draft owners.
	claimed := map[string]struct{}{}
	for _, s := range refined.Slices {
		for _, p := range slicePackagePaths(s) {
			claimed[normalizeRolePath(p)] = struct{}{}
		}
	}
	for _, lib := range refined.Libraries {
		for _, p := range libraryPackagePaths(lib) {
			claimed[normalizeRolePath(p)] = struct{}{}
		}
	}
	for path, ownerID := range draftOwner {
		if _, ok := claimed[path]; ok {
			continue
		}
		for si := range refined.Slices {
			if strings.TrimSpace(refined.Slices[si].ID) != ownerID {
				continue
			}
			refined.Slices[si].Owns = append(refined.Slices[si].Owns, catalog.Component{Path: path})
			claimed[path] = struct{}{}
			changed = true
			break
		}
	}
	return refined, changed
}

func filterOwnsToAcceptedOrDraft(
	owns []catalog.Component,
	accepted map[string]struct{},
	draft catalog.Typology,
	draftOwner map[string]string,
	sliceID string,
) ([]catalog.Component, bool) {
	pkgs := normalizePackageList(componentPaths(owns))
	if len(pkgs) < 2 {
		return owns, false
	}
	key := packageSetKey(pkgs)
	if _, ok := accepted[key]; ok {
		return owns, false
	}
	if multiPackageOwnerSetIndex(draft).contains(key) {
		return owns, false
	}
	// Keep only packages whose draft owner is this slice; others remount later.
	// Empty sliceID (libraries) drops all packages from the illegal set.
	var kept []catalog.Component
	for _, c := range owns {
		p := normalizeRolePath(normalizeCatalogPath(c.Path))
		if p == "" {
			continue
		}
		if sliceID != "" && draftOwner[p] == strings.TrimSpace(sliceID) {
			kept = append(kept, c)
		}
	}
	return kept, true
}

func componentPaths(owns []catalog.Component) []string {
	var out []string
	for _, c := range owns {
		if p := normalizeCatalogPath(c.Path); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func draftPackageOwner(draft catalog.Typology) map[string]string {
	out := map[string]string{}
	for _, s := range draft.Slices {
		id := strings.TrimSpace(s.ID)
		for _, p := range slicePackagePaths(s) {
			out[normalizeRolePath(p)] = id
		}
	}
	for _, lib := range draft.Libraries {
		id := strings.TrimSpace(lib.ID)
		for _, p := range libraryPackagePaths(lib) {
			// Libraries are not slice owners; remount uses slice draft owners only.
			if _, ok := out[normalizeRolePath(p)]; !ok {
				out[normalizeRolePath(p)] = id
			}
		}
	}
	return out
}
