package orchestrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/behaviorengineering/majordomo/internal/agent"
	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/filereview"
	"github.com/behaviorengineering/majordomo/internal/judge"
	"github.com/behaviorengineering/majordomo/internal/report"
	"github.com/behaviorengineering/majordomo/internal/staging"
)

// Options configures an orchestrate run.
type Options struct {
	PRNumber    string
	BaseBranch  string
	StagingDir  string
	OutputDir   string // pipeline output root, e.g. copilot-review-pr-42/pr-review
	Pipeline    string // COPILOT_PIPELINE / label (default pr-review)
	Concurrency int
	ScriptsDir  string
	SkipPrep    bool
	SkipDeep    bool
	SkipReport  bool
	RepoRoot    string // for prep / deep; empty = cwd

	RoutingPath       string
	AgentContextPath  string
	SummaryConfigPath string
	ContextDir        string // merged context-branch checkout (agenting); optional

	// ConfigDir + RepoID load majordomo-central-config and materialize routing/agentContext
	// when RoutingPath / AgentContextPath are empty.
	ConfigDir string
	RepoID    string

	// BatchTimeout for a single dispatch; 0 → COPILOT_BATCH_TIMEOUT_MINUTES or 8m.
	BatchTimeout time.Duration

	// Injectables for tests
	Dispatch   func(agent.DispatchOptions) error
	RunSummary func(agent.SummaryLoopOptions) error
	RunTech    func(agent.TechLoopOptions) error
}

