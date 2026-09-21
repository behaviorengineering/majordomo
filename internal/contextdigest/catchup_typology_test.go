package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/typology/pkg/catalog"
)

func TestPruneDraftPackagesAbsentFromRoles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	draftPath := filepath.Join(dir, "typology.yaml")
	rolesPath := filepath.Join(dir, "package_roles.yaml")
	typo := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{{
			ID:        "app",
			Objective: "Keep the packages that still exist.",
			Owns: []catalog.Component{
				{ID: "keep", Path: "internal/keep"},
				{ID: "gone", Path: "internal/gone"},
			},
			Surfaces: []catalog.Surface{{
				ID:   "app-cli",
				Kind: catalog.InteractionCLI,
				Components: []catalog.Component{
					{ID: "cmd", Path: "cmd/demo"},
					{ID: "oldcmd", Path: "cmd/old"},
				},
			}},
		}},
	}
	if err := catalog.SaveYAML(draftPath, typo); err != nil {
		t.Fatal(err)
	}
	roles := `packages:
  - path: internal/keep
    role: logic
  - path: internal/newpkg
    role: logic
  - path: cmd/demo
    role: entrypoint
`
	if err := os.WriteFile(rolesPath, []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pruneDraftPackagesAbsentFromRoles(draftPath, rolesPath); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.LoadYAML(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]struct{}{}
	for _, s := range got.Slices {
		for _, c := range s.Owns {
			paths[normalizeCatalogPath(c.Path)] = struct{}{}
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				paths[normalizeCatalogPath(c.Path)] = struct{}{}
			}
		}
	}
	if _, ok := paths["internal/gone"]; ok {
		t.Fatalf("removed package still on draft: %+v", got.Slices)
	}
	if _, ok := paths["cmd/old"]; ok {
		t.Fatalf("removed command still on draft: %+v", got.Slices)
	}
	if _, ok := paths["internal/keep"]; !ok {
		t.Fatalf("live package dropped: %+v", got.Slices)
	}
	if _, ok := paths["cmd/demo"]; !ok {
		t.Fatalf("live command dropped: %+v", got.Slices)
	}
	if _, ok := paths["internal/newpkg"]; ok {
		t.Fatal("prune must not invent packages; refine adds them")
	}
}

func TestCatchUpRefreshesTypologyWhenCursorMoves(t *testing.T) {
	served := initServedRemote(t)
	ctxBase := config.ContextBranch("demo")
	seedContextRemote(t, served.remote, ctxBase, "demo", served.head)
	cfgDir, workDir := writeDigestConfig(t, served.remote, "demo")
	runGit(t, workDir, "commit", "--allow-empty", "-m", "grow the tree")
	runGit(t, workDir, "push", "origin", "HEAD:main")

	surveys := 0
	runner := bootstrapSurveyRunnerFunc(func(_ context.Context, input BootstrapSurveyInput) error {
		surveys++
		if input.RepoID != "demo" || input.SourceSHA == "" {
			t.Fatalf("survey input repo=%q sha=%q", input.RepoID, input.SourceSHA)
		}
		return writeFallbackSurvey(input)
	})
	t.Setenv("GH_TOKEN_ACME", "tok")
	forge := &Forge{
		SCM: "github", Owner: "acme", Name: "demo", Token: "tok",
		Runner: func(string, []string, []string) (string, error) { return "", nil },
	}
	opts := Options{
		ConfigDir:             cfgDir,
		RepoID:                "demo",
		WorkDir:               workDir,
		SkipStory:             true,
		BootstrapSurveyPolicy: "auto",
		BootstrapSurveyRunner: runner,
		Now:                   time.Date(2026, 8, 28, 5, 0, 0, 0, time.UTC),
		Forge:                 forge,
	}
	res, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "catchup" {
		t.Fatalf("action=%q", res.Action)
	}
	if surveys != 1 {
		t.Fatalf("surveys=%d want 1 on catch-up", surveys)
	}
	if res.CommitsWalked != 1 {
		t.Fatalf("commits=%d", res.CommitsWalked)
	}

	res, err = Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "noop" {
		t.Fatalf("second action=%q", res.Action)
	}
	if surveys != 1 {
		t.Fatalf("caught-up digest refreshed typology again: surveys=%d", surveys)
	}
}

