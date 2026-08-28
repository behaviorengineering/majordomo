package contextdigest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func TestDetectRewrite(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main", "--template=")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "c0")
	old, _ := (&Git{Dir: dir}).trim("rev-parse", "HEAD")
	runGit(t, dir, "checkout", "--orphan", "rewritten")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "c1")
	newHead, _ := (&Git{Dir: dir}).trim("rev-parse", "HEAD")
	g := &Git{Dir: dir}
	ok, err := DetectRewrite(g, old, newHead, "rewritten")
	if err != nil || !ok {
		t.Fatalf("DetectRewrite = %v err=%v", ok, err)
	}
}

func TestBeginRewriteBlockedWhy(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := contextstore.Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	if _, err := BeginRewrite(dir, "newhead", at, "alice", "evidence"); err != nil {
		t.Fatal(err)
	}
	meta, err := contextstore.ParseMeta(filepath.Join(dir, "meta.yaml"))
	if err != nil || !meta.RewritePending {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
	err = CompleteRewrite(dir, "newhead", at)
	if err == nil {
		t.Fatal("expected blocked without why")
	}
	if err := ApplyRewriteWhy(dir, "rebased main"); err != nil {
		t.Fatal(err)
	}
	if err := CompleteRewrite(dir, "newhead", at); err != nil {
		t.Fatal(err)
	}
}

func TestCompactChronology(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := contextstore.Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		ev := contextstore.ChronologyEvent{
			Date: at.AddDate(0, 0, -i), Actor: "t", Source: "s",
			Did: "d", Because: "b", InOrderTo: "o", Evidence: "e",
		}
		if err := contextstore.AppendChronologyEvent(dir, ev); err != nil {
			t.Fatal(err)
		}
	}
	changed, err := CompactChronology(dir, CompactOptions{MaxEntries: 3, KeepRecent: 2, ForceCompact: true})
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
}
