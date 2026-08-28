package contextstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTreeGolden(t *testing.T) {
	if err := ValidateTree(filepath.Join("testdata", "valid")); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTreeEmptyDir(t *testing.T) {
	if err := ValidateTree(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTreeMissingFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("testdata", "valid")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(dir, "mission.md")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTree(dir); err == nil {
		t.Fatal("expected missing mission.md")
	}
}

func TestValidateTreeBadMeta(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("testdata", "valid")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bad, err := os.ReadFile(filepath.Join("testdata", "bad-meta", "meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.yaml"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTree(dir); err == nil {
		t.Fatal("expected bad meta")
	}
}
