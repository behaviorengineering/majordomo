package contextstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/agenting"
	"gopkg.in/yaml.v3"
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
	if err := validateTypologyEvidence(dir); err != nil {
		return err
	}
	return validateAgenting(dir)
}

func validateTypologyEvidence(dir string) error {
	evidenceDir := filepath.Join(dir, "evidence", "typology")
	st, err := os.Stat(evidenceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("typology evidence: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("typology evidence %s is not a directory", evidenceDir)
	}

	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	manifest, err := ParseTypologyManifest(manifestPath)
	if err != nil {
		return err
	}
	if err := ValidateTypologyManifest(manifest); err != nil {
		return err
	}
	if err := validateTypologyEvidenceFile(evidenceDir, manifest.ArchitecturePath); err != nil {
		return err
	}
	if strings.TrimSpace(manifest.SnapshotPath) != "" {
		if err := validateTypologyEvidenceFile(evidenceDir, manifest.SnapshotPath); err != nil {
			return err
		}
		if err := validateTypologySnapshot(filepath.Join(evidenceDir, manifest.SnapshotPath)); err != nil {
			return err
		}
	}
	refine := strings.ToLower(strings.TrimSpace(manifest.RefineStatus))
	if refine == TypologyRefineComplete {
		for _, rel := range []string{
			manifest.GraphPath,
			manifest.PackageContractsPath,
			manifest.ClusterProposalPath,
			manifest.RefinedSnapshotPath,
			manifest.JourneyPath,
			manifest.HumanInterventionPath,
		} {
			if err := validateTypologyEvidenceFile(evidenceDir, rel); err != nil {
				return err
			}
		}
		if err := validateTypologySnapshot(filepath.Join(evidenceDir, manifest.RefinedSnapshotPath)); err != nil {
			return err
		}
	}
	if refine == TypologyRefinePending {
		for _, rel := range []string{manifest.GraphPath, manifest.PackageContractsPath} {
			if strings.TrimSpace(rel) == "" {
				continue
			}
			if err := validateTypologyEvidenceFile(evidenceDir, rel); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTypologyEvidenceFile(baseDir, relPath string) error {
	joined := filepath.Join(baseDir, relPath)
	st, err := os.Stat(joined)
	if err != nil {
		return fmt.Errorf("typology evidence missing %s: %w", relPath, err)
	}
	if st.IsDir() {
		return fmt.Errorf("typology evidence %s is a directory", relPath)
	}
	return nil
}

func validateTypologySnapshot(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read typology snapshot: %w", err)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse typology snapshot: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("typology snapshot %s is empty", path)
	}
	return nil
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