func TestApplyCatchUpSliceDeltaKeepsUnchangedSlices(t *testing.T) {
	t.Parallel()
	prev := twoSliceCatalog()
	roles := map[string]packageRoleNode{
		"internal/keep":  {Path: "internal/keep", Role: "logic"},
		"internal/other": {Path: "internal/other", Role: "logic"},
	}
	got := applyCatchUpSliceDelta(prev, roles, nil)
	if len(got.Changed) != 0 {
		t.Fatalf("changed=%v", got.Changed)
	}
	if got.Catalog.Slices[0].Objective != "Keep the billing packages." {
		t.Fatalf("objective rewritten: %+v", got.Catalog.Slices[0])
	}
}

func TestApplyCatchUpSliceDeltaDropsRemovedPackageOnly(t *testing.T) {
	t.Parallel()
	prev := twoSliceCatalog()
	roles := map[string]packageRoleNode{
		"internal/other": {Path: "internal/other", Role: "logic"},
	}
	got := applyCatchUpSliceDelta(prev, roles, nil)
	if len(got.Catalog.Slices) != 1 || got.Catalog.Slices[0].ID != "other" {
		t.Fatalf("slices=%+v", got.Catalog.Slices)
	}
	if len(got.Changed) != 0 {
		t.Fatalf("empty slice must not be rewritten: %v", got.Changed)
	}
	if got.Catalog.Slices[0].Objective != "Keep the other packages." {
		t.Fatalf("other slice changed: %+v", got.Catalog.Slices[0])
	}
}

func TestApplyCatchUpSliceDeltaAttachesBySoleImporter(t *testing.T) {
	t.Parallel()
	prev := twoSliceCatalog()
	roles := map[string]packageRoleNode{
		"internal/keep":  {Path: "internal/keep", Role: "logic"},
		"internal/other": {Path: "internal/other", Role: "logic"},
		"internal/extra": {Path: "internal/extra", Role: "logic"},
	}
	importers := map[string][]string{"internal/extra": {"internal/keep"}}
	got := applyCatchUpSliceDelta(prev, roles, importers)
	if len(got.Changed) != 1 || got.Changed[0] != "billing" {
		t.Fatalf("changed=%v", got.Changed)
	}
	var billing catalog.Slice
	for _, s := range got.Catalog.Slices {
		if s.ID == "billing" {
			billing = s
		}
		if s.ID == "other" && s.Objective != "Keep the other packages." {
			t.Fatalf("other slice rewritten: %+v", s)
		}
	}
	found := false
	for _, c := range billing.Owns {
		if normalizeCatalogPath(c.Path) == "internal/extra" {
			found = true
		}
	}
	if !found {
		t.Fatalf("extra not on billing: %+v", billing)
	}
}

func TestApplyCatchUpSliceDeltaNewSliceWhenNeighborhoodIsSplit(t *testing.T) {
	t.Parallel()
	prev := twoSliceCatalog()
	roles := map[string]packageRoleNode{
		"internal/keep":  {Path: "internal/keep", Role: "logic"},
		"internal/other": {Path: "internal/other", Role: "logic"},
		"internal/fresh": {Path: "internal/fresh", Role: "logic"},
	}
	got := applyCatchUpSliceDelta(prev, roles, nil)
	if len(got.Changed) != 1 || got.Changed[0] != "fresh" {
		t.Fatalf("changed=%v", got.Changed)
	}
	for _, s := range got.Catalog.Slices {
		if s.ID == "billing" && pathSetKey(slicePackagePaths(s)) != "internal/keep" {
			t.Fatalf("billing changed: %+v", s)
		}
		if s.ID == "other" && pathSetKey(slicePackagePaths(s)) != "internal/other" {
			t.Fatalf("other changed: %+v", s)
		}
	}
}

func twoSliceCatalog() catalog.Typology {
	return catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{
				ID:        "billing",
				Objective: "Keep the billing packages.",
				Owns:      []catalog.Component{{ID: "keep", Path: "internal/keep"}},
			},
			{
				ID:        "other",
				Objective: "Keep the other packages.",
				Owns:      []catalog.Component{{ID: "other", Path: "internal/other"}},
			},
		},
		SliceBindings: []catalog.SliceBinding{{From: "billing", To: "other", Kind: catalog.SliceReads}},
	}
}
