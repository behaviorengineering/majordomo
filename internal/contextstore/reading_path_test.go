package contextstore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func TestEnsureReadingNav_insertAndReplace(t *testing.T) {
	t.Parallel()
	md := "# Mission\n\nBody.\n"
	got := contextstore.EnsureReadingNav(md, "README.md", "README.md", "architecture.md", "architecture.md", "README.md")
	if !strings.Contains(got, "<!-- majordomo-reading-nav:start -->") {
		t.Fatalf("missing nav start:\n%s", got)
	}
	if !strings.Contains(got, "[Prev: README.md](README.md)") {
		t.Fatalf("missing prev:\n%s", got)
	}
	if !strings.Contains(got, "[Next: architecture.md](architecture.md)") {
		t.Fatalf("missing next:\n%s", got)
	}
	again := contextstore.EnsureReadingNav(got, "README.md", "README.md", "architecture.md", "architecture.md", "README.md")
	if strings.Count(again, "<!-- majordomo-reading-nav:start -->") != 1 {
		t.Fatalf("expected single nav block:\n%s", again)
	}
}

func TestApplyReadingPath_storyAndTypology(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	if err := contextstore.Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(dir, "evidence", "typology")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"architecture_brief.md",
		"cluster_proposal.md",
		"journey.md",
		"human_intervention.md",
		"package_roles.yaml",
	} {
		if err := os.WriteFile(filepath.Join(evidence, name), []byte("# "+name+"\n\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(evidence, "manifest.yaml"), []byte("repo_id: demo\nsource_sha: abc\nmode: fallback\narchitecture_path: architecture_brief.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := contextstore.ApplyReadingPath(dir); err != nil {
		t.Fatal(err)
	}

	rootReadme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	rr := string(rootReadme)
	if !strings.Contains(rr, "## Reading order") {
		t.Fatalf("root README missing TOC:\n%s", rr)
	}
	if !strings.Contains(rr, "evidence/typology/README.md") {
		t.Fatalf("root TOC missing typology handoff:\n%s", rr)
	}
	if !strings.Contains(rr, "<!-- majordomo-reading-nav:start -->") {
		t.Fatalf("root README missing nav:\n%s", rr)
	}

	typoReadme, err := os.ReadFile(filepath.Join(evidence, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	tr := string(typoReadme)
	if !strings.Contains(tr, "architecture_brief.md") {
		t.Fatalf("typology README missing briefing order:\n%s", tr)
	}
	if !strings.Contains(tr, "[Next: architecture_brief.md]") {
		t.Fatalf("typology README missing next:\n%s", tr)
	}

	chron, err := os.ReadFile(filepath.Join(dir, "chronology.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(chron), "evidence/typology/README.md") {
		t.Fatalf("chronology should hand off to typology:\n%s", chron)
	}

	brief, err := os.ReadFile(filepath.Join(evidence, "architecture_brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(brief), "[Prev: README.md](README.md)") {
		t.Fatalf("brief prev should be local README:\n%s", brief)
	}
}

func TestApplyReadingPath_skipsMissingPriority(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	if err := contextstore.Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(dir, "evidence", "typology")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"architecture_brief.md", "cluster_proposal.md", "journey.md", "human_intervention.md"} {
		if err := os.WriteFile(filepath.Join(evidence, name), []byte("# x\n\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := contextstore.ApplyReadingPath(dir); err != nil {
		t.Fatal(err)
	}
	hi, err := os.ReadFile(filepath.Join(evidence, "human_intervention.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hi), "../../README.md") && !strings.Contains(string(hi), "[Next: README.md]") {
		t.Fatalf("human_intervention should next to story README when priority absent:\n%s", hi)
	}
}
