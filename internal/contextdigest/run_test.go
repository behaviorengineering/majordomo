package contextdigest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func TestFirstParentCommits(t *testing.T) {
	dir := initLinearRepo(t)
	g := &Git{Dir: dir}
	shas := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		sha, err := g.trim("rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		shas = append(shas, sha)
		if i == 3 {
			break
		}
		runGit(t, dir, "commit", "--allow-empty", "-m", "next")
	}
	got, err := FirstParentCommits(g, shas[0], shas[3], "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d commits, want 3: %v", len(got), got)
	}
}

func TestIsBehind(t *testing.T) {
	dir := initLinearRepo(t)
	g := &Git{Dir: dir}
	old, _ := g.trim("rev-parse", "HEAD~1")
	head, _ := g.trim("rev-parse", "HEAD")
	ok, err := IsBehind(g, old, head, "main")
	if err != nil || !ok {
		t.Fatalf("behind=%v err=%v", ok, err)
	}
	ok, err = IsBehind(g, head, head, "main")
	if err != nil || ok {
		t.Fatalf("equal should not be behind: %v %v", ok, err)
	}
}

func TestGitLabAuthHeader(t *testing.T) {
	g := &Git{SCM: "gitlab", Token: "tok"}
	args := g.authConfigArgs()
	if len(args) != 2 || !strings.Contains(args[1], "PRIVATE-TOKEN: tok") {
		t.Fatalf("gitlab auth = %v", args)
	}
}

func TestSeedOrphanCommitIdentity(t *testing.T) {
	dir := t.TempDir()
	branch := config.ContextBranch("demo")
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := seedOrphan(dir, branch, "demo", "abc123", at, "", "github", "https://github.com/acme/demo.git"); err != nil {
		t.Fatal(err)
	}
	g := &Git{Dir: dir}
	email, err := g.trim("config", "user.email")
	if err != nil || email == "" {
		t.Fatalf("user.email = %q err=%v", email, err)
	}
}

