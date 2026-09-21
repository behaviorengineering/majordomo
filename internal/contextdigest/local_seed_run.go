package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func isLocalSeedMode(opts Options) bool {
	return strings.TrimSpace(opts.LocalSeedDir) != ""
}

func isPRResumeMode(opts Options) bool {
	return opts.ResumePR > 0
}

func validateLocalSeedOptions(opts Options) error {
	if !isLocalSeedMode(opts) {
		return nil
	}
	if opts.ResumePR > 0 {
		return fmt.Errorf("--local-seed-dir conflicts with --resume-pr")
	}
	stage := normalizeResumeStage(opts.FromStage)
	switch stage {
	case "", LocalStageSurvey, LocalStageCatalog, LocalStageIntervention, LocalStageStory:
	default:
		return fmt.Errorf("unsupported --from-stage %q for local seed (want survey|catalog|intervention|story)", opts.FromStage)
	}
	if opts.SkipStory && stage == LocalStageStory {
		return fmt.Errorf("--skip-story conflicts with --from-stage story")
	}
	return nil
}

func validateDigestModeOptions(opts Options) error {
	if isLocalSeedMode(opts) {
		return validateLocalSeedOptions(opts)
	}
	return validateResumeOptions(opts)
}

func runLocalSeed(opts Options, cfg config.RepoConfig, now time.Time) (Result, error) {
	servedGit := &Git{Dir: opts.WorkDir}
	sourceSHA, err := servedGit.trim("rev-parse", "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("local seed resolve workdir HEAD: %w", err)
	}

	ws, err := OpenLocalSeedWorkspace(opts.LocalSeedDir, opts.RepoID, sourceSHA, opts.ModuleScope, opts.AllowSourceMove, now)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = ws.Release() }()

	fromStage := normalizeResumeStage(opts.FromStage)
	if fromStage == "" {
		if ws.IsNew() {
			fromStage = LocalStageSurvey
		} else {
			fromStage = nextLocalStage(ws.Manifest.CompletedStage)
			if fromStage == "" {
				logf("INFO", "local seed workspace already complete at %s", ws.Manifest.CompletedStage)
				return Result{
					Action:         "local",
					DefaultHEAD:    sourceSHA,
					Message:        "local seed workspace already complete",
					WorkStoryDir:   chooseLocalWorkStory(opts, ws),
					LocalSeedDir:   ws.Root,
					CacheMode:      "local",
					SourceSHA:      ws.Manifest.SourceSHA,
					CompletedStage: ws.Manifest.CompletedStage,
					LocalOutDir:    ws.ContextDir(),
				}, nil
			}
		}
	}
	if fromStage == LocalStageSurvey && !ws.IsNew() && strings.TrimSpace(ws.Manifest.CompletedStage) != "" {
		return Result{}, fmt.Errorf("--from-stage survey only valid for a new workspace (completed=%q); use a fresh --local-seed-dir", ws.Manifest.CompletedStage)
	}
	if err := localStageReady(ws.Manifest.CompletedStage, fromStage); err != nil {
		return Result{}, err
	}

	if strings.TrimSpace(opts.WorkStoryDir) == "" {
		opts.WorkStoryDir = ws.WorkStoryDir()
	}
	closeTrace, err := prepareWorkStory(&opts, now)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = closeTrace() }()

	if err := ensureDigestJudge(&opts, cfg); err != nil {
		return Result{}, err
	}

	store := &cache.DigestStore{Dir: ws.CacheDir()}
	opts.DigestCache = store
	opts.DigestSkips = cfg.Cache.SkipsEnabled()
	opts.DigestModelID = digestModelID(cfg)
	logf("INFO", "cache_mode=local context_publish=disabled dir=%s skips=%v model=%s", ws.CacheDir(), opts.DigestSkips, opts.DigestModelID)
	defer func() {
		if ferr := store.Flush(); ferr != nil {
			logf("WARN", "local digest cache flush: %v", ferr)
		}
		logf("INFO", "%s", cache.FormatStatsLine(store.Stats()))
	}()

	ctxDir := ws.ContextDir()
	if ws.IsNew() || !treeHasRequiredContext(ctxDir) {
		if err := contextstore.Bootstrap(ctxDir, opts.RepoID, sourceSHA, now); err != nil {
			return Result{}, fmt.Errorf("local seed bootstrap context: %w", err)
		}
	}

	logf("INFO", "local_seed dir=%s source=%s from_stage=%s completed=%s (no forge token, no context/cache push)",
		ws.Root, shortSHA(sourceSHA), fromStage, ws.Manifest.CompletedStage)

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	analysisDir, err := cloneAnalysisRepoFn(ctx, opts.WorkDir)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(analysisDir)

	if fromStage != LocalStageSurvey {
		if err := ws.RestoreAnalysisDrafts(analysisDir); err != nil {
			return Result{}, err
		}
	}

	runErr := runLocalStages(ctx, ws, opts, cfg, analysisDir, sourceSHA, now, fromStage)
	if runErr != nil {
		_ = ws.RecordFailure(fromStage, runErr, now)
		return Result{}, runErr
	}

	diffBody := fmt.Sprintf("local seed complete repo=%s source=%s completed=%s\n", opts.RepoID, sourceSHA, ws.Manifest.CompletedStage)
	_ = ws.WriteLocalDiff(diffBody)

	return Result{
		Action:         "local",
		DefaultHEAD:    sourceSHA,
		Message:        fmt.Sprintf("local seed complete at stage %s", ws.Manifest.CompletedStage),
		WorkStoryDir:   opts.WorkStoryDir,
		LocalSeedDir:   ws.Root,
		CacheMode:      "local",
		SourceSHA:      ws.Manifest.SourceSHA,
		CompletedStage: ws.Manifest.CompletedStage,
		FromStage:      fromStage,
		LocalOutDir:    ws.ContextDir(),
	}, nil
}

