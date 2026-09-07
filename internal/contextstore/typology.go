package contextstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	TypologyModeDiscover = "discover"
	TypologyModeReuse    = "reuse"
	TypologyModeFallback = "fallback"

	TypologyRefinePending  = "pending"
	TypologyRefineComplete = "complete"
	TypologyRefineSkipped  = "skipped"

	// TypologyArchitectureBriefPath is the evidence file for the Typology architecture brief.
	// It is intentionally not named architecture.md so PR reviewers do not confuse it with
	// the teaching-story root architecture.md.
	TypologyArchitectureBriefPath = "architecture_brief.md"
)

// TypologyManifest records how a bootstrap survey was produced.
// Discover drafts stay ephemeral under the analysis worktree and are not
// listed here for committed evidence.
type TypologyManifest struct {
	RepoID                string `yaml:"repo_id,omitempty"`
	SourceSHA             string `yaml:"source_sha"`
	GeneratedAt           string `yaml:"generated_at,omitempty"`
	TypologyVersion       string `yaml:"typology_version,omitempty"`
	Mode                  string `yaml:"mode"`
	ModuleScope           string `yaml:"module_scope,omitempty"`
	SnapshotPath          string `yaml:"snapshot_path,omitempty"`
	ArchitecturePath      string `yaml:"architecture_path"`
	RefineStatus          string `yaml:"refine_status,omitempty"`
	GraphPath             string `yaml:"graph_path,omitempty"`
	PackageContractsPath  string `yaml:"package_contracts_path,omitempty"`
	ClusterProposalPath   string `yaml:"cluster_proposal_path,omitempty"`
	RefinedSnapshotPath   string `yaml:"refined_snapshot_path,omitempty"`
	JourneyPath           string `yaml:"journey_path,omitempty"`
	HumanInterventionPath string `yaml:"human_intervention_path,omitempty"`
}

// ParseTypologyManifest reads a manifest from path.
func ParseTypologyManifest(path string) (TypologyManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TypologyManifest{}, fmt.Errorf("read typology manifest: %w", err)
	}
	var m TypologyManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return TypologyManifest{}, fmt.Errorf("parse typology manifest: %w", err)
	}
	return m, nil
}

// ValidateTypologyManifest checks the bootstrap evidence contract.
func ValidateTypologyManifest(m TypologyManifest) error {
	if strings.TrimSpace(m.RepoID) == "" {
		return fmt.Errorf("typology manifest repo_id is required")
	}
	if strings.TrimSpace(m.SourceSHA) == "" {
		return fmt.Errorf("typology manifest source_sha is required")
	}
	if ts := strings.TrimSpace(m.GeneratedAt); ts != "" {
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			return fmt.Errorf("typology manifest generated_at %q is not RFC3339: %w", m.GeneratedAt, err)
		}
	}
	mode := strings.ToLower(strings.TrimSpace(m.Mode))
	switch mode {
	case TypologyModeDiscover, TypologyModeReuse, TypologyModeFallback:
	default:
		return fmt.Errorf("typology manifest mode %q is not supported", m.Mode)
	}
	if err := validateRelativeEvidencePath(m.ArchitecturePath, "architecture_path"); err != nil {
		return err
	}
	if strings.TrimSpace(m.SnapshotPath) != "" {
		if err := validateRelativeEvidencePath(m.SnapshotPath, "snapshot_path"); err != nil {
			return err
		}
	}
	refine := strings.ToLower(strings.TrimSpace(m.RefineStatus))
	switch refine {
	case "", TypologyRefinePending, TypologyRefineComplete, TypologyRefineSkipped:
	default:
		return fmt.Errorf("typology manifest refine_status %q is not supported", m.RefineStatus)
	}
	if mode == TypologyModeFallback {
		return nil
	}
	switch refine {
	case TypologyRefinePending:
		if err := validateRelativeEvidencePath(m.GraphPath, "graph_path"); err != nil {
			return err
		}
		if err := validateRelativeEvidencePath(m.PackageContractsPath, "package_contracts_path"); err != nil {
			return err
		}
	case TypologyRefineComplete:
		if strings.TrimSpace(m.SnapshotPath) == "" {
			return fmt.Errorf("typology manifest snapshot_path is required for mode %q", m.Mode)
		}
		for _, pair := range []struct {
			path, field string
		}{
			{m.GraphPath, "graph_path"},
			{m.PackageContractsPath, "package_contracts_path"},
			{m.ClusterProposalPath, "cluster_proposal_path"},
			{m.RefinedSnapshotPath, "refined_snapshot_path"},
			{m.JourneyPath, "journey_path"},
			{m.HumanInterventionPath, "human_intervention_path"},
		} {
			if err := validateRelativeEvidencePath(pair.path, pair.field); err != nil {
				return err
			}
		}
	default:
		// Legacy evidence: snapshot + architecture only.
		if strings.TrimSpace(m.SnapshotPath) == "" {
			return fmt.Errorf("typology manifest snapshot_path is required for mode %q", m.Mode)
		}
	}
	return nil
}

func validateRelativeEvidencePath(path, field string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return fmt.Errorf("typology manifest %s is required", field)
	}
	if filepath.IsAbs(p) {
		return fmt.Errorf("typology manifest %s must be relative, got %q", field, path)
	}
	clean := filepath.Clean(p)
	if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return fmt.Errorf("typology manifest %s must stay within evidence/typology, got %q", field, path)
	}
	return nil
}
