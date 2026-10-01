package sa

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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
		Context:    contextWithDeadline(t),
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
		Runner: func(scriptPath, slug, image, command, repoRoot string, files []string) error {
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

func contextWithDeadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}
