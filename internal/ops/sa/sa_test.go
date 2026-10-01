package sa

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func mustMkdir(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, perm); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
}

func TestRunFiltersByGlob(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	mustMkdir(t, cfgDir, 0o755)
	mustWrite(t, filepath.Join(cfgDir, "_defaults.yaml"), []byte("{}\n"), 0o644)
	mustWrite(t, filepath.Join(cfgDir, "demo.yaml"), []byte(`
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
`), 0o644)

	scripts := filepath.Join(dir, "scripts")
	mustMkdir(t, scripts, 0o755)
	mustWrite(t, filepath.Join(scripts, "run-sa-tool.sh"), []byte("#!/bin/true\n"), 0o755)

	var ran []string
	err := Run(Options{
		Context:    context.Background(),
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
