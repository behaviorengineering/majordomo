package agent

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestRunSummaryLoopAcceptsThreshold(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "batch")
	skillOut := filepath.Join(dir, "pipeline", "pr-review-summary")
	pipelineOut := filepath.Dir(skillOut)
	_ = os.MkdirAll(staging, 0o755)
	_ = os.MkdirAll(skillOut, 0o755)

	var n atomic.Int32
	err := RunSummaryLoop(SummaryLoopOptions{
		PRNumber: "9", StagingDir: staging, OutputDir: skillOut,
		PassScore: 10, MaxIter: 3,
		Dispatch: func(o DispatchOptions) error {
			n.Add(1)
			_ = os.WriteFile(filepath.Join(pipelineOut, "summary.md"), []byte("# S\n"), 0o644)
			_ = os.WriteFile(filepath.Join(pipelineOut, "score.md"), []byte("SCORE: 12\n"), 0o644)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n.Load() != 2 { // summary + score once
		t.Fatalf("dispatch calls=%d", n.Load())
	}
	if os.Getenv("SUMMARY_ITER") != "" {
		t.Fatalf("SUMMARY_ITER leaked: %q", os.Getenv("SUMMARY_ITER"))
	}
}
