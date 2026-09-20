package contextdigest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	stropdspy "github.com/behaviorengineering/strop/pkg/dspy"
)

// clusterReplayFixture is one captured typology_cluster Process span used by the
// offline "recorded none" gate (older seed that emitted literal none).
type clusterReplayFixture struct {
	Source          string                 `json:"source"`
	Operation       string                 `json:"operation"`
	RepoID          string                 `json:"repo_id"`
	Fields          map[string]interface{} `json:"fields"`
	RecordedOutputs map[string]interface{} `json:"recorded_outputs"`
}

func loadClusterReplayFixture(t *testing.T) clusterReplayFixture {
	t.Helper()
	path := filepath.Join("testdata", "cluster_replay", "gitboard_typology_cluster_span.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var doc clusterReplayFixture
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if len(doc.Fields) == 0 {
		t.Fatal("fixture fields empty")
	}
	return doc
}

// TestClusterReplayFixtureZipsRecordedNone locks the offline gate: recorded seed
// outputs of literal "none" become an empty merge proposal (no audit pending).
func TestClusterReplayFixtureZipsRecordedNone(t *testing.T) {
	doc := loadClusterReplayFixture(t)
	rows, err := mergesFromClusterOut(doc.RecordedOutputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("recorded none → want 0 merges, got %+v", rows)
	}
}

// TestLiveTypologyClusterReplay is kept for the older env name. Prefer
// TestLiveDigestGeneratorReplay with MAJORDOMO_LIVE_GENERATOR_REPLAY=1.
func TestLiveTypologyClusterReplay(t *testing.T) {
	if os.Getenv("MAJORDOMO_LIVE_CLUSTER_REPLAY") != "1" && os.Getenv("MAJORDOMO_LIVE_GENERATOR_REPLAY") != "1" {
		t.Skip("set MAJORDOMO_LIVE_CLUSTER_REPLAY=1 (or MAJORDOMO_LIVE_GENERATOR_REPLAY=1) to run live Polypus cluster replay")
	}
	if !polypusReachable(t) {
		t.Fatal("Polypus not reachable at http://127.0.0.1:1320/v1/models")
	}

	doc := loadClusterReplayFixture(t)
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
		"pipeline": "cluster-replay-test",
		"repo_id":  doc.RepoID,
		"source":   doc.Source,
	})
	if err != nil {
		t.Fatalf("attach module trace: %v", err)
	}
	defer func() { _ = closeTrace() }()

	rt, err := judge.NewRuntime(ctx, cfg, judge.RuntimeOptions{
		Tasks: []string{jmodules.TaskTypologySliceGrouping},
	})
	if err != nil {
		t.Fatalf("judge runtime: %v", err)
	}

	out, err := rt.Generate(ctx, jmodules.TaskTypologySliceGrouping, doc.Fields, 1)
	if err != nil {
		t.Fatalf("typology_cluster generate: %v", err)
	}

	mergeIDs := stringField(out, "merge_ids")
	t.Logf("merge_ids=%q", mergeIDs)
	t.Logf("merge_packages=%q", stringField(out, "merge_packages"))
	t.Logf("merge_intents=%q", stringField(out, "merge_intents"))
	if ack := stringField(out, "directives_ack"); ack != "" {
		t.Logf("directives_ack head=%q", truncateReplayLog(ack, 240))
	}

	rows, err := mergesFromClusterOut(out)
	if err != nil {
		t.Fatalf("zip merges: %v", err)
	}
	t.Logf("proposed_merge_rows=%d", len(rows))
	for _, row := range rows {
		t.Logf("  merge id=%s intent=%s packages=%v", row.ID, row.Intent, row.Packages)
		if strings.EqualFold(strings.TrimSpace(row.ID), "analysis") {
			t.Fatalf("cluster replay must not invent merge id %q", row.ID)
		}
	}
	if strings.Contains(strings.ToLower(mergeIDs), "analysis") {
		t.Fatalf("merge_ids must not contain analysis: %q", mergeIDs)
	}

	entries, err := os.ReadDir(traceDir)
	if err != nil {
		t.Fatalf("read module-traces: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected module-traces JSONL")
	}
	t.Logf("module_trace=%s", filepath.Join(traceDir, entries[0].Name()))
}