func TestRunGenericSCMSkip(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("scm: generic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte("repository:\n  id: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(Options{ConfigDir: cfgDir, RepoID: "demo", WorkDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "skipped" {
		t.Fatalf("action=%q", res.Action)
	}
}

func TestRunNoopWhenCaughtUp(t *testing.T) {
	served := initServedRemote(t)
	ctxBase := config.ContextBranch("demo")
	seedContextRemote(t, served.remote, ctxBase, "demo", served.head)

	cfgDir, workDir := writeDigestConfig(t, served.remote, "demo")
	t.Setenv("GH_TOKEN_ACME", "tok")
	res, err := Run(Options{
		ConfigDir: cfgDir,
		RepoID:    "demo",
		WorkDir:   workDir,
		Now:       time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC),
		Forge: &Forge{
			SCM: "github", Owner: "acme", Name: "demo", Token: "tok",
			Runner: func(string, []string, []string) (string, error) { return "", nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "noop" {
		t.Fatalf("action=%q result=%s", res.Action, mustJSON(res))
	}
}

func TestRunNoopWhenUpdateBranchHasHEAD(t *testing.T) {
	served := initServedRemote(t)
	ctxBase := config.ContextBranch("demo")
	ctxUpdate := config.ContextUpdateBranch("demo")
	seedContextRemote(t, served.remote, ctxBase, "demo", "")

	// Update branch already at default HEAD while merged base still has empty cursor.
	updateDir := t.TempDir()
	g, err := InitOrphan(updateDir, ctxUpdate)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, updateDir, "config", "user.email", "t@example.com")
	runGit(t, updateDir, "config", "user.name", "t")
	if err := contextstore.Bootstrap(updateDir, "demo", served.head, time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	runGit(t, updateDir, "add", "-A")
	runGit(t, updateDir, "commit", "-m", "update tip")
	runGit(t, updateDir, "remote", "add", "origin", served.remote)
	runGit(t, updateDir, "push", "origin", "HEAD:"+ctxUpdate)
	_ = g

	cfgDir, workDir := writeDigestConfig(t, served.remote, "demo")
	t.Setenv("GH_TOKEN_ACME", "tok")
	res, err := Run(Options{
		ConfigDir: cfgDir,
		RepoID:    "demo",
		WorkDir:   workDir,
		Forge: &Forge{
			SCM: "github", Owner: "acme", Name: "demo", Token: "tok",
			Runner: func(string, []string, []string) (string, error) { return "", nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "noop" {
		t.Fatalf("action=%q", res.Action)
	}
}

func TestListTargetsSkipsExamples(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte("scm: github\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	example := `scm: github
repository:
  id: example-github
  owner: YOUR_ORG
  name: YOUR_REPO
  cloneUrl: https://github.com/YOUR_ORG/YOUR_REPO.git
`
	demo := `scm: github
repository:
  id: demo
  owner: acme
  name: demo
  cloneUrl: https://github.com/acme/demo.git
`
	if err := os.WriteFile(filepath.Join(dir, "example-github.yaml"), []byte(example), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte(demo), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListTargets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 || got.Repos[0].RepoID != "demo" {
		t.Fatalf("repos=%+v", got.Repos)
	}
}

func TestOpenUpdatePRUpdatesExistingGitHub(t *testing.T) {
	var edited bool
	f := &Forge{
		SCM: "github", Owner: "acme", Name: "demo", Token: "tok",
		Runner: func(name string, args []string, _ []string) (string, error) {
			if name == "gh" && len(args) > 0 && args[0] == "pr" && len(args) > 1 && args[1] == "list" {
				return "42", nil
			}
			if name == "gh" && len(args) > 0 && args[0] == "pr" && len(args) > 1 && args[1] == "edit" {
				edited = true
				return "", nil
			}
			return "", nil
		},
	}
	n, err := f.OpenUpdatePR("majordomo-context/demo", "majordomo-context/demo-update", "t", "b")
	if err != nil || n != "42" || !edited {
		t.Fatalf("n=%q edited=%v err=%v", n, edited, err)
	}
}

func initLinearRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main", "--template=")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "c0")
	for i := 0; i < 3; i++ {
		runGit(t, dir, "commit", "--allow-empty", "-m", "c")
	}
	return dir
}

type servedFixture struct {
	remote string
	head   string
}

func initServedRemote(t *testing.T) servedFixture {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main", "--template=")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	head, _ := (&Git{Dir: dir}).trim("rev-parse", "HEAD")
	remote := filepath.Join(t.TempDir(), "remote.git")
	cmd := exec.Command("git", "init", "--bare", remote)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_EMAIL=test@example.com", "GIT_AUTHOR_NAME=test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v\n%s", err, out)
	}
	runGit(t, dir, "remote", "add", "origin", remote)
	runGit(t, dir, "push", "-u", "origin", "main")
	runGit(t, dir, "fetch", "origin")
	return servedFixture{remote: remote, head: head}
}

func seedContextRemote(t *testing.T, servedRemote, baseBranch, repoID, cursor string) {
	t.Helper()
	dir := t.TempDir()
	g, err := InitOrphan(dir, baseBranch)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "t")
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := contextstore.Bootstrap(dir, repoID, cursor, at); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "bootstrap")
	runGit(t, dir, "remote", "add", "origin", servedRemote)
	runGit(t, dir, "push", "origin", "HEAD:"+baseBranch)
	_ = g
}

func writeDigestConfig(t *testing.T, remote, repoID string) (cfgDir, workDir string) {
	t.Helper()
	root := t.TempDir()
	cfgDir = filepath.Join(root, "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defaults := `context:
  repo: served
`
	repo := `scm: github
repository:
  id: demo
  owner: acme
  name: demo
  cloneUrl: ` + remote + `
`
	if err := os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte(defaults), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, repoID+".yaml"), []byte(repo), 0o644); err != nil {
		t.Fatal(err)
	}
	cloneParent := filepath.Join(root, "clone")
	if err := os.MkdirAll(cloneParent, 0o755); err != nil {
		t.Fatal(err)
	}
	workDir = filepath.Join(cloneParent, "demo")
	runGit(t, cloneParent, "clone", remote, workDir)
	return cfgDir, workDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
