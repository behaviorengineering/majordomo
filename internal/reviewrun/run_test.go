package reviewrun

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/orchestrate"
	"github.com/behaviorengineering/majordomo/internal/publish"
	"github.com/behaviorengineering/majordomo/internal/sa"
)

func writeConfig(t *testing.T, dir string) string {
	t.Helper()
	cfg := filepath.Join(dir, "cfg")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "_defaults.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
  cloneUrl: https://github.com/acme/demo.git
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestParseUntil(t *testing.T) {
	got, err := ParseUntil(" Prep ")
	if err != nil || got != StagePrep {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := ParseUntil("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUntilPrepDoesNotPublish(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir)
	var orchUntil string
	err := Run(Options{
		ConfigDir:  cfg,
		RepoID:     "demo",
		PRNumber:   "12",
		WorkDir:    dir,
		StagingDir: filepath.Join(dir, "st"),
		OutputDir:  filepath.Join(dir, "out"),
		Until:      StagePrep,
		Publish:    true,
		Clone:      func() error { return nil },
		SA:         func(sa.Options) error { return nil },
		Orchestrate: func(o orchestrate.Options) error {
			orchUntil = o.Until
			return nil
		},
		PublishFn: func(publish.Options) error {
			t.Fatal("publish must not run when until=prep")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if orchUntil != orchestrate.StagePrep {
		t.Fatalf("orchestrate until=%q want prep", orchUntil)
	}
}

func TestNoPublishSkipsPublish(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir)
	err := Run(Options{
		ConfigDir:  cfg,
		RepoID:     "demo",
		PRNumber:   "12",
		WorkDir:    dir,
		StagingDir: filepath.Join(dir, "st"),
		OutputDir:  filepath.Join(dir, "out"),
		Publish:    false,
		Clone:      func() error { return nil },
		SA:         func(sa.Options) error { return nil },
		Orchestrate: func(o orchestrate.Options) error {
			return os.WriteFile(filepath.Join(o.OutputDir, "summary.md"), []byte("# s\n\nbody\n"), 0o644)
		},
		PublishFn: func(publish.Options) error {
			t.Fatal("publish must not run without --publish")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublishTrueCallsPublish(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir)
	var published string
	err := Run(Options{
		ConfigDir:  cfg,
		RepoID:     "demo",
		PRNumber:   "12",
		WorkDir:    dir,
		StagingDir: filepath.Join(dir, "st"),
		OutputDir:  filepath.Join(dir, "out"),
		Publish:    true,
		Clone:      func() error { return nil },
		SA:         func(sa.Options) error { return nil },
		Orchestrate: func(o orchestrate.Options) error {
			p := filepath.Join(o.OutputDir, "summary.md")
			return os.WriteFile(p, []byte("# s\n\nbody\n"), 0o644)
		},
		PublishFn: func(o publish.Options) error {
			published = o.SummaryFile
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if published == "" {
		t.Fatal("expected publish")
	}
}

func TestUntilCloneSkipsOrchestrate(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir)
	err := Run(Options{
		ConfigDir:  cfg,
		RepoID:     "demo",
		PRNumber:   "12",
		WorkDir:    dir,
		StagingDir: filepath.Join(dir, "st"),
		OutputDir:  filepath.Join(dir, "out"),
		Until:      StageClone,
		Clone:      func() error { return nil },
		SA: func(sa.Options) error {
			t.Fatal("sa must not run when until=clone")
			return nil
		},
		Orchestrate: func(o orchestrate.Options) error {
			t.Fatal("orchestrate must not run when until=clone")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCloneSkippedWhenWorkdirAtHead(t *testing.T) {
	repo, sha := initRepo(t)
	cfg := writeConfig(t, t.TempDir())
	err := Run(Options{
		ConfigDir:  cfg,
		RepoID:     "demo",
		PRNumber:   "12",
		HeadSHA:    sha,
		BaseBranch: "main",
		WorkDir:    repo,
		StagingDir: filepath.Join(t.TempDir(), "st"),
		OutputDir:  filepath.Join(t.TempDir(), "out"),
		Until:      StageClone,
		Orchestrate: func(o orchestrate.Options) error {
			t.Fatal("orchestrate must not run")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func initRepo(t *testing.T) (dir, sha string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@t.test")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return dir, strings.TrimSpace(string(out))
}
