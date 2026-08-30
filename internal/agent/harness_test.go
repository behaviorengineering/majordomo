package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
)

func TestRunOpenCodeRequiresArgs(t *testing.T) {
	err := RunOpenCode(DispatchOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunOpenCodeUsesChildEnv(t *testing.T) {
	aigateway.ResetForTests()
	t.Cleanup(aigateway.ResetForTests)
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("OPENAI_API_KEY", "sk-leak")
	t.Setenv("GEMINI_API_KEY", "")

	dir := t.TempDir()
	script := filepath.Join(dir, "agent-dispatch.sh")
	if err := os.WriteFile(script, []byte("#!/bin/bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var gotEnv []string
	var gotArgs []string
	err := RunOpenCode(DispatchOptions{
		PRNumber:   "42",
		StagingDir: dir,
		OutputDir:  dir,
		ScriptsDir: dir,
		Mode:       ModeSummary,
		Runner: func(name string, args []string, env []string, _ string) error {
			if name != script {
				t.Fatalf("script=%s", name)
			}
			gotArgs = append([]string{}, args...)
			gotEnv = append([]string{}, env...)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(gotArgs) != 4 || gotArgs[0] != "42" || gotArgs[3] != string(ModeSummary) {
		t.Fatalf("args=%v", gotArgs)
	}
	for _, e := range gotEnv {
		if e == "OPENAI_API_KEY=sk-leak" || e == "ANTHROPIC_API_KEY=sk-test" {
			t.Fatalf("real key leaked: %s", e)
		}
	}
	foundDummy := false
	foundBase := false
	for _, e := range gotEnv {
		if e == "OPENAI_API_KEY="+aigateway.DummyAPIKey {
			foundDummy = true
		}
		if strings.HasPrefix(e, "OPENAI_BASE_URL=http://127.0.0.1:") {
			foundBase = true
		}
	}
	if !foundDummy || !foundBase {
		t.Fatalf("missing gateway env dummy=%v base=%v", foundDummy, foundBase)
	}
}