func chooseLocalWorkStory(opts Options, ws *LocalSeedWorkspace) string {
	if s := strings.TrimSpace(opts.WorkStoryDir); s != "" {
		return s
	}
	return ws.WorkStoryDir()
}

func treeHasRequiredContext(ctxDir string) bool {
	_, err := os.Stat(filepath.Join(ctxDir, "meta.yaml"))
	return err == nil
}

func nextLocalStage(completed string) string {
	switch normalizeResumeStage(completed) {
	case "":
		return LocalStageSurvey
	case LocalStageSurvey:
		return LocalStageCatalog
	case LocalStageCatalog, LocalStageIntervention:
		return LocalStageStory
	case LocalStageStory:
		return ""
	default:
		return LocalStageSurvey
	}
}

func runLocalStages(ctx context.Context, ws *LocalSeedWorkspace, opts Options, cfg config.RepoConfig, analysisDir, sourceSHA string, now time.Time, fromStage string) error {
	policy := strings.ToLower(strings.TrimSpace(opts.BootstrapSurveyPolicy))
	ctxDir := ws.ContextDir()
	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	judgeGen := opts.Judge

	start := normalizeResumeStage(fromStage)
	runSurvey := start == LocalStageSurvey
	runCatalog := start == LocalStageSurvey || start == LocalStageCatalog
	runInterventionOnly := start == LocalStageIntervention
	runStory := start == LocalStageSurvey || start == LocalStageCatalog || start == LocalStageIntervention || start == LocalStageStory
	if opts.SkipStory {
		runStory = false
	}

	if runSurvey {
		if policy == "never" {
			logf("INFO", "local seed survey skipped (bootstrap-survey-policy=never)")
			if err := ws.SaveCheckpoint(LocalStageSurvey, now); err != nil {
				return err
			}
			// Match remote seed: policy never produces no typology evidence, so do not
			// continue into catalog/story in the same invocation that started at survey.
			if start == LocalStageSurvey {
				runCatalog = false
				runInterventionOnly = false
				runStory = false
			}
		} else {
			if err := ws.BeginStage(LocalStageSurvey, now); err != nil {
				return err
			}
			runner := opts.BootstrapSurveyRunner
			if runner == nil {
				runner = LocalBootstrapSurveyRunner{}
			}
			input := BootstrapSurveyInput{
				AnalysisDir:    analysisDir,
				EvidenceDir:    evidenceDir,
				SourceSHA:      sourceSHA,
				RepoID:         opts.RepoID,
				TypologyBinary: opts.TypologyBinary,
				ModuleScope:    opts.ModuleScope,
				GeneratedAt:    now,
			}
			if err := runner.Survey(ctx, input); err != nil {
				return err
			}
			if err := ws.PersistAnalysisDrafts(analysisDir); err != nil {
				return err
			}
			if err := ws.SaveCheckpoint(LocalStageSurvey, now); err != nil {
				return err
			}
			logf("INFO", "local seed checkpoint completed=survey")
		}
	}

	if runCatalog {
		if err := ws.BeginStage(LocalStageCatalog, now); err != nil {
			return err
		}
		if err := refineTypologyEvidence(ctx, opts, analysisDir, evidenceDir, opts.TypologySlicePipeline, judgeGen); err != nil {
			return err
		}
		if err := ws.PersistAnalysisDrafts(analysisDir); err != nil {
			return err
		}
		// refineTypologyEvidence includes flagHumanIntervention.
		if err := ws.SaveCheckpoint(LocalStageIntervention, now); err != nil {
			return err
		}
		logf("INFO", "local seed checkpoint completed=intervention (refine+human-intervention)")
	}

	if runInterventionOnly {
		if err := ws.BeginStage(LocalStageIntervention, now); err != nil {
			return err
		}
		if err := flagHumanIntervention(ctx, evidenceDir, opts.HumanInterventionGenerator, judgeGen, opts); err != nil {
			return err
		}
		if err := ws.SaveCheckpoint(LocalStageIntervention, now); err != nil {
			return err
		}
		logf("INFO", "local seed checkpoint completed=intervention")
	}

	if runStory {
		if err := ws.BeginStage(LocalStageStory, now); err != nil {
			return err
		}
		if err := writeBootstrapStory(ctx, ctxDir, analysisDir, now, sourceSHA, opts); err != nil {
			return err
		}
		if err := contextstore.ApplyReadingPath(ctxDir); err != nil {
			return err
		}
		if err := ws.SaveCheckpoint(LocalStageStory, now); err != nil {
			return err
		}
		logf("INFO", "local seed checkpoint completed=story")
	} else if runCatalog || runInterventionOnly || runSurvey {
		if err := contextstore.ApplyReadingPath(ctxDir); err != nil {
			return err
		}
	}

	_ = cfg
	return nil
}
