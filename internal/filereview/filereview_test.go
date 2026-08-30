package filereview_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/filereview"
)

func TestParseAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo_bar.md")
	_ = os.WriteFile(path, []byte("# src/foo/bar.go\n\n- [CRITICAL] nil deref\n- [WARN] naming\n"), 0o644)
	rep, err := filereview.ParseMarkdownReport(path, "foo_bar")
	if err != nil {
		t.Fatal(err)
	}
	if rep.File != "src/foo/bar.go" || len(rep.Findings) != 2 {
		t.Fatalf("%#v", rep)
	}
	revs := []filereview.Reviewable{{File: "src/foo/bar.go", Slug: "foo_bar"}}
	if err := filereview.ValidateReports(revs, map[string]filereview.Report{"foo_bar": rep}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMissingSlug(t *testing.T) {
	revs := []filereview.Reviewable{{File: "a.go", Slug: "a"}}
	err := filereview.ValidateReports(revs, map[string]filereview.Report{})
	if err == nil || !strings.Contains(err.Error(), "missing report") {
		t.Fatalf("got %v", err)
	}
}

func TestFormatRoundTrip(t *testing.T) {
	rep := filereview.Report{
		File: "x.go", Slug: "x",
		Findings: []filereview.Finding{{Severity: filereview.SeverityInfo, Text: "note"}},
	}
	md := filereview.FormatMarkdown(rep)
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	_ = os.WriteFile(path, []byte(md), 0o644)
	got, err := filereview.ParseMarkdownReport(path, "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 1 || got.Findings[0].Text != "note" {
		t.Fatalf("%#v", got)
	}
}

func TestRunHappyPath(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "batch")
	skillOut := filepath.Join(dir, "out", "pr-review-code")
	_ = os.MkdirAll(staging, 0o755)
	_ = os.WriteFile(filepath.Join(staging, "manifest.json"), []byte(`{
  "reviewable": [{"file":"a.go","slug":"a"},{"file":"b.go","slug":"b"}]
}`), 0o644)

	err := filereview.Run(filereview.Options{
		StagingDir: staging,
		SkillOut:   skillOut,
		MaxRetries: 1,
		Judge: func() error {
			per := filereview.PerFileDir(skillOut)
			_ = os.MkdirAll(per, 0o755)
			_ = os.WriteFile(filepath.Join(per, "a.md"), []byte("# a.go\n\nNo issues found.\n"), 0o644)
			_ = os.WriteFile(filepath.Join(per, "b.md"), []byte("# b.go\n\n- [WARN] odd\n"), 0o644)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(skillOut, "findings.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunRetriesThenFails(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "batch")
	skillOut := filepath.Join(dir, "out", "pr-review-code")
	_ = os.MkdirAll(staging, 0o755)
	_ = os.WriteFile(filepath.Join(staging, "manifest.json"), []byte(`{
  "reviewable": [{"file":"a.go","slug":"a"}]
}`), 0o644)
	n := 0
	err := filereview.Run(filereview.Options{
		StagingDir: staging,
		SkillOut:   skillOut,
		MaxRetries: 1,
		Judge: func() error {
			n++
			per := filereview.PerFileDir(skillOut)
			_ = os.MkdirAll(per, 0o755)
			// Missing a.md → collect/validate fails
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if n != 2 {
		t.Fatalf("attempts=%d", n)
	}
	if _, err := os.Stat(filepath.Join(staging, "filereview_feedback.md")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReviewables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	_ = os.WriteFile(path, []byte(`{"reviewable":[{"file":"x.go","slug":"x"},{"file":"x.go","slug":"x"}]}`), 0o644)
	revs, err := filereview.LoadReviewables(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 1 {
		t.Fatalf("%#v", revs)
	}
}

func TestJudgeRequired(t *testing.T) {
	err := filereview.Run(filereview.Options{StagingDir: t.TempDir(), SkillOut: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "Judge required") {
		t.Fatalf("got %v", err)
	}
}
