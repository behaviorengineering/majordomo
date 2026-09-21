package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func bootstrapContextBranch(ctxDir string, opts Options, repoID, sourceSHA string, at time.Time) error {
	policy := strings.ToLower(strings.TrimSpace(opts.BootstrapSurveyPolicy))
	if policy == "never" {
		return nil
	}
	runner := opts.BootstrapSurveyRunner
	if runner == nil {
		runner = LocalBootstrapSurveyRunner{}
	}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	analysisDir, err := cloneAnalysisRepoFn(ctx, opts.WorkDir)
	if err != nil {
		return fmt.Errorf("typology catch-up clone analysis: %w", err)
	}
	defer func() { _ = os.RemoveAll(analysisDir) }()

	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	input := BootstrapSurveyInput{
		AnalysisDir:    analysisDir,
		EvidenceDir:    evidenceDir,
		SourceSHA:      sourceSHA,
		RepoID:         repoID,
		TypologyBinary: opts.TypologyBinary,
		ModuleScope:    opts.ModuleScope,
		GeneratedAt:    at,
	}
	if err := runner.Survey(ctx, input); err != nil {
		return err
	}
	// Full seed walks refine (includes intervention) then story.
	if err := runBootstrapFromStage(ctx, ctxDir, analysisDir, sourceSHA, at, opts, ResumeStageCatalog); err != nil {
		return err
	}
	ctxGit := &Git{Dir: ctxDir}
	if err := configureCommitIdentity(ctxGit); err != nil {
		return err
	}
	msg := fmt.Sprintf("context bootstrap: seed snapshot at %s", shortSHA(sourceSHA))
	if committed, err := CommitAll(ctxGit, msg); err != nil {
		return err
	} else if !committed {
		return fmt.Errorf("bootstrap survey produced no changes to commit")
	}
	return contextstore.ValidateTree(ctxDir)
}

// refreshTypologyOnCatchUp surveys the current served tree when the cursor moves.
// An existing refined catalog keeps slices whose packages did not change.
// Slices that gained or lost a package are the only ones rewritten.
// The first map, with no refined catalog yet, still runs the full catalog pipeline.
func refreshTypologyOnCatchUp(ctx context.Context, ctxDir string, opts Options, repoID, sourceSHA string, at time.Time) error {
	policy := strings.ToLower(strings.TrimSpace(opts.BootstrapSurveyPolicy))
	if policy == "never" {
		return nil
	}
	if strings.TrimSpace(sourceSHA) == "" {
		return fmt.Errorf("typology catch-up: source sha is required")
	}
	runner := opts.BootstrapSurveyRunner
	if runner == nil {
		runner = LocalBootstrapSurveyRunner{}
	}
	if ctx == nil {
		return fmt.Errorf("typology catch-up: context is required")
	}

	analysisDir, err := cloneAnalysisRepoFn(ctx, opts.WorkDir)
	if err != nil {
		return fmt.Errorf("typology catch-up clone analysis: %w", err)
	}
	defer func() { _ = os.RemoveAll(analysisDir) }()

	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return fmt.Errorf("typology catch-up evidence dir: %w", err)
	}
	prev, hadPrev, err := loadRefinedCatalogIfPresent(filepath.Join(evidenceDir, refinedSnapshotRel))
	if err != nil {
		return fmt.Errorf("typology catch-up load refined catalog: %w", err)
	}
	input := BootstrapSurveyInput{
		AnalysisDir:    analysisDir,
		EvidenceDir:    evidenceDir,
		SourceSHA:      sourceSHA,
		RepoID:         repoID,
		TypologyBinary: opts.TypologyBinary,
		ModuleScope:    opts.ModuleScope,
		GeneratedAt:    at,
	}
	if err := runner.Survey(ctx, input); err != nil {
		return fmt.Errorf("typology catch-up survey: %w", err)
	}
	manifest, err := contextstore.ParseTypologyManifest(filepath.Join(evidenceDir, "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("typology catch-up parse survey manifest: %w", err)
	}
	if manifest.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(manifest.RefineStatus, contextstore.TypologyRefineSkipped) {
		logf("INFO", "typology catch-up skipped refine mode=%s status=%s", manifest.Mode, manifest.RefineStatus)
		return nil
	}
	rolesRel := strings.TrimSpace(manifest.PackageRolesPath)
	if rolesRel == "" {
		rolesRel = packageRolesRel
	}
	if err := pruneDraftPackagesAbsentFromRoles(
		filepath.Join(analysisDir, analysisDraftCatalogRel),
		filepath.Join(evidenceDir, rolesRel),
	); err != nil {
		return fmt.Errorf("typology catch-up prune draft: %w", err)
	}
	if !hadPrev {
		logf("INFO", "typology catch-up full refine source=%s", shortSHA(sourceSHA))
		if err := refineTypologyEvidence(ctx, opts, analysisDir, evidenceDir, opts.TypologySlicePipeline, opts.Judge); err != nil {
			return fmt.Errorf("typology catch-up refine: %w", err)
		}
		return nil
	}
	doc, err := loadPackageRoles(filepath.Join(evidenceDir, rolesRel))
	if err != nil {
		return fmt.Errorf("typology catch-up load package roles: %w", err)
	}
	graphRel := strings.TrimSpace(manifest.GraphPath)
	if graphRel == "" {
		graphRel = "graph.txt"
	}
	graphText, err := os.ReadFile(filepath.Join(evidenceDir, graphRel))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("typology catch-up read graph: %w", err)
	}
	delta := applyCatchUpSliceDelta(prev, roleByPath(doc), parseGraphImporters(string(graphText)))
	if len(delta.Changed) == 0 {
		logf("INFO", "typology catch-up unchanged slices source=%s", shortSHA(sourceSHA))
		return nil
	}
	logf("INFO", "typology catch-up rewrite slices=%s source=%s", strings.Join(delta.Changed, ","), shortSHA(sourceSHA))
	updated, err := rewriteChangedSlices(ctx, opts, analysisDir, evidenceDir, delta.Catalog, delta.Changed)
	if err != nil {
		return fmt.Errorf("typology catch-up rewrite slices: %w", err)
	}
	refinedRel := strings.TrimSpace(manifest.RefinedSnapshotPath)
	if refinedRel == "" {
		refinedRel = refinedSnapshotRel
	}
	if err := saveRefinedCatalog(filepath.Join(evidenceDir, refinedRel), updated); err != nil {
		return fmt.Errorf("typology catch-up save refined catalog: %w", err)
	}
	return nil
}

