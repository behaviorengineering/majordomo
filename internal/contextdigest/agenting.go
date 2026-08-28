package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/agenting"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

// MaterializeAgenting regenerates grounding packs from story files after digest.
func MaterializeAgenting(ctxDir string, changedFiles []string) error {
	mission, err := os.ReadFile(filepath.Join(ctxDir, "mission.md"))
	if err != nil {
		return err
	}
	arch, err := os.ReadFile(filepath.Join(ctxDir, "architecture.md"))
	if err != nil {
		return err
	}
	overview := fmt.Sprintf("# Overview\n\n%s\n\n## Architecture\n\n%s\n",
		strings.TrimSpace(string(mission)), strings.TrimSpace(string(arch)))
	overviewDir := filepath.Join(ctxDir, "agenting", "overview")
	if err := os.MkdirAll(overviewDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(overviewDir, agenting.GroundingName), []byte(overview), 0o644); err != nil {
		return err
	}

	if _, statErr := os.Stat(filepath.Join(ctxDir, agenting.IndexRelPath)); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return statErr
	}
	idx, err := agenting.LoadIndex(ctxDir)
	if err != nil {
		return err
	}
	for _, id := range idx.PackIDs() {
		if id == "overview" {
			continue
		}
		pack, ok := idx.Packs[id]
		if !ok || len(pack.Globs) == 0 {
			continue
		}
		if !anyPathMatches(changedFiles, pack.Globs) {
			continue
		}
		packDir := filepath.Join(ctxDir, "agenting", id)
		if err := os.MkdirAll(packDir, 0o755); err != nil {
			return err
		}
		body := fmt.Sprintf("# %s\n\nGrounding for `%s` from digest (files touched: %s).\n",
			id, id, strings.Join(changedFiles, ", "))
		if err := os.WriteFile(filepath.Join(packDir, agenting.GroundingName), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return contextstore.ValidateTree(ctxDir)
}

// CollectChangedFiles unions file paths from commit contexts.
func CollectChangedFiles(commits []CommitContext) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range commits {
		for _, f := range c.Files {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out
}
