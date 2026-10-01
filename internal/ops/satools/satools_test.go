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
	if err := os.MkdirAll(sa, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sa, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err := os.MkdirAll(sa, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := Run(Options{Context: ctx, RepoRoot: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCorpRequiresEnv(t *testing.T) {
	dir := t.TempDir()
	sa := filepath.Join(dir, "dockerfiles", "sa-tools")
	if err := os.MkdirAll(sa, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sa, "ruff.Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REGISTRY_USER", "")
	t.Setenv("REGISTRY_TOKEN", "")
	t.Setenv("PACKAGE_REGISTRY_HOST", "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := Run(Options{Context: ctx, RepoRoot: dir, Corp: true})
	if err == nil {
		t.Fatal("expected corp env error")
	}
}
