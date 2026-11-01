package satools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverDockerfiles(t *testing.T) {
	dir := t.TempDir()
	sa := filepath.Join(dir, "dockerfiles", "sa-tools")
	mustMkdir(t, sa)
	mustWrite(t, filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644)
	mustWrite(t, filepath.Join(sa, "notes.txt"), []byte("x"), 0o644)
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
	mustMkdir(t, sa)
	mustWrite(t, filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644)
	mustWrite(t, filepath.Join(dir, "go.mod"), []byte("module example\n"), 0o644)
	err := Run(Options{Context: testContext(t), RepoRoot: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCorpRequiresEnv(t *testing.T) {
	dir := t.TempDir()
	sa := filepath.Join(dir, "dockerfiles", "sa-tools")
	mustMkdir(t, sa)
	mustWrite(t, filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644)
	t.Setenv("REGISTRY_USER", "")
	t.Setenv("REGISTRY_TOKEN", "")
	t.Setenv("PACKAGE_REGISTRY_HOST", "")
	err := Run(Options{Context: testContext(t), RepoRoot: dir, Corp: true})
	if err == nil {
		t.Fatal("expected corp env error")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}
