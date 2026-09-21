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
		return err
	}
	defer os.RemoveAll(analysisDir)

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
