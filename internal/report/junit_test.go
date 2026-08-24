package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeRemovesIllegalXMLCharacters(t *testing.T) {
	got := Sanitize("good\x00text\x1fdone")
	if got != "goodtextdone" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildTestsuiteSkipsAuxFilesAndSADuplicates(t *testing.T) {
	skillDir := t.TempDir()
	perFile := filepath.Join(skillDir, "per-file")
	if err := os.MkdirAll(perFile, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(perFile, "summary.md"), []byte("# summary\n"), 0o644)
	_ = os.WriteFile(filepath.Join(perFile, "foo_session.md"), []byte("# session\n"), 0o644)
	_ = os.WriteFile(filepath.Join(perFile, "service-a.md"), []byte(
		"# src/service_a.py\n"+
			"- [WARN] naming issue (already flagged by static analysis)\n"+
			"- [CRITICAL] null dereference risk\n",
	), 0o644)

	suite, err := BuildTestsuite(skillDir, "pr-review", "pr-review-code")
	if err != nil {
		t.Fatal(err)
	}
	if suite.Tests != "1" {
		t.Fatalf("tests=%s want 1", suite.Tests)
	}
	if suite.Failures != "1" {
		t.Fatalf("failures=%s want 1", suite.Failures)
	}
	if len(suite.Cases) != 1 {
		t.Fatalf("testcase count=%d want 1", len(suite.Cases))
	}
}
