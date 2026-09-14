package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	stropdspy "github.com/behaviorengineering/strop/dspy"
)

// digestGeneratorReplayTasks are CoT/Predict digest generators that should gain
// env-gated live replay fixtures (golang-quality C17 / dspy-pipeline-isolation).
func digestGeneratorReplayTasks() []string {
	return judge.DigestTasks()
}

// TestGeneratorReplayFixturesOffline loads every present generator_replay span
// and checks recorded_outputs are non-empty maps (no LLM).
func TestGeneratorReplayFixturesOffline(t *testing.T) {
	found := 0
	for _, task := range digestGeneratorReplayTasks() {
		task := task
		t.Run(task, func(t *testing.T) {
			doc, ok := loadGeneratorReplayFixture(t, task)
			if !ok {
				t.Skip("no fixture yet; capture Predict:" + task + " from module-traces into " + generatorReplayFixturePath(task))
			}
			found++
			if len(doc.RecordedOutputs) == 0 {
				t.Fatal("recorded_outputs empty")
			}
			switch task {
			case jmodules.TaskTypologyCluster:
				rows, err := mergesFromClusterOut(doc.RecordedOutputs)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("recorded merge rows=%d", len(rows))
			case jmodules.TaskTypologyRefine:
				if stringField(doc.RecordedOutputs, "refined_catalog_yaml") == "" {
					t.Fatal("recorded refined_catalog_yaml empty")
				}
			}
		})
	}
	if found == 0 {
		t.Fatal("expected at least one generator_replay fixture under testdata/generator_replay/")
	}
}

// TestLiveDigestGeneratorReplay re-runs each digest CoT generator that has a
// fixture against Polypus. Opt-in only (CI stays offline):
//
//	MAJORDOMO_LIVE_GENERATOR_REPLAY=1 go test ./internal/contextdigest/ \
//	  -run LiveDigestGeneratorReplay -count=1 -v -timeout 15m
//
// MAJORDOMO_LIVE_CLUSTER_REPLAY=1 also enables this suite (compat with the
// earlier cluster-only env). Missing fixtures skip their subtest.
//
// Optional: MAJORDOMO_CENTRAL_CONFIG=/path/to/majordomo-central-config
func TestLiveDigestGeneratorReplay(t *testing.T) {
	if !liveGeneratorReplayEnabled() {
		t.Skip("set MAJORDOMO_LIVE_GENERATOR_REPLAY=1 (or MAJORDOMO_LIVE_CLUSTER_REPLAY=1) for live Polypus generator replay")
	}
	if !polypusReachable(t) {
		t.Fatal("Polypus not reachable at http://127.0.0.1:1320/v1/models")
	}

	configDir := generatorReplayConfigDir(t)
	defaults, err := config.LoadDefaults(configDir)
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	cfg, err := config.LoadRepoFile(configDir, "gitboard", defaults)
	if err != nil {
		t.Fatalf("load gitboard config: %v", err)
	}

	for _, task := range digestGeneratorReplayTasks() {
		task := task
		t.Run(task, func(t *testing.T) {
			doc, ok := loadGeneratorReplayFixture(t, task)
			if !ok {
				t.Skip("no fixture; capture from module-traces → " + generatorReplayFixturePath(task))
			}

			judge.ResetRegistryForTests()
			workStory := t.TempDir()
			traceDir := filepath.Join(workStory, "module-traces")
			ctx, closeTrace, err := stropdspy.AttachModuleTrace(context.Background(), traceDir, map[string]any{
				"pipeline": "generator-replay-test",
				"repo_id":  doc.RepoID,
				"task":     task,
				"source":   doc.Source,
			})
			if err != nil {
				t.Fatalf("attach module trace: %v", err)
			}
			defer func() { _ = closeTrace() }()

			rt, err := judge.NewRuntime(ctx, cfg, judge.RuntimeOptions{
				Tasks: []string{task},
			})
			if err != nil {
				t.Fatalf("judge runtime: %v", err)
			}

			out, err := rt.Generate(ctx, task, doc.Fields, 1)
			if err != nil {
				t.Fatalf("%s generate: %v", task, err)
			}

			for k, v := range out {
				if s, ok := v.(string); ok {
					t.Logf("%s=%q", k, truncateReplayLog(s, 240))
				} else {
					t.Logf("%s=%T", k, v)
				}
			}

			switch task {
			case jmodules.TaskTypologyCluster:
				rows, err := mergesFromClusterOut(out)
				if err != nil {
					t.Fatalf("zip merges: %v", err)
				}
				t.Logf("proposed_merge_rows=%d", len(rows))
			case jmodules.TaskTypologyRefine:
				if stringField(out, "refined_catalog_yaml") == "" {
					t.Fatal("live refined_catalog_yaml empty")
				}
			}

			entries, err := os.ReadDir(traceDir)
			if err != nil {
				t.Fatalf("read module-traces: %v", err)
			}
			if len(entries) == 0 {
				t.Fatal("expected module-traces JSONL")
			}
			t.Logf("module_trace=%s", filepath.Join(traceDir, entries[0].Name()))
		})
	}
}
