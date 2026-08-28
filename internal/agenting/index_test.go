package agenting

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIndexAndValidate(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "agenting", "index.yaml")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	index := `packs:
  overview:
    modes: [files, summary]
  auth:
    globs: ["**/auth/**"]
    modes: [files]
`
	if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.PackIDs()) != 2 || idx.PackIDs()[0] != "overview" {
		t.Fatalf("order = %v", idx.PackIDs())
	}
}

func TestValidateIndexRejectsUnknownMode(t *testing.T) {
	idx := Index{
		Packs: map[string]Pack{
			"bad": {Modes: []string{"nope"}},
		},
	}
	if err := ValidateIndex(idx); err == nil {
		t.Fatal("expected unknown mode error")
	}
}
