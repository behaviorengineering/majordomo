package staging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/behaviorengineering/majordomo/internal/agenting"
)

// AttachGrounding selects agenting packs per batch manifest and stages GROUNDING.md copies.
func AttachGrounding(contextDir string, batches []BatchEntry) error {
	contextDir = filepath.Clean(contextDir)
	idx, err := agenting.LoadIndex(contextDir)
	if err != nil {
		return fmt.Errorf("agenting: %w", err)
	}
	for _, b := range batches {
		manifestPath := filepath.Join(b.StagingDir, "manifest.json")
		files, err := manifestChangedFiles(manifestPath)
		if err != nil {
			return err
		}
		mode := agenting.ModeForSkill(b.Skill)
		packIDs := agenting.Select(idx, mode, files)
		if len(packIDs) == 0 {
			logf("INFO", "Grounding: %s/%s — no packs for mode %s", b.Skill, b.BatchNum, mode)
			continue
		}
		staged, err := agenting.Stage(contextDir, b.StagingDir, packIDs)
		if err != nil {
			return err
		}
		if err := patchManifestGrounding(manifestPath, staged); err != nil {
			return err
		}
		logf("INFO", "Grounding: %s/%s — packs %v (mode %s)", b.Skill, b.BatchNum, packIDs, mode)
	}
	return nil
}

func manifestChangedFiles(manifestPath string) ([]string, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}
	var raw struct {
		ReviewAgents map[string][]string `json:"review_agents"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode manifest %s: %w", manifestPath, err)
	}
	seen := map[string]struct{}{}
	var files []string
	for _, list := range raw.ReviewAgents {
		for _, f := range list {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			files = append(files, f)
		}
	}
	return files, nil
}

func patchManifestGrounding(manifestPath string, packs []agenting.StagedPack) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	entries := make([]map[string]string, 0, len(packs))
	for _, p := range packs {
		entries = append(entries, map[string]string{"id": p.ID, "file": p.Filename})
	}
	raw["grounding_packs"] = entries
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(manifestPath, out, 0o644)
}
