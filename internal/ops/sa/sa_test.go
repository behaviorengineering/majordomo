package sa

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunFiltersByGlob(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: demo
staticAnalysis:
  - tool: ruff
    image: fake/sa-ruff:1
    command: check
    glob: "**/*.py"
  - tool: eslint
    image: fake/sa-eslint:1
    command: lint
    glob: "**/*.js"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scripts, "run-sa-tool.sh"), []byte("#!/bin/true\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var ran []string
	err := Run(Options{
		ConfigDir:  cfgDir,
		RepoID:     "demo",
		BaseBranch: "main",
		RepoRoot:   dir,
		ScriptsDir: scripts,
		ChangedFiles: []string{
			"src/a.py",
			"README.md",
			"web/app.js",
		},
		Runner: func(_ context.Context, scriptPath, slug, image, command, repoRoot string, files []string) error {
			ran = append(ran, slug)
			switch slug {
			case "ruff":
				if len(files) != 1 || files[0] != "src/a.py" {
					t.Fatalf("ruff files=%v", files)
				}
			case "eslint":
				if len(files) != 1 || files[0] != "web/app.js" {
					t.Fatalf("eslint files=%v", files)
				}
			default:
				t.Fatalf("unexpected slug %s", slug)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ran) != 2 {
		t.Fatalf("ran=%v", ran)
	}
}

func TestRunReturnsToolFailure(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := []byte(`
scm: github
repository:
  owner: acme
  name: demo
staticAnalysis:
  - tool: ruff
    image: fake/sa-ruff:1
    command: check
`)
	if err := os.WriteFile(filepath.Join(configDir, "demo.yaml"), config, 0o644); err != nil {
		t.Fatal(err)
	}
	err := Run(Options{
		ConfigDir:    configDir,
		RepoID:       "demo",
		BaseBranch:   "main",
		RepoRoot:     dir,
		ScriptsDir:   dir,
		ChangedFiles: []string{"src/a.py"},
		Runner: func(_ context.Context, scriptPath, slug, image, command, repoRoot string, files []string) error {
			return errors.New("tool failed")
		},
	})
	if err == nil {
		t.Fatal("expected static-analysis failure")
	}
}
