package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAllWithCapAndContext(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "a.txt")
	lines := []string{"L1", "L2", "L3", "L4"}
	if err := os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "manifest.json")
	man := `{
  "reviewable": [
    {
      "file": "src/a.py",
      "input_file": "` + strings.ReplaceAll(input, `\`, `\\`) + `",
      "agent_context": {"z": 1, "a": "b"}
    }
  ]
}`
	if err := os.WriteFile(manifest, []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "all-diffs.txt")
	capN := 2
	if err := BuildAll(BuildAllOptions{Manifest: manifest, Output: out, Cap: &capN}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "=== FILE: src/a.py ===\n") {
		t.Fatalf("missing file header:\n%s", text)
	}
	if !strings.Contains(text, `=== AGENT CONTEXT: {"a":"b","z":1} ===`) {
		t.Fatalf("missing sorted agent context:\n%s", text)
	}
	if !strings.Contains(text, "L1\nL2\n") {
		t.Fatalf("missing capped lines:\n%s", text)
	}
	if !strings.Contains(text, "[... 2 lines omitted") {
		t.Fatalf("missing omission marker:\n%s", text)
	}
}

func TestBuildAllMissingInputWritesHeaderOnly(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.json")
	man := `{"reviewable":[{"file":"missing.py","input_file":"/no/such/file"}]}`
	if err := os.WriteFile(manifest, []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "all-diffs.txt")
	if err := BuildAll(BuildAllOptions{Manifest: manifest, Output: out}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	want := "=== FILE: missing.py ===\n\n"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
