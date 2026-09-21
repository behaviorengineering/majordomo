package contextdigest

import (
	"testing"

	"github.com/behaviorengineering/typology/pkg/catalog"
)

func TestApplyAcceptedMergesFoldsPackages(t *testing.T) {
	t.Parallel()
	draft := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "local", Objective: "Local forge", Owns: []catalog.Component{{Path: "internal/localgit"}}},
			{ID: "remote", Objective: "Remote forge", Owns: []catalog.Component{{Path: "internal/remotegit"}}},
			{ID: "board", Objective: "Board", Owns: []catalog.Component{{Path: "internal/board"}}},
		},
	}
	verdicts := []clusterMergeVerdict{
		{ID: "git-adapters", Packages: []string{"internal/localgit", "internal/remotegit"}, Verdict: verdictAccept},
		{ID: "noise", Packages: []string{"internal/a", "internal/b"}, Verdict: verdictReject},
		{ID: "overlay-x", Packages: []string{"internal/board", "internal/x"}, Verdict: verdictOverlay},
	}
	got, err := applyAcceptedMerges(draft, verdicts)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]catalog.Slice{}
	for _, s := range got.Slices {
		byID[s.ID] = s
	}
	git, ok := byID["git-adapters"]
	if !ok {
		t.Fatalf("slices=%v", got.Slices)
	}
	pkgs := normalizePackageList(slicePackagePaths(git))
	if len(pkgs) != 2 || pkgs[0] != "internal/localgit" || pkgs[1] != "internal/remotegit" {
		t.Fatalf("git owns=%v", pkgs)
	}
	if _, ok := byID["board"]; !ok {
		t.Fatal("board slice should remain")
	}
	// Donors emptied of folded packages.
	if local, ok := byID["local"]; ok && len(slicePackagePaths(local)) > 0 {
		t.Fatalf("local still owns %v", slicePackagePaths(local))
	}
	if remote, ok := byID["remote"]; ok && len(slicePackagePaths(remote)) > 0 {
		t.Fatalf("remote still owns %v", slicePackagePaths(remote))
	}
}

func TestApplyAcceptedMergesKeepsDraftIDWhenSameOwner(t *testing.T) {
	t.Parallel()
	draft := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "review", Objective: "Review work", Owns: []catalog.Component{
				{Path: "internal/staging"},
				{Path: "internal/judge/evaluation/summary"},
				{Path: "internal/judge/evaluation/digest"},
			}},
		},
	}
	got, err := applyAcceptedMerges(draft, []clusterMergeVerdict{
		{ID: "judge-eval", Packages: []string{
			"internal/judge/evaluation/summary",
			"internal/judge/evaluation/digest",
		}, Verdict: verdictAccept},
	})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]catalog.Slice{}
	for _, s := range got.Slices {
		byID[s.ID] = s
	}
	if _, ok := byID["judge-eval"]; ok {
		t.Fatalf("same-owner accept must not rename draft id; slices=%v", got.Slices)
	}
	if _, ok := byID["review"]; !ok {
		t.Fatalf("expected review survivor; slices=%v", got.Slices)
	}
}

func TestApplyAcceptedMergesLeavesRejectAndOverlay(t *testing.T) {
	t.Parallel()
	draft := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "a", Objective: "A", Owns: []catalog.Component{{Path: "internal/a"}}},
			{ID: "b", Objective: "B", Owns: []catalog.Component{{Path: "internal/b"}}},
		},
	}
	got, err := applyAcceptedMerges(draft, []clusterMergeVerdict{
		{ID: "ab", Packages: []string{"internal/a", "internal/b"}, Verdict: verdictOverlay},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Slices) != 2 {
		t.Fatalf("len=%d", len(got.Slices))
	}
	if len(slicePackagePaths(got.Slices[0])) != 1 || len(slicePackagePaths(got.Slices[1])) != 1 {
		t.Fatalf("unexpected fold: %#v", got.Slices)
	}
}

func TestJoinSliceCatalogReplacesByID(t *testing.T) {
	t.Parallel()
	skeleton := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "board", Objective: "placeholder", Owns: []catalog.Component{{Path: "internal/board"}}},
			{ID: "ops", Objective: "ops", Owns: []catalog.Component{{Path: "internal/ops"}}},
		},
		Libraries: []catalog.Library{{ID: "shared", Purpose: "shared", Owns: []catalog.Component{{Path: "internal/lib"}}}},
	}
	frag := catalog.Slice{
		ID:        "board",
		Objective: "Shared board payload shapes.",
		Owns:      []catalog.Component{{ID: "board-core", Path: "internal/board"}},
		Surfaces: []catalog.Surface{{
			ID: "board-api", Kind: catalog.InteractionAPI,
			Components: []catalog.Component{{Path: "internal/board"}},
		}},
	}
	got, err := joinSliceCatalog(skeleton, []catalog.Slice{frag})
	if err != nil {
		t.Fatal(err)
	}
	if got.Slices[0].Objective != "Shared board payload shapes." {
		t.Fatalf("objective=%q", got.Slices[0].Objective)
	}
	if len(got.Slices[0].Surfaces) != 1 {
		t.Fatalf("surfaces=%v", got.Slices[0].Surfaces)
	}
	if got.Slices[1].Objective != "ops" {
		t.Fatalf("ops mutated: %q", got.Slices[1].Objective)
	}
	if len(got.Libraries) != 1 || got.Libraries[0].ID != "shared" {
		t.Fatalf("libraries=%v", got.Libraries)
	}
}

func TestJoinSliceCatalogUnknownIDFails(t *testing.T) {
	t.Parallel()
	skeleton := catalog.Typology{
		ID:     "demo",
		Slices: []catalog.Slice{{ID: "board", Objective: "x"}},
	}
	_, err := joinSliceCatalog(skeleton, []catalog.Slice{{ID: "invented", Objective: "nope"}})
	if err == nil {
		t.Fatal("expected error")
	}
}
