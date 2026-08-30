package contextstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/agenting"
)

// RequiredFiles is the canonical context-branch tree (relative paths).
var RequiredFiles = []string{
	"README.md",
	"meta.yaml",
	"mission.md",
	"architecture.md",
	"conventions.md",
	"weaknesses.md",
	"chronology.md",
}

// ValidateTree checks that dir is a complete, schema-valid context worktree.
func ValidateTree(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("context validate: dir is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("context tree %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("context tree %s is not a directory", dir)
	}
	for _, name := range RequiredFiles {
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("context tree missing %s: %w", name, err)
		}
		if st.IsDir() {
			return fmt.Errorf("context tree %s is a directory, want a file", name)
		}
	}
	meta, err := ParseMeta(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return err
	}
	if err := ValidateMeta(meta); err != nil {
		return err
	}
	_, err = ParseChronologyFile(filepath.Join(dir, "chronology.md"))
	if err != nil {
		return err
	}
	return validateAgenting(dir)
}

func validateAgenting(dir string) error {
	indexPath := filepath.Join(dir, agenting.IndexRelPath)
	if _, err := os.Stat(indexPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("agenting index: %w", err)
	}
	idx, err := agenting.LoadIndex(dir)
	if err != nil {
		return err
	}
	for _, id := range idx.PackIDs() {
		grounding := filepath.Join(dir, "agenting", id, agenting.GroundingName)
		st, err := os.Stat(grounding)
		if err != nil {
			return fmt.Errorf("agenting pack %q missing %s: %w", id, agenting.GroundingName, err)
		}
		if st.IsDir() {
			return fmt.Errorf("agenting pack %q: %s is a directory", id, agenting.GroundingName)
		}
	}
	return nil
}