// runBootstrapFromStage runs the post-survey seed chain starting at fromStage.
// Resume mode skips survey; full seed calls this with catalog after survey.
//
//	catalog      → refineTypologyEvidence (includes flagHumanIntervention) → story
//	intervention → flagHumanIntervention → story
//	story        → writeBootstrapStory only
func runBootstrapFromStage(ctx context.Context, ctxDir, analysisDir, sourceSHA string, at time.Time, opts Options, fromStage string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	stage := normalizeResumeStage(fromStage)
	judgeGen := opts.Judge

	switch stage {
	case ResumeStageCatalog:
		manifest, err := contextstore.ParseTypologyManifest(filepath.Join(evidenceDir, "manifest.yaml"))
		if err != nil {
			return err
		}
		if isPRResumeMode(opts) {
			if err := stageAnalysisDraftsFromEvidence(analysisDir, evidenceDir, manifest); err != nil {
				return err
			}
		}
		if err := refineTypologyEvidence(ctx, opts, analysisDir, evidenceDir, opts.TypologySlicePipeline, judgeGen); err != nil {
			return err
		}
		if err := writeBootstrapStory(ctx, ctxDir, analysisDir, at, sourceSHA, opts); err != nil {
			return err
		}
	case ResumeStageIntervention:
		if err := flagHumanIntervention(ctx, evidenceDir, opts.HumanInterventionGenerator, judgeGen, opts); err != nil {
			return err
		}
		if err := writeBootstrapStory(ctx, ctxDir, analysisDir, at, sourceSHA, opts); err != nil {
			return err
		}
	case ResumeStageStory:
		if err := writeBootstrapStory(ctx, ctxDir, analysisDir, at, sourceSHA, opts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("bootstrap from-stage %q is not supported", fromStage)
	}

	if err := contextstore.ApplyReadingPath(ctxDir); err != nil {
		return err
	}
	return nil
}