// Run executes the full review orchestration (prep optional → waves → finalize → synthesis).
func Run(opts Options) error {
	if opts.PRNumber == "" || opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("orchestrate requires --pr, --staging-dir, and --output-dir")
	}
	if opts.Pipeline == "" {
		opts.Pipeline = envOr("COPILOT_PIPELINE", "pr-review")
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = envInt("COPILOT_CONCURRENCY", 6)
	}
	if opts.BatchTimeout <= 0 {
		mins := envInt("COPILOT_BATCH_TIMEOUT_MINUTES", 8)
		opts.BatchTimeout = time.Duration(mins) * time.Minute
	}
	if opts.Dispatch == nil {
		opts.Dispatch = agent.Dispatch
	}
	if opts.RunSummary == nil {
		opts.RunSummary = agent.RunSummaryLoop
	}
	if opts.RunTech == nil {
		opts.RunTech = agent.RunTechLoop
	}

	logf := agent.Logf
	logf("INFO", "========== majordomo orchestrate ==========")
	judgeMode, err := judge.ResolveMode()
	if err != nil {
		return err
	}
	logf("INFO", "PR: %s  pipeline: %s  concurrency: %d  judge: %s", opts.PRNumber, opts.Pipeline, opts.Concurrency, judgeMode)
	if judgeMode == judge.ModeStrop {
		if err := judge.EnsureStropReady(); err != nil {
			return err
		}
		stropDispatch := func(opts agent.DispatchOptions) error {
			return judge.StropDispatch(judge.StropDispatchOptions{
				StagingDir: opts.StagingDir,
				OutputDir:  opts.OutputDir,
				Mode:       judge.DispatchMode(opts.Mode),
			})
		}
		opts.Dispatch = stropDispatch
	}

	if !opts.SkipPrep {
		if opts.BaseBranch == "" {
			return fmt.Errorf("--base-branch required unless --skip-prep")
		}
		routingPath, agentContextPath, cfg, err := config.ResolvePrepPaths(
			opts.ConfigDir, opts.RepoID, opts.Pipeline,
			config.MaterializeDirForStaging(opts.StagingDir),
			opts.RoutingPath, opts.AgentContextPath,
		)
		if err != nil {
			return fmt.Errorf("central config: %w", err)
		}
		if opts.ConfigDir != "" && opts.RepoID != "" {
			config.ApplyPipelineModelEnv(cfg, opts.Pipeline)
		}
		logf("INFO", "Running prep against %s → %s", opts.BaseBranch, opts.StagingDir)
		err = staging.Run(staging.Options{
			BaseBranch:        opts.BaseBranch,
			StagingDir:        opts.StagingDir,
			RoutingPath:       routingPath,
			AgentContextPath:  agentContextPath,
			SummaryConfigPath: opts.SummaryConfigPath,
			RepoRoot:          opts.RepoRoot,
			ContextDir:        staging.ResolveContextDir(opts.ContextDir),
		})
		if err != nil {
			if errors.Is(err, staging.ErrNothingToReview) {
				logf("INFO", "prep: nothing to review — skipping")
				return nil
			}
			return fmt.Errorf("prep: %w", err)
		}
	} else if opts.ConfigDir != "" && opts.RepoID != "" {
		cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
		if err != nil {
			return fmt.Errorf("central config: %w", err)
		}
		config.ApplyPipelineModelEnv(cfg, opts.Pipeline)
	}

	planPath := filepath.Join(opts.StagingDir, "batch-plan.json")
	plan, err := LoadBatchPlan(planPath)
	if err != nil {
		return err
	}
	fileBatches, synthBatches := SplitBatches(plan.Batches)
	logf("INFO", "%d batch(es) across skill(s): %v", len(plan.Batches), plan.Skills)
	logf("INFO", "Phase 1 — file-review batches: %d", len(fileBatches))
	logf("INFO", "Phase 2 — synthesis batches:   %d", len(synthBatches))

	if err := runFileWaves(opts, fileBatches); err != nil {
		return err
	}
	if err := runFinalizeAndProse(opts, plan.Skills); err != nil {
		return err
	}
	if err := runSynthesis(opts, synthBatches); err != nil {
		return err
	}
	if err := runSynthesisProse(opts, synthBatches); err != nil {
		return err
	}
	if !opts.SkipDeep {
		if err := runTechDeep(opts); err != nil {
			return err
		}
	}

	// Copy top-level staging manifest into output for archiving
	srcManifest := filepath.Join(opts.StagingDir, "manifest.json")
	dstManifest := filepath.Join(opts.OutputDir, "review-manifest.json")
	if FileExists(srcManifest) {
		if err := copyFile(srcManifest, dstManifest); err != nil {
			return fmt.Errorf("copy review-manifest.json: %w", err)
		}
	}

	if !opts.SkipReport {
		junitDir := filepath.Join(opts.OutputDir, "junit")
		// Convert parent of pipeline output if structure is outputBase/pipeline —
		// ConvertToJUnit expects the PR output root. Here OutputDir is
		// already the pipeline dir; convert the parent if it contains pipelines.
		reviewRoot := filepath.Dir(opts.OutputDir)
		if reviewRoot == "." || reviewRoot == "" {
			reviewRoot = opts.OutputDir
		}
		logf("INFO", "Writing JUnit under %s", junitDir)
		if err := report.ConvertToJUnit(reviewRoot, junitDir); err != nil {
			return fmt.Errorf("junit report: %w", err)
		}
	}

	logf("INFO", "Orchestrate complete")
	return nil
}

