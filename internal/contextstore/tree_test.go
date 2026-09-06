package contextstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
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

func TestValidateTreeTypologyEvidence(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	if err := writeTypologyEvidence(dir, TypologyManifest{
		RepoID:           "demo",
		SourceSHA:        "abc123def456",
		GeneratedAt:      at.Format(time.RFC3339),
		TypologyVersion:  "v0.0.6",
		Mode:             TypologyModeDiscover,
		ModuleScope:      ".",
		SnapshotPath:     "snapshot.yaml",
		ArchitecturePath: "architecture.md",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTree(dir); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTreeFallbackEvidence(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	if err := writeTypologyEvidence(dir, TypologyManifest{
		RepoID:           "demo",
		SourceSHA:        "abc123def456",
		GeneratedAt:      at.Format(time.RFC3339),
		Mode:             TypologyModeFallback,
		ArchitecturePath: "architecture.md",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTree(dir); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTreeRejectsBadTypologySnapshot(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	if err := writeTypologyEvidence(dir, TypologyManifest{
		RepoID:           "demo",
		SourceSHA:        "abc123def456",
		GeneratedAt:      at.Format(time.RFC3339),
		Mode:             TypologyModeDiscover,
		SnapshotPath:     "snapshot.yaml",
		ArchitecturePath: "architecture.md",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence", "typology", "snapshot.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTree(dir); err == nil {
		t.Fatal("expected bad typology snapshot")
	}
}

func writeTypologyEvidence(dir string, m TypologyManifest) error {
	evidenceDir := filepath.Join(dir, "evidence", "typology")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return err
	}
	manifestBytes, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "manifest.yaml"), manifestBytes, 0o644); err != nil {
		return err
	}
	if strings.TrimSpace(m.ArchitecturePath) != "" {
		if err := os.WriteFile(filepath.Join(evidenceDir, m.ArchitecturePath), []byte("# Architecture\n\nSeeded from current HEAD.\n"), 0o644); err != nil {
			return err
		}
	}
	if strings.TrimSpace(m.SnapshotPath) != "" {
		if err := os.WriteFile(filepath.Join(evidenceDir, m.SnapshotPath), []byte("slices:\n  - id: review\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}
