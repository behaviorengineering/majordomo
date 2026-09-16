package contextdigest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	stropdspy "github.com/behaviorengineering/strop/dspy"
)

type evaluatorReplayFixture struct {
	Source          string                 `json:"source"`
	Operation       string                 `json:"operation"`
	RepoID          string                 `json:"repo_id"`
	Task            string                 `json:"task"`
	Fields          map[string]interface{} `json:"fields"`
	RecordedOutputs map[string]interface{} `json:"recorded_outputs"`
}

func evaluatorReplayFixturePath(slug string) string {
	return filepath.Join("testdata", "evaluator_replay", "gitboard_"+slug+"_span.json")
}

func loadEvaluatorReplayFixture(t *testing.T, slug string) (evaluatorReplayFixture, bool) {
	t.Helper()
	path := evaluatorReplayFixturePath(slug)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return evaluatorReplayFixture{}, false
		}
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var doc evaluatorReplayFixture
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	if len(doc.RecordedOutputs) == 0 {
		t.Fatalf("fixture %s recorded_outputs empty", path)
	}
	return doc, true
}

func liveEvaluatorReplayEnabled() bool {
	return os.Getenv("MAJORDOMO_LIVE_EVALUATOR_REPLAY") == "1" ||
		os.Getenv("MAJORDOMO_LIVE_GENERATOR_REPLAY") == "1"
}

// TestEvaluatorReplayFixturesOffline locks recorded typology_quality span outs.
func TestEvaluatorReplayFixturesOffline(t *testing.T) {
	slugs := []string{
		"typology_quality_feedback",
		"typology_quality_score",
		"typology_quality_consolidator",
	}
	found := 0
	for _, slug := range slugs {
		slug := slug
		t.Run(slug, func(t *testing.T) {
			doc, ok := loadEvaluatorReplayFixture(t, slug)
			if !ok {
				t.Skip("no fixture yet → " + evaluatorReplayFixturePath(slug))
			}
			found++
			switch slug {
			case "typology_quality_feedback":
				if stringField(doc.RecordedOutputs, "feedback") == "" {
					t.Fatal("recorded feedback empty")
				}
			case "typology_quality_score":
				if doc.RecordedOutputs["criterion_scores"] == nil {
					t.Fatal("recorded criterion_scores empty")
				}
			case "typology_quality_consolidator":
				if stringField(doc.RecordedOutputs, "consolidated_feedback") == "" {
					t.Fatal("recorded consolidated_feedback empty")
				}
			}
			t.Logf("task=%s outs=%v", doc.Task, keysOf(doc.RecordedOutputs))
		})
	}
	if found == 0 {
		t.Fatal("expected evaluator_replay fixtures")
	}
}

// TestLiveTypologyRefineEvaluateReplay runs the refine EvaluateWorkflow (quality
// feedback + score + consolidator) against the captured refine Generate I/O.
//
//	MAJORDOMO_LIVE_EVALUATOR_REPLAY=1 go test ./internal/contextdigest/ \
//	  -run LiveTypologyRefineEvaluateReplay -count=1 -v -timeout 15m
func TestLiveTypologyRefineEvaluateReplay(t *testing.T) {
	if !liveEvaluatorReplayEnabled() {
		t.Skip("set MAJORDOMO_LIVE_EVALUATOR_REPLAY=1 (or MAJORDOMO_LIVE_GENERATOR_REPLAY=1) for live evaluator replay")
	}
	if !polypusReachable(t) {
		t.Fatal("Polypus not reachable at http://127.0.0.1:1320/v1/models")
	}

	refine, ok := loadGeneratorReplayFixture(t, jmodules.TaskTypologyRefine)
	if !ok {
		t.Fatal("missing refine generator fixture")
	}
	if len(refine.RecordedOutputs) == 0 {
		t.Fatal("refine recorded_outputs empty")
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

	judge.ResetRegistryForTests()
	workStory := t.TempDir()
	traceDir := filepath.Join(workStory, "module-traces")
	ctx, closeTrace, err := stropdspy.AttachModuleTrace(context.Background(), traceDir, map[string]any{
		"pipeline": "evaluator-replay-test",
		"repo_id":  refine.RepoID,
		"task":     jmodules.TaskTypologyRefine,
	})
	if err != nil {
		t.Fatalf("attach module trace: %v", err)
	}
	defer func() { _ = closeTrace() }()

	rt, err := judge.NewRuntime(ctx, cfg, judge.RuntimeOptions{
		Tasks: []string{jmodules.TaskTypologyRefine},
	})
	if err != nil {
		t.Fatalf("judge runtime: %v", err)
	}

	agg, err := rt.Evaluate(ctx, jmodules.TaskTypologyRefine, refine.Fields, refine.RecordedOutputs, 1)
	if err != nil {
		t.Fatalf("evaluate typology_refine: %v", err)
	}
	if agg == nil {
		t.Fatal("nil aggregated evaluation")
	}
	t.Logf("aggregated=%+v", agg)

	entries, err := os.ReadDir(traceDir)
	if err != nil {
		t.Fatalf("read module-traces: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected module-traces JSONL from evaluate")
	}
	t.Logf("module_trace=%s", filepath.Join(traceDir, entries[0].Name()))
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
