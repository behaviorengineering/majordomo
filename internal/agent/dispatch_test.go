package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/judge"
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

func TestDispatchRefusesStropMode(t *testing.T) {
	t.Setenv(judge.EnvJudge, "strop")
	err := Dispatch(DispatchOptions{
		PRNumber: "1", StagingDir: t.TempDir(), OutputDir: t.TempDir(),
		Runner: func(string, []string, []string, string) error {
			t.Fatal("runner must not run when strop selected")
			return nil
		},
	})
	if !errors.Is(err, judge.ErrStropJudgeNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestDispatchSetsGroundingEnv(t *testing.T) {
	t.Setenv(judge.EnvJudge, "opencode")
	scriptsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scriptsDir, "agent-dispatch.sh"), []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	batchDir := t.TempDir()
	outDir := t.TempDir()
	groundDir := filepath.Join(batchDir, ".grounding")
	if err := os.MkdirAll(groundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groundDir, "overview.md"), []byte("# o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{"review_agents":{"pr-review-code":["a.go"]},"grounding_packs":[{"id":"overview","file":".grounding/overview.md"}]}`
	if err := os.WriteFile(filepath.Join(batchDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotEnv []string
	err := Dispatch(DispatchOptions{
		PRNumber:   "1",
		StagingDir: batchDir,
		OutputDir:  outDir,
		ScriptsDir: scriptsDir,
		Runner: func(_ string, _ []string, env []string, _ string) error {
			gotEnv = env
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var grounding string
	for _, e := range gotEnv {
		if strings.HasPrefix(e, "MAJORDOMO_GROUNDING=") {
			grounding = strings.TrimPrefix(e, "MAJORDOMO_GROUNDING=")
		}
	}
	if grounding == "" || !strings.Contains(grounding, "overview.md") {
		t.Fatalf("MAJORDOMO_GROUNDING missing from env: %v", gotEnv)
	}
}
