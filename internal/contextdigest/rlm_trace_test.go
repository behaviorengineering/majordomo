package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

func TestResolveWorkStoryDirDefaultUnderTmp(t *testing.T) {
	t.Setenv("MAJORDOMO_DIGEST_WORK_STORY_DIR", "")
	now := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	got, err := resolveWorkStoryDir(Options{RepoID: "gitboard"}, now)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("tmp", "digest-runs", "gitboard-20260914-030000Z")
	if got != wantSuffix {
		t.Fatalf("got %q want %q", got, wantSuffix)
	}
}

func TestResolveWorkStoryDirEnvParent(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("MAJORDOMO_DIGEST_WORK_STORY_DIR", parent)
	now := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	got, err := resolveWorkStoryDir(Options{RepoID: "gitboard"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, parent) {
		t.Fatalf("got %q want under %q", got, parent)
	}
	if filepath.Base(got) != "gitboard-20260914-030000Z" {
		t.Fatalf("leaf=%q", filepath.Base(got))
	}
}

func TestResolveWorkStoryDirExplicitWins(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "custom-run")
	t.Setenv("MAJORDOMO_DIGEST_WORK_STORY_DIR", t.TempDir())
	got, err := resolveWorkStoryDir(Options{RepoID: "gitboard", WorkStoryDir: explicit}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(explicit) {
		t.Fatalf("got %q want %q", got, explicit)
	}
}

func TestEnsureWorkStoryDirWritable(t *testing.T) {
	t.Setenv("MAJORDOMO_DIGEST_WORK_STORY_DIR", t.TempDir())
	dir, err := ensureWorkStoryDir(Options{RepoID: "demo"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	trace := rlmTraceDir(dir, jmodules.TaskTypologySliceGroupingAudit)
	marker := filepath.Join(trace, "step.jsonl")
	if err := os.WriteFile(marker, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRLMTraceDirCreatesScratch(t *testing.T) {
	base := t.TempDir()
	dir := rlmTraceDir(base, jmodules.TaskTypologySliceGroupingAudit)
	marker := filepath.Join(dir, "step.jsonl")
	if err := os.WriteFile(marker, []byte(`{"step":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dir, "rlm-traces") {
		t.Fatalf("dir=%q", dir)
	}
}

func TestModuleTraceDirSiblingOfRLM(t *testing.T) {
	base := t.TempDir()
	dir := moduleTraceDir(base)
	if filepath.Base(dir) != "module-traces" {
		t.Fatalf("dir=%q", dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestAttachModuleTracePutsSessionOnContext(t *testing.T) {
	opts := Options{RepoID: "gitboard", WorkStoryDir: t.TempDir()}
	closeFn, err := attachModuleTrace(&opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeFn() }()
	if opts.Context == nil {
		t.Fatal("expected Context")
	}
	entries, err := os.ReadDir(filepath.Join(opts.WorkStoryDir, "module-traces"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries=%d", len(entries))
	}
}
