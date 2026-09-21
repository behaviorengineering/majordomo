package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/typology/pkg/catalog"
)

func TestDecideTypologyPromote_skipModes(t *testing.T) {
	t.Parallel()
	d, err := decideTypologyPromote(contextstore.TypologyModeDiscover, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if d.ShouldPromote || d.SkipReason != "mode_discover" {
		t.Fatalf("got %+v", d)
	}
	d, err = decideTypologyPromote(contextstore.TypologyModeFallback, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if d.ShouldPromote || d.SkipReason != "mode_fallback" {
		t.Fatalf("got %+v", d)
	}
}

func TestDecideTypologyPromote_equalAndDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confirmed := filepath.Join(dir, "confirmed.yaml")
	refinedSame := filepath.Join(dir, "refined-same.yaml")
	refinedDrift := filepath.Join(dir, "refined-drift.yaml")

	base := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "ops", Objective: "ops"},
		},
	}
	if err := catalog.SaveYAML(confirmed, base); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveYAML(refinedSame, base); err != nil {
		t.Fatal(err)
	}
	drift := base
	drift.Slices = append(append([]catalog.Slice{}, base.Slices...), catalog.Slice{
		ID: "ops-http", Objective: "http",
	})
	if err := catalog.SaveYAML(refinedDrift, drift); err != nil {
		t.Fatal(err)
	}

	eq, err := decideTypologyPromote(contextstore.TypologyModeReuse, confirmed, refinedSame)
	if err != nil {
		t.Fatal(err)
	}
	if eq.ShouldPromote || eq.SkipReason != "catalogs_equal" {
		t.Fatalf("equal: %+v", eq)
	}

	chg, err := decideTypologyPromote(contextstore.TypologyModeReuse, confirmed, refinedDrift)
	if err != nil {
		t.Fatal(err)
	}
	if !chg.ShouldPromote || chg.SkipReason != "" {
		t.Fatalf("drift: %+v", chg)
	}
	if len(chg.Refined.Slices) != 2 {
		t.Fatalf("refined slices=%d", len(chg.Refined.Slices))
	}
}

func TestDecideTypologyPromote_missingPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confirmed := filepath.Join(dir, "confirmed.yaml")
	if err := catalog.SaveYAML(confirmed, catalog.Typology{ID: "demo"}); err != nil {
		t.Fatal(err)
	}
	d, err := decideTypologyPromote(contextstore.TypologyModeReuse, confirmed, filepath.Join(dir, "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if d.ShouldPromote || d.SkipReason != "missing_refined" {
		t.Fatalf("got %+v", d)
	}
}

