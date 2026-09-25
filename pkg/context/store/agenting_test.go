package contextstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateTreeRequiresGroundingWhenIndexPresent(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	index := `packs:
  overview:
    modes: [files]
  auth:
    modes: [files]
`
	indexPath := filepath.Join(dir, "agenting", "index.yaml")
	if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTree(dir); err == nil {
		t.Fatal("expected missing auth GROUNDING.md")
	}
}

func TestBootstrapIncludesAgenting(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "agenting", "overview", "GROUNDING.md")); err != nil {
		t.Fatal(err)
	}
}
