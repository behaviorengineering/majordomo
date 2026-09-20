package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

// validateResumeEvidence fail-closes when the PR head tree lacks files required for from-stage.
func validateResumeEvidence(ctxDir, stage string) error {
	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	manifest, err := contextstore.ParseTypologyManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("resume evidence: %w", err)
	}
	if err := contextstore.ValidateTypologyManifest(manifest); err != nil {
		return fmt.Errorf("resume evidence manifest: %w", err)
	}
	stage = normalizeResumeStage(stage)
	switch stage {
	case ResumeStageCatalog:
		return requireEvidenceFiles(evidenceDir, refineEvidenceReqs(manifest))
	case ResumeStageIntervention:
		return requireEvidenceFiles(evidenceDir, interventionEvidenceReqs(manifest))
	case ResumeStageStory:
		if err := assertStoryResumeReady(manifest); err != nil {
			return err
		}
		return requireEvidenceFiles(evidenceDir, storyEvidenceReqs(manifest))
	default:
		return fmt.Errorf("resume evidence: unsupported stage %q", stage)
	}
}

type evidenceReq struct {
	field string
	rel   string
}

func refineEvidenceReqs(m contextstore.TypologyManifest) []evidenceReq {
	snap := strings.TrimSpace(m.SnapshotPath)
	if snap == "" {
		snap = "snapshot.yaml"
	}
	graph := strings.TrimSpace(m.GraphPath)
	if graph == "" {
		graph = "graph.txt"
	}
	contracts := strings.TrimSpace(m.PackageContractsPath)
	if contracts == "" {
		contracts = "package_contracts.md"
	}
	roles := strings.TrimSpace(m.PackageRolesPath)
	if roles == "" {
		roles = packageRolesRel
	}
	return []evidenceReq{
		{field: "snapshot_path (draft)", rel: snap},
		{field: "graph_path", rel: graph},
		{field: "package_contracts_path", rel: contracts},
		{field: "package_roles_path", rel: roles},
	}
}

func interventionEvidenceReqs(m contextstore.TypologyManifest) []evidenceReq {
	refined := strings.TrimSpace(m.RefinedSnapshotPath)
	if refined == "" {
		refined = refinedSnapshotRel
	}
	journey := strings.TrimSpace(m.JourneyPath)
	if journey == "" {
		journey = "journey.md"
	}
	arch := strings.TrimSpace(m.ArchitecturePath)
	if arch == "" {
		arch = contextstore.TypologyArchitectureBriefPath
	}
	return []evidenceReq{
		{field: "refined_snapshot_path", rel: refined},
		{field: "journey_path", rel: journey},
		{field: "architecture_path", rel: arch},
	}
}

func assertStoryResumeReady(m contextstore.TypologyManifest) error {
	if m.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(m.RefineStatus, contextstore.TypologyRefineSkipped) {
		return nil
	}
	if strings.TrimSpace(m.RefinedSnapshotPath) == "" {
		return fmt.Errorf("resume evidence: story requires refined_snapshot_path on the PR tree")
	}
	if strings.TrimSpace(m.SliceObjectiveLedgerPath) == "" {
		return fmt.Errorf("resume evidence: story requires slice_objective_ledger_path on the PR tree")
	}
	return nil
}

func storyEvidenceReqs(m contextstore.TypologyManifest) []evidenceReq {
	arch := strings.TrimSpace(m.ArchitecturePath)
	if arch == "" {
		arch = contextstore.TypologyArchitectureBriefPath
	}
	reqs := []evidenceReq{{field: "architecture_path", rel: arch}}
	if m.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(m.RefineStatus, contextstore.TypologyRefineSkipped) {
		return reqs
	}
	refined := strings.TrimSpace(m.RefinedSnapshotPath)
	ledger := strings.TrimSpace(m.SliceObjectiveLedgerPath)
	if ledger == "" {
		ledger = sliceObjectiveLedgerRel
	}
	reqs = append(reqs,
		evidenceReq{field: "refined_snapshot_path", rel: refined},
		evidenceReq{field: "slice_objective_ledger_path", rel: ledger},
	)
	return reqs
}

func requireEvidenceFiles(evidenceDir string, reqs []evidenceReq) error {
	var missing []string
	for _, r := range reqs {
		rel := strings.TrimSpace(r.rel)
		if rel == "" {
			missing = append(missing, r.field+" (empty path)")
			continue
		}
		path := filepath.Join(evidenceDir, rel)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			missing = append(missing, fmt.Sprintf("%s=%s", r.field, rel))
			continue
		}
		if st.Size() == 0 {
			missing = append(missing, fmt.Sprintf("%s=%s (empty)", r.field, rel))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("resume evidence missing required files for stage: %s", strings.Join(missing, ", "))
	}
	return nil
}

// stageAnalysisDraftsFromEvidence copies PR evidence draft catalog + architecture brief into
// the analysis worktree paths refineTypologyEvidence expects.
func stageAnalysisDraftsFromEvidence(analysisDir, evidenceDir string, m contextstore.TypologyManifest) error {
	snap := strings.TrimSpace(m.SnapshotPath)
	if snap == "" {
		snap = "snapshot.yaml"
	}
	arch := strings.TrimSpace(m.ArchitecturePath)
	if arch == "" {
		arch = contextstore.TypologyArchitectureBriefPath
	}
	dstDir := filepath.Join(analysisDir, "tmp", "typology")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("stage analysis drafts mkdir: %w", err)
	}
	if err := copyFile(filepath.Join(evidenceDir, snap), filepath.Join(dstDir, "typology.yaml")); err != nil {
		return fmt.Errorf("stage analysis draft catalog: %w", err)
	}
	if err := copyFile(filepath.Join(evidenceDir, arch), filepath.Join(dstDir, "architecture_draft.md")); err != nil {
		return fmt.Errorf("stage analysis architecture draft: %w", err)
	}
	return nil
}
