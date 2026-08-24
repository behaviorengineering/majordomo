package staging_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/staging"
)

// TestPrepGoldenVsPython builds a fixture repo, runs Go prep and Python git-diff-prep,
// and compares key fields of manifest.json / batch-plan.json.
func TestPrepGoldenVsPython(t *testing.T) {
	python := "python3"
	if _, err := exec.LookPath(python); err != nil {
		t.Skip("python3 not available")
	}
	modRoot := findModuleRoot(t)
	pyScript := filepath.Join(modRoot, "pipelines", "scripts", "git-diff-prep.py")
	if _, err := os.Stat(pyScript); err != nil {
		t.Fatalf("python script missing: %v", err)
	}

	fixture := t.TempDir()
	initFixtureRepo(t, fixture)

	goStaging := filepath.Join(t.TempDir(), "go-staging")
	pyStaging := filepath.Join(t.TempDir(), "py-staging")
	_ = os.MkdirAll(goStaging, 0o755)
	_ = os.MkdirAll(pyStaging, 0o755)

	// Run from fixture cwd so relative paths match.
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

	cmd := exec.Command(python, pyScript, "main", pyStaging)
	cmd.Dir = fixture
	cmd.Env = append(os.Environ(), "COPILOT_BATCH_SIZE=15")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python prep: %v\n%s", err, out)
	}

	comparePlans(t, filepath.Join(goStaging, "batch-plan.json"), filepath.Join(pyStaging, "batch-plan.json"))
	compareManifests(t, filepath.Join(goStaging, "manifest.json"), filepath.Join(pyStaging, "manifest.json"))
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
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

	// Bare remote so origin/main exists for three-dot diff.
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

func comparePlans(t *testing.T, goPath, pyPath string) {
	t.Helper()
	goPlan := readJSON(t, goPath)
	pyPlan := readJSON(t, pyPath)

	goSkills := toStringSlice(goPlan["skills"])
	pySkills := toStringSlice(pyPlan["skills"])
	if !reflect.DeepEqual(goSkills, pySkills) {
		t.Fatalf("skills mismatch\ngo=%v\npy=%v", goSkills, pySkills)
	}
	goTotal, _ := asInt(goPlan["total_batches"])
	pyTotal, _ := asInt(pyPlan["total_batches"])
	if goTotal != pyTotal {
		t.Fatalf("total_batches go=%d py=%d", goTotal, pyTotal)
	}

	goBatches := toMapSlice(goPlan["batches"])
	pyBatches := toMapSlice(pyPlan["batches"])
	if len(goBatches) != len(pyBatches) {
		t.Fatalf("batch count go=%d py=%d", len(goBatches), len(pyBatches))
	}
	for i := range goBatches {
		gs, _ := goBatches[i]["skill"].(string)
		ps, _ := pyBatches[i]["skill"].(string)
		if gs != ps {
			t.Fatalf("batch[%d] skill go=%s py=%s", i, gs, ps)
		}
		gb, _ := goBatches[i]["batch_num"].(string)
		pb, _ := pyBatches[i]["batch_num"].(string)
		if gb != pb {
			t.Fatalf("batch[%d] batch_num go=%s py=%s", i, gb, pb)
		}
		gt, _ := asInt(goBatches[i]["task_count"])
		pt, _ := asInt(pyBatches[i]["task_count"])
		if gt != pt {
			t.Fatalf("batch[%d] task_count go=%d py=%d", i, gt, pt)
		}
	}
}

func compareManifests(t *testing.T, goPath, pyPath string) {
	t.Helper()
	goM := readJSON(t, goPath)
	pyM := readJSON(t, pyPath)

	goFiles := reviewableFiles(goM)
	pyFiles := reviewableFiles(pyM)
	sort.Strings(goFiles)
	sort.Strings(pyFiles)
	if !reflect.DeepEqual(goFiles, pyFiles) {
		t.Fatalf("reviewable files mismatch\ngo=%v\npy=%v", goFiles, pyFiles)
	}

	goAgents := toStringMapLists(goM["review_agents"])
	pyAgents := toStringMapLists(pyM["review_agents"])
	for k := range goAgents {
		sort.Strings(goAgents[k])
	}
	for k := range pyAgents {
		sort.Strings(pyAgents[k])
	}
	if !reflect.DeepEqual(goAgents, pyAgents) {
		t.Fatalf("review_agents mismatch\ngo=%v\npy=%v", goAgents, pyAgents)
	}
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
