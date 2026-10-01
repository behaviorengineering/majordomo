package satools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDockerfiles(t *testing.T) {
	dir := t.TempDir()
	sa := filepath.Join(dir, "dockerfiles", "sa-tools")
	mustMkdirAll(t, sa)
	mustWriteFile(t, filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644)
	mustWriteFile(t, filepath.Join(sa, "notes.txt"), []byte("x"), 0o644)
	got, err := discoverDockerfiles(sa)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "ruff.Dockerfile" {
		t.Fatalf("got %v", got)
	}
	if toolName(got[0]) != "ruff" {
		t.Fatalf("tool name %q", toolName(got[0]))
	}
}

func TestDryRun(t *testing.T) {
	dir := t.TempDir()
	sa := filepath.Join(dir, "dockerfiles", "sa-tools")
	mustMkdirAll(t, sa)
	mustWriteFile(t, filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644)
	mustWriteFile(t, filepath.Join(dir, "go.mod"), []byte("module example\n"), 0o644)
	err := Run(Options{RepoRoot: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCorpRequiresEnv(t *testing.T) {
	dir := t.TempDir()
	sa := filepath.Join(dir, "dockerfiles", "sa-tools")
	mustMkdirAll(t, sa)
	mustWriteFile(t, filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644)
	t.Setenv("REGISTRY_USER", "")
	t.Setenv("REGISTRY_TOKEN", "")
	t.Setenv("PACKAGE_REGISTRY_HOST", "")
	err := Run(Options{RepoRoot: dir, Corp: true})
	if err == nil {
		t.Fatal("expected corp env error")
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
