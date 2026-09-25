package orchestrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// BatchEntry mirrors staging/batch-plan.json batch rows.
type BatchEntry struct {
	Skill      string `json:"skill"`
	BatchNum   string `json:"batch_num"`
	TaskCount  int    `json:"task_count"`
	StagingDir string `json:"staging_dir"`
}

// BatchPlan is the prep output consumed by orchestrate.
type BatchPlan struct {
	Batches      []BatchEntry `json:"batches"`
	Skills       []string     `json:"skills"`
	TotalBatches int          `json:"total_batches"`
}

var synthesisSkills = map[string]struct{}{
	"pr-review-summary":      {},
	"pr-review-blast-radius": {},
	"pr-review-technical":    {},
}

// IsSynthesisSkill reports whether skill runs in Phase 2.
func IsSynthesisSkill(skill string) bool {
	_, ok := synthesisSkills[skill]
	return ok
}

// LoadBatchPlan reads batch-plan.json.
func LoadBatchPlan(path string) (*BatchPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan BatchPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("invalid batch-plan.json: %w", err)
	}
	return &plan, nil
}

// SplitBatches partitions file-review vs synthesis batches (prep order preserved).
func SplitBatches(batches []BatchEntry) (fileBatches, synthesisBatches []BatchEntry) {
	for _, b := range batches {
		if IsSynthesisSkill(b.Skill) {
			synthesisBatches = append(synthesisBatches, b)
		} else {
			fileBatches = append(fileBatches, b)
		}
	}
	return fileBatches, synthesisBatches
}

// ChunkBatches splits into waves of at most concurrency.
func ChunkBatches(batches []BatchEntry, concurrency int) [][]BatchEntry {
	if concurrency < 1 {
		concurrency = 1
	}
	var waves [][]BatchEntry
	for i := 0; i < len(batches); i += concurrency {
		end := i + concurrency
		if end > len(batches) {
			end = len(batches)
		}
		waves = append(waves, batches[i:end])
	}
	return waves
}

// CheckpointPath returns the done-file path for a batch.
func CheckpointPath(skillOutputDir, batchNum string) string {
	return filepath.Join(skillOutputDir, "logs", "batch_"+batchNum+".done.txt")
}

// FileExists is a small helper.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TouchCheckpoint creates an empty checkpoint file.
func TouchCheckpoint(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
