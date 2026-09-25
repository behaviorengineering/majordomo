package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageSourceHashStableAcrossMarkdownNoise(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "board")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "board.go"), []byte("package board\n\ntype P struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := PackageSourceHash(dir, "internal/board")
	if err != nil {
		t.Fatal(err)
	}
	b, err := PackageSourceHash(dir, "./internal/board")
	if err != nil {
		t.Fatal(err)
	}
	if a == "" || a != b {
		t.Fatalf("unstable hash %q vs %q", a, b)
	}
	if err := os.WriteFile(filepath.Join(pkg, "board.go"), []byte("package board\n\ntype Q struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := PackageSourceHash(dir, "internal/board")
	if err != nil {
		t.Fatal(err)
	}
	if c == a {
		t.Fatal("source change must invalidate hash")
	}
}