func TestTypologyPromotePRBody(t *testing.T) {
	t.Parallel()
	body := typologyPromotePRBody(
		"demo",
		"abc123",
		"https://example.com/pr/9",
		[]string{"SliceBinding ops -> ops-http missing"},
		"- promote ops-http",
		"## Brief\nDeclare reads bindings.",
	)
	for _, want := range []string{
		"Typology catalog promote",
		"`.typology/typology.yaml`",
		"demo",
		"abc123",
		"https://example.com/pr/9",
		"SliceBinding ops -> ops-http missing",
		"promote ops-http",
		"Declare reads bindings",
		"does **not** auto-merge",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestPromoteConfirmedTypology_discoverSkips(t *testing.T) {
	t.Parallel()
	ctxDir := t.TempDir()
	ev := filepath.Join(ctxDir, typologyEvidenceDir)
	if err := os.MkdirAll(ev, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := contextstore.TypologyManifest{
		RepoID:              "demo",
		SourceSHA:           "abc",
		Mode:                contextstore.TypologyModeDiscover,
		ArchitecturePath:    contextstore.TypologyArchitectureBriefPath,
		RefinedSnapshotPath: refinedSnapshotRel,
	}
	if err := writeTypologyManifest(ev, manifest); err != nil {
		t.Fatal(err)
	}
	served := t.TempDir()
	got, err := promoteConfirmedTypology(finishParams{
		cfg:           config.RepoConfig{Repository: config.Repository{ID: "demo"}},
		forge:         &Forge{SCM: "github"},
		ctxDir:        ctxDir,
		servedGit:     &Git{Dir: served},
		defaultBranch: "main",
	}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if got.PR != "" || got.SkipReason != "mode_discover" {
		t.Fatalf("got %+v", got)
	}
}

func TestPromoteConfirmedTypology_opensProductPR(t *testing.T) {
	served := initServedRemote(t)
	parent := t.TempDir()
	runGit(t, parent, "clone", served.remote, "work")
	clone := filepath.Join(parent, "work")
	runGit(t, clone, "config", "user.email", "t@example.com")
	runGit(t, clone, "config", "user.name", "t")
	runGit(t, clone, "checkout", "-B", "main", "origin/main")

	confirmed := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "ops", Objective: "ops"},
		},
	}
	if err := catalog.SaveYAML(filepath.Join(clone, confirmedCatalogRel), confirmed); err != nil {
		t.Fatal(err)
	}
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "commit", "-m", "add typology")
	runGit(t, clone, "push", "origin", "HEAD:main")

	ctxDir := t.TempDir()
	ev := filepath.Join(ctxDir, typologyEvidenceDir)
	if err := os.MkdirAll(ev, 0o755); err != nil {
		t.Fatal(err)
	}
	refined := confirmed
	refined.Slices = append(append([]catalog.Slice{}, confirmed.Slices...), catalog.Slice{
		ID: "ops-http", Objective: "http gateway",
	})
	if err := catalog.SaveYAML(filepath.Join(ev, refinedSnapshotRel), refined); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ev, contextstore.TypologyArchitectureBriefPath), []byte(""+
		"## Findings\n\n"+
		"- SliceBinding ops -> ops-http missing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ev, "pr_priority.md"), []byte("- promote ops-http\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := contextstore.TypologyManifest{
		RepoID:              "demo",
		SourceSHA:           "abc",
		Mode:                contextstore.TypologyModeReuse,
		ArchitecturePath:    contextstore.TypologyArchitectureBriefPath,
		RefinedSnapshotPath: refinedSnapshotRel,
	}
	if err := writeTypologyManifest(ev, manifest); err != nil {
		t.Fatal(err)
	}

	var createdBase, createdHead, createdTitle, createdBody string
	var listCalls, createCalls int
	f := &Forge{
		SCM: "github", Owner: "acme", Name: "demo", Token: "tok",
		Runner: func(name string, args []string, _ []string) (string, error) {
			if name != "gh" || len(args) < 2 || args[0] != "pr" {
				return "", nil
			}
			switch args[1] {
			case "list":
				listCalls++
				return "", nil
			case "create":
				createCalls++
				for i := 0; i < len(args)-1; i++ {
					switch args[i] {
					case "--base":
						createdBase = args[i+1]
					case "--head":
						createdHead = args[i+1]
					case "--title":
						createdTitle = args[i+1]
					case "--body-file":
						raw, err := os.ReadFile(args[i+1])
						if err != nil {
							return "", err
						}
						createdBody = string(raw)
					}
				}
				return "https://example.com/acme/demo/pull/77", nil
			}
			return "", nil
		},
	}

	got, err := promoteConfirmedTypology(finishParams{
		cfg:           config.RepoConfig{Repository: config.Repository{ID: "demo"}},
		forge:         f,
		ctxDir:        ctxDir,
		servedGit:     &Git{Dir: clone},
		token:         "",
		scm:           "github",
		defaultBranch: "main",
		defaultHEAD:   served.head,
	}, "https://example.com/context/9")
	if err != nil {
		t.Fatal(err)
	}
	if got.PR == "" || got.SkipReason != "" {
		t.Fatalf("got %+v", got)
	}
	if listCalls < 1 || createCalls != 1 {
		t.Fatalf("list=%d create=%d", listCalls, createCalls)
	}
	wantHead := config.TypologyPromoteUpdateBranch("demo")
	if createdBase != "main" || createdHead != wantHead {
		t.Fatalf("base=%q head=%q want base=main head=%q", createdBase, createdHead, wantHead)
	}
	if !strings.Contains(createdTitle, "Promote typology catalog") {
		t.Fatalf("title=%q", createdTitle)
	}
	if !strings.Contains(createdBody, "SliceBinding ops -> ops-http missing") {
		t.Fatalf("body=%q", createdBody)
	}

	// Restack: second call should edit existing PR.
	var edited bool
	f.Runner = func(name string, args []string, _ []string) (string, error) {
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "list" {
			return "77", nil
		}
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "edit" {
			edited = true
			return "", nil
		}
		return "", nil
	}
	got2, err := promoteConfirmedTypology(finishParams{
		cfg:           config.RepoConfig{Repository: config.Repository{ID: "demo"}},
		forge:         f,
		ctxDir:        ctxDir,
		servedGit:     &Git{Dir: clone},
		defaultBranch: "main",
		defaultHEAD:   served.head,
	}, "https://example.com/context/9")
	if err != nil {
		t.Fatal(err)
	}
	if got2.PR != "77" || !edited {
		t.Fatalf("restack got=%+v edited=%v", got2, edited)
	}
}
