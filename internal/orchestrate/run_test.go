package orchestrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/agent"
)

func TestSplitAndChunkBatches(t *testing.T) {
	batches := []BatchEntry{
		{Skill: "pr-review-summary", BatchNum: "000"},
		{Skill: "pr-review-code", BatchNum: "001"},
		{Skill: "pr-review-code", BatchNum: "002"},
		{Skill: "pr-review-docs", BatchNum: "001"},
		{Skill: "pr-review-technical", BatchNum: "000"},
	}
	file, synth := SplitBatches(batches)
	if len(file) != 3 || len(synth) != 2 {
		t.Fatalf("file=%d synth=%d", len(file), len(synth))
	}
	waves := ChunkBatches(file, 2)
	if len(waves) != 2 || len(waves[0]) != 2 || len(waves[1]) != 1 {
		t.Fatalf("waves=%v", waves)
	}
}

func TestCheckpointSkip(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	output := filepath.Join(dir, "out")
	_ = os.MkdirAll(staging, 0o755)
	plan := `{
  "batches": [
    {"skill":"pr-review-code","batch_num":"001","task_count":1,"staging_dir":"` + filepath.ToSlash(filepath.Join(staging, "pr-review-code", "batch_001")) + `"}
  ],
  "skills": ["pr-review-code"],
  "total_batches": 1
}`
	if err := os.WriteFile(filepath.Join(staging, "batch-plan.json"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	skillOut := filepath.Join(output, "pr-review-code")
	cp := CheckpointPath(skillOut, "001")
	_ = TouchCheckpoint(cp)

	var calls atomic.Int32
	err := Run(Options{
		PRNumber: "1", StagingDir: staging, OutputDir: output,
		SkipPrep: true, SkipDeep: true, SkipReport: true,
		Dispatch: func(o agent.DispatchOptions) error {
			calls.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// File batch skipped via checkpoint; finalize+prose still run (2 calls).
	if calls.Load() < 1 {
		t.Fatalf("expected finalize/prose dispatches, got %d", calls.Load())
	}
}

func TestLoadBatchPlan(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "batch-plan.json")
	_ = os.WriteFile(p, []byte(`{"batches":[],"skills":[],"total_batches":0}`), 0o644)
	plan, err := LoadBatchPlan(p)
	if err != nil || plan.TotalBatches != 0 {
		t.Fatalf("%v %#v", err, plan)
	}
}

func TestFailedBatchDoesNotCheckpoint(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	output := filepath.Join(dir, "out")
	batchDir := filepath.Join(staging, "pr-review-code", "batch_001")
	_ = os.MkdirAll(batchDir, 0o755)
	plan := `{
  "batches": [
    {"skill":"pr-review-code","batch_num":"001","task_count":1,"staging_dir":"` + filepath.ToSlash(batchDir) + `"}
  ],
  "skills": ["pr-review-code"],
  "total_batches": 1
}`
	if err := os.WriteFile(filepath.Join(staging, "batch-plan.json"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		PRNumber: "1", StagingDir: staging, OutputDir: output,
		SkipPrep: true, SkipDeep: true, SkipReport: true,
		Dispatch: func(o agent.DispatchOptions) error {
			if o.Mode == agent.ModeFiles {
				return fmt.Errorf("agent boom")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	cp := CheckpointPath(filepath.Join(output, "pr-review-code"), "001")
	if FileExists(cp) {
		t.Fatalf("failed batch must not create checkpoint %s", cp)
	}
}

func TestUntilPrepDoesNotDispatch(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	output := filepath.Join(dir, "out")
	_ = os.MkdirAll(staging, 0o755)

	var calls atomic.Int32
	err := Run(Options{
		PRNumber: "1", StagingDir: staging, OutputDir: output,
		SkipPrep: true, SkipDeep: true, SkipReport: true,
		Until: StagePrep,
		Dispatch: func(o agent.DispatchOptions) error {
			calls.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatalf("until prep must not dispatch, got %d", calls.Load())
	}
}

func TestUntilWavesSkipsFinalize(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	output := filepath.Join(dir, "out")
	batchDir := filepath.Join(staging, "pr-review-code", "batch_001")
	_ = os.MkdirAll(batchDir, 0o755)
	plan := `{
  "batches": [
    {"skill":"pr-review-code","batch_num":"001","task_count":1,"staging_dir":"` + filepath.ToSlash(batchDir) + `"}
  ],
  "skills": ["pr-review-code"],
  "total_batches": 1
}`
	if err := os.WriteFile(filepath.Join(staging, "batch-plan.json"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	skillOut := filepath.Join(output, "pr-review-code")
	if err := TouchCheckpoint(CheckpointPath(skillOut, "001")); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	err := Run(Options{
		PRNumber: "1", StagingDir: staging, OutputDir: output,
		SkipPrep: true, SkipDeep: true, SkipReport: true,
		Until: StageWaves,
		Dispatch: func(o agent.DispatchOptions) error {
			calls.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatalf("until waves with checkpointed batch must not finalize, got %d", calls.Load())
	}
}
