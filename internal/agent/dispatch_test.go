package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseScore(t *testing.T) {
	n, ok := ParseScore("blah\nSCORE: 17\nmore")
	if !ok || n != 17 {
		t.Fatalf("got %d %v", n, ok)
	}
	_, ok = ParseScore("no score here")
	if ok {
		t.Fatal("expected false")
	}
}

func TestDispatchRequiresArgs(t *testing.T) {
	err := Dispatch(DispatchOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveScriptsDirExplicit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent-dispatch.sh"), []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveScriptsDir(dir)
	if err != nil || got != dir {
		t.Fatalf("got %q err=%v", got, err)
	}
}
