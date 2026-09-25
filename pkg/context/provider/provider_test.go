package contextprovider

import (
	"context"
	"errors"
	"testing"
	"time"

	contextstore "github.com/behaviorengineering/majordomo/pkg/context/store"
)

func TestLoadSnapshotFromDirExplicitMissingFailsClosed(t *testing.T) {
	_, err := LoadSnapshotFromDir(t.TempDir()+"/missing", "demo", true)
	if !errors.Is(err, ErrInvalidContextTree) {
		t.Fatalf("err = %v, want ErrInvalidContextTree", err)
	}
}

func TestLoadSnapshotFromDirOptionalMissingReturnsNoContext(t *testing.T) {
	_, err := LoadSnapshotFromDir("", "demo", false)
	if !errors.Is(err, ErrNoContext) {
		t.Fatalf("err = %v, want ErrNoContext", err)
	}
}

func TestChainExplicitValidTree(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := contextstore.Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	chain := NewChain(ChainConfig{ExplicitDir: dir})
	snap, err := chain.Resolve(context.Background(), Request{RepoID: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if snap == nil || snap.RootDir != dir {
		t.Fatalf("snap = %+v", snap)
	}
	if snap.Provenance.SourceCommit != "abc" {
		t.Fatalf("provenance = %+v", snap.Provenance)
	}
}

func TestChainNoExplicitNoRemoteReturnsNoContext(t *testing.T) {
	chain := NewChain(ChainConfig{})
	_, err := chain.Resolve(context.Background(), Request{RepoID: "demo"})
	if !errors.Is(err, ErrNoContext) {
		t.Fatalf("err = %v", err)
	}
}

func TestStaticProvider(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := contextstore.Bootstrap(dir, "demo", "abc", at); err != nil {
		t.Fatal(err)
	}
	snap, err := LoadSnapshotFromDir(dir, "demo", true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Static(snap).Resolve(context.Background(), Request{RepoID: "demo"})
	if err != nil || got == nil {
		t.Fatalf("Resolve: snap=%v err=%v", got, err)
	}
}
