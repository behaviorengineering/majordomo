package staging_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/staging"
)

// TestPrepProducesReviewableManifest builds a fixture repo and checks Go prep
// writes a coherent batch plan and reviewable manifest.
func TestPrepProducesReviewableManifest(t *testing.T) {
	fixture := t.TempDir()
	initFixtureRepo(t, fixture)

	goStaging := filepath.Join(t.TempDir(), "go-staging")
	_ = os.MkdirAll(goStaging, 0o755)

	prev, _ := os.Getwd()
	if err := os.Chdir(fixture); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(prev) }()

	err := staging.Run(staging.Options{
		BaseBranch: "main",
		StagingDir: goStaging,
		BatchSize:  15,
	})
	if err != nil {
		t.Fatalf("go prep: %v", err)
	}

	plan := readJSON(t, filepath.Join(goStaging, "batch-plan.json"))
	skills := toStringSlice(plan["skills"])
	if len(skills) == 0 {
		t.Fatal("expected at least one skill in batch-plan")
	}
	total, ok := asInt(plan["total_batches"])
	if !ok || total < 1 {
		t.Fatalf("expected total_batches >= 1, got %v", plan["total_batches"])
	}

	manifest := readJSON(t, filepath.Join(goStaging, "manifest.json"))
	files := reviewableFiles(manifest)
	if len(files) == 0 {
		t.Fatal("expected reviewable files in manifest")
	}
	agents := toStringMapLists(manifest["review_agents"])
	if len(agents) == 0 {
		t.Fatal("expected review_agents in manifest")
	}
}

func initFixtureRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main", "--template=")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")

	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src/a.py", "def a():\n    return 1\n")
	write("src/b.py", "from src.a import a\n\ndef b():\n    return a()\n")
	write("docs/intro.md", "# Intro\n\nSee [guide](guide.md).\n")
	write("docs/guide.md", "# Guide\n\nDetails.\n")
	write("README.md", "# Fixture\n")
	run("add", ".")
	run("commit", "-m", "initial")

	remote := filepath.Join(dir, "..", "fixture-remote.git")
	_ = os.RemoveAll(remote)
	r2 := exec.Command("git", "init", "--bare", remote)
	if out, err := r2.CombinedOutput(); err != nil {
		t.Fatalf("bare: %v\n%s", err, out)
	}
	run("remote", "add", "origin", remote)
	run("push", "-u", "origin", "main")

	run("checkout", "-b", "feature")
	write("src/a.py", "def a():\n    return 2\n")
	write("src/c.py", "from src.b import b\n\ndef c():\n    return b()\n")
	write("docs/guide.md", "# Guide\n\nUpdated details.\n\nBack to [intro](intro.md).\n")
	write("config/app.yaml", "name: fixture\n")
	run("add", ".")
	run("commit", "-m", "feature changes")
}

func reviewableFiles(m map[string]any) []string {
	tasks := toMapSlice(m["reviewable"])
	out := make([]string, 0, len(tasks))
	seen := map[string]struct{}{}
	for _, task := range tasks {
		f, _ := task["file"].(string)
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func toStringSlice(v any) []string {
	arr, _ := v.([]any)
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

func toMapSlice(v any) []map[string]any {
	arr, _ := v.([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, x := range arr {
		m, _ := x.(map[string]any)
		out = append(out, m)
	}
	return out
}

func toStringMapLists(v any) map[string][]string {
	m, _ := v.(map[string]any)
	out := map[string][]string{}
	for k, val := range m {
		out[k] = toStringSlice(val)
	}
	return out
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}
