package cache

import (
	"os"
	"path/filepath"
	"strings"
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

func TestFormatStatsLine(t *testing.T) {
	line := FormatStatsLine(DigestRunStats{
		InspectHits: 2, InspectMisses: 1, LedgerHits: 3, LedgerMisses: 0,
		TokensSavedTotal: 900, TokensSavedPrompt: 700, TokensSavedCompletion: 200,
	})
	for _, want := range []string{"inspect_hits=2", "ledger_hits=3", "estimated_tokens_saved=900"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
}