func runFileWaves(opts Options, fileBatches []BatchEntry) error {
	waves := ChunkBatches(fileBatches, opts.Concurrency)
	for wi, wave := range waves {
		agent.Logf("INFO", "Wave %d: %d batch(es)", wi+1, len(wave))
		var wg sync.WaitGroup
		errCh := make(chan error, len(wave))
		for _, batch := range wave {
			b := batch
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := runOneFileBatch(opts, b); err != nil {
					errCh <- err
				}
			}()
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func runOneFileBatch(opts Options, b BatchEntry) error {
	skillOut := filepath.Join(opts.OutputDir, b.Skill)
	cp := CheckpointPath(skillOut, b.BatchNum)
	label := b.Skill + " / batch_" + b.BatchNum
	_ = os.MkdirAll(filepath.Join(skillOut, "logs"), 0o755)
	if FileExists(cp) {
		agent.Logf("INFO", "%s: checkpoint — skipping", label)
		return nil
	}
	judgeMode, _ := judge.ResolveMode()

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		lastErr = filereview.Run(filereview.Options{
			StagingDir: b.StagingDir,
			SkillOut:   skillOut,
			MaxRetries: 2,
			Logf: func(format string, args ...any) {
				agent.Logf("INFO", "%s: "+format, append([]any{label}, args...)...)
			},
			Judge: func() error {
				if judgeMode == judge.ModeStrop {
					return judge.FileReviewBatch(judge.FileReviewOptions{
						StagingDir: b.StagingDir,
						SkillOut:   skillOut,
					})
				}
				return opts.Dispatch(agent.DispatchOptions{
					PRNumber:   opts.PRNumber,
					StagingDir: b.StagingDir,
					OutputDir:  skillOut,
					Mode:       agent.ModeFiles,
					ScriptsDir: opts.ScriptsDir,
					Timeout:    opts.BatchTimeout,
				})
			},
		})
		if lastErr == nil {
			break
		}
		if attempt < 2 && isTimeout(lastErr) {
			agent.Logf("WARN", "%s: timed out — retrying once after 60s", label)
			time.Sleep(60 * time.Second)
			continue
		}
		break
	}
	if lastErr != nil {
		return fmt.Errorf("%s failed: %w", label, lastErr)
	}
	if err := TouchCheckpoint(cp); err != nil {
		return fmt.Errorf("%s checkpoint: %w", label, err)
	}
	agent.Logf("INFO", "%s: done", label)
	return nil
}

func runFinalizeAndProse(opts Options, skills []string) error {
	for _, skill := range skills {
		if IsSynthesisSkill(skill) {
			continue
		}
		skillOut := filepath.Join(opts.OutputDir, skill)
		finalizeCP := filepath.Join(skillOut, "logs", "finalize.done.txt")
		if FileExists(finalizeCP) {
			agent.Logf("INFO", "%s finalize: checkpoint — skipping", skill)
		} else {
			stagingSkill := filepath.Join(opts.StagingDir, skill)
			if err := opts.Dispatch(agent.DispatchOptions{
				PRNumber: opts.PRNumber, StagingDir: stagingSkill,
				OutputDir: skillOut, Mode: agent.ModeFinalize, ScriptsDir: opts.ScriptsDir,
			}); err != nil {
				return fmt.Errorf("finalize %s: %w", skill, err)
			}
			// Require summary.md + index.md like groovy
			if !FileExists(filepath.Join(skillOut, "summary.md")) || !FileExists(filepath.Join(skillOut, "index.md")) {
				agent.Logf("WARN", "%s finalize: summary.md or index.md missing", skill)
			}
			if err := TouchCheckpoint(finalizeCP); err != nil {
				return fmt.Errorf("finalize %s checkpoint: %w", skill, err)
			}
		}

		proseCP := filepath.Join(skillOut, "logs", "prose.done.txt")
		if FileExists(proseCP) {
			agent.Logf("INFO", "%s prose: checkpoint — skipping", skill)
			continue
		}
		stagingSkill := filepath.Join(opts.StagingDir, skill)
		if err := opts.Dispatch(agent.DispatchOptions{
			PRNumber: opts.PRNumber, StagingDir: stagingSkill,
			OutputDir: skillOut, Mode: agent.ModeProse, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("prose %s: %w", skill, err)
		}
		if err := TouchCheckpoint(proseCP); err != nil {
			return fmt.Errorf("prose %s checkpoint: %w", skill, err)
		}
	}
	return nil
}

func runSynthesis(opts Options, batches []BatchEntry) error {
	if len(batches) == 0 {
		return nil
	}
	agent.Logf("INFO", "Phase 2 — running %d synthesis batch(es)", len(batches))
	var wg sync.WaitGroup
	errCh := make(chan error, len(batches))
	for _, batch := range batches {
		b := batch
		wg.Add(1)
		go func() {
			defer wg.Done()
			skillOut := filepath.Join(opts.OutputDir, b.Skill)
			cp := CheckpointPath(skillOut, b.BatchNum)
			label := b.Skill + " / batch_" + b.BatchNum
			_ = os.MkdirAll(filepath.Join(skillOut, "logs"), 0o755)
			if FileExists(cp) {
				agent.Logf("INFO", "%s: checkpoint — skipping", label)
				return
			}
			var err error
			switch b.Skill {
			case "pr-review-summary":
				err = opts.RunSummary(agent.SummaryLoopOptions{
					PRNumber: opts.PRNumber, StagingDir: b.StagingDir,
					OutputDir: skillOut, ScriptsDir: opts.ScriptsDir,
					Dispatch: opts.Dispatch,
				})
			case "pr-review-technical":
				err = opts.RunTech(agent.TechLoopOptions{
					PRNumber: opts.PRNumber, StagingDir: b.StagingDir,
					OutputDir: skillOut, ScriptsDir: opts.ScriptsDir,
					Dispatch: opts.Dispatch,
				})
			case "pr-review-blast-radius":
				err = opts.Dispatch(agent.DispatchOptions{
					PRNumber: opts.PRNumber, StagingDir: b.StagingDir,
					OutputDir: skillOut, Mode: agent.ModeSummary, ScriptsDir: opts.ScriptsDir,
				})
			default:
				err = fmt.Errorf("unknown synthesis skill %q (expected pr-review-summary|pr-review-technical|pr-review-blast-radius)", b.Skill)
			}
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", label, err)
				return
			}
			if err := TouchCheckpoint(cp); err != nil {
				errCh <- fmt.Errorf("%s checkpoint: %w", label, err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func runSynthesisProse(opts Options, batches []BatchEntry) error {
	for _, b := range batches {
		skillOut := filepath.Join(opts.OutputDir, b.Skill)
		cp := filepath.Join(skillOut, "logs", "prose-synthesis.done.txt")
		if FileExists(cp) {
			agent.Logf("INFO", "%s prose-synthesis: checkpoint — skipping", b.Skill)
			continue
		}
		stagingSkill := filepath.Join(opts.StagingDir, b.Skill)
		if err := opts.Dispatch(agent.DispatchOptions{
			PRNumber: opts.PRNumber, StagingDir: stagingSkill,
			OutputDir: skillOut, Mode: agent.ModeProse, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("prose-synthesis %s: %w", b.Skill, err)
		}
		if err := TouchCheckpoint(cp); err != nil {
			return fmt.Errorf("prose-synthesis %s checkpoint: %w", b.Skill, err)
		}
	}
	return nil
}

func runTechDeep(opts Options) error {
	techReview := filepath.Join(opts.OutputDir, "tech-review.md")
	if !FileExists(techReview) {
		agent.Logf("INFO", "tech-review-deep: tech-review.md not found — skipping")
		return nil
	}
	deepCP := filepath.Join(opts.OutputDir, "pr-review-technical", "logs", "deep.done.txt")
	if FileExists(deepCP) {
		agent.Logf("INFO", "tech-review-deep: checkpoint — skipping")
		return nil
	}
	deepOut := filepath.Join(opts.OutputDir, "pr-review-technical-deep")
	_ = os.MkdirAll(deepOut, 0o755)
	repo := opts.RepoRoot
	if repo == "" {
		repo, _ = os.Getwd()
	}
	if err := agent.RunTechDeep(agent.TechDeepOptions{
		PRNumber:       opts.PRNumber,
		TechReviewPath: techReview,
		WorkspaceRoot:  repo,
		StagingBase:    opts.StagingDir,
		OutputDir:      deepOut,
		ScriptsDir:     opts.ScriptsDir,
		Dispatch:       opts.Dispatch,
	}); err != nil {
		return fmt.Errorf("tech-review-deep: %w", err)
	}
	if err := TouchCheckpoint(deepCP); err != nil {
		return fmt.Errorf("tech-review-deep checkpoint: %w", err)
	}
	return nil
}

func isTimeout(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timed out")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	return os.WriteFile(dst, data, 0o644)
}
