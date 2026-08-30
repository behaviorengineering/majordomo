package agenting

import (
	"fmt"
	"os"
	"path/filepath"
)

const stagingDirName = ".grounding"

// StagedPack is one grounding file copied beside a batch manifest.
type StagedPack struct {
	ID       string `json:"id"`
	Filename string `json:"file"`
}

// Stage copies selected GROUNDING.md files into batchDir/.grounding/.
func Stage(contextDir, batchDir string, packIDs []string) ([]StagedPack, error) {
	if len(packIDs) == 0 {
		return nil, nil
	}
	outDir := filepath.Join(batchDir, stagingDirName)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	var staged []StagedPack
	for _, id := range packIDs {
		src := filepath.Join(contextDir, "agenting", id, GroundingName)
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("read grounding pack %q: %w", id, err)
		}
		name := id + ".md"
		dst := filepath.Join(outDir, name)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, fmt.Errorf("stage grounding pack %q: %w", id, err)
		}
		staged = append(staged, StagedPack{ID: id, Filename: filepath.Join(stagingDirName, name)})
	}
	return staged, nil
}
