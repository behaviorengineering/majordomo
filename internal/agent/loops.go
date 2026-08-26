package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// SummaryLoopOptions configures the summary generate/score loop.
type SummaryLoopOptions struct {
	PRNumber   string
	StagingDir string
	OutputDir  string // skill output: .../pr-review-summary
	PassScore  int
	MaxIter    int
	ScriptsDir string
	Dispatch   func(DispatchOptions) error
}

// RunSummaryLoop ports summary-loop.py.
func RunSummaryLoop(opts SummaryLoopOptions) error {
	if opts.PassScore <= 0 {
		opts.PassScore = envInt("SUMMARY_PASS_SCORE", 15)
	}
	if opts.MaxIter <= 0 {
		opts.MaxIter = envInt("SUMMARY_MAX_ITERATIONS", 5)
	}
	dispatch := opts.Dispatch
	if dispatch == nil {
		dispatch = Dispatch
	}
	pipelineOut := filepath.Dir(opts.OutputDir)
	scoreFile := filepath.Join(pipelineOut, "score.md")
	logsDir := filepath.Join(opts.OutputDir, "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	Logf("INFO", "========== Summary loop: PR #%s (pass>=%d, max=%d) ==========",
		opts.PRNumber, opts.PassScore, opts.MaxIter)
	defer func() { _ = os.Unsetenv("SUMMARY_ITER") }()

	for iteration := 1; iteration <= opts.MaxIter; iteration++ {
		Logf("INFO", "[summary-loop] Iteration %d/%d", iteration, opts.MaxIter)
		if err := os.Setenv("SUMMARY_ITER", strconv.Itoa(iteration)); err != nil {
			return fmt.Errorf("summary-loop: set SUMMARY_ITER: %w", err)
		}

		if err := dispatch(DispatchOptions{
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeSummary, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("summary-loop: --summary failed: %w", err)
		}
		if err := dispatch(DispatchOptions{
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeScore, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("summary-loop: --score failed: %w", err)
		}

		data, err := os.ReadFile(scoreFile)
		if err != nil {
			return fmt.Errorf("summary-loop: score.md not found: %w", err)
		}
		score, ok := ParseScore(string(data))
		if !ok {
			return fmt.Errorf("summary-loop: could not parse SCORE from score.md")
		}
		Logf("INFO", "[summary-loop] Score: %d (threshold %d)", score, opts.PassScore)

		// Archive iteration artefacts
		summarySrc := filepath.Join(pipelineOut, "summary.md")
		_ = copyIfExists(summarySrc, filepath.Join(logsDir, fmt.Sprintf("summary_iter_%d.md", iteration)))
		_ = copyIfExists(scoreFile, filepath.Join(logsDir, fmt.Sprintf("score_iter_%d.md", iteration)))

		if score >= opts.PassScore {
			Logf("INFO", "[summary-loop] Accepted score %d", score)
			return nil
		}
		feedback := filepath.Join(opts.StagingDir, "score_feedback.md")
		if err := copyFile(scoreFile, feedback); err != nil {
			return err
		}
	}
	Logf("INFO", "[summary-loop] Reached max iterations; keeping last summary")
	return nil
}

// TechLoopOptions configures the technical review loop.
type TechLoopOptions struct {
	PRNumber   string
	StagingDir string
	OutputDir  string
	PassScore  int
	MaxIter    int
	ScriptsDir string
	Dispatch   func(DispatchOptions) error
}

// RunTechLoop ports tech-review-loop.py.
func RunTechLoop(opts TechLoopOptions) error {
	if opts.PassScore <= 0 {
		opts.PassScore = envInt("TECH_PASS_SCORE", 11)
	}
	if opts.MaxIter <= 0 {
		opts.MaxIter = envInt("TECH_MAX_ITERATIONS", 3)
	}
	dispatch := opts.Dispatch
	if dispatch == nil {
		dispatch = Dispatch
	}
	pipelineOut := filepath.Dir(opts.OutputDir)
	scoreFile := filepath.Join(pipelineOut, "tech-score.md")
	logsDir := filepath.Join(opts.OutputDir, "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	Logf("INFO", "========== Tech loop: PR #%s (pass>=%d, max=%d) ==========",
		opts.PRNumber, opts.PassScore, opts.MaxIter)
	defer func() { _ = os.Unsetenv("TECH_ITER") }()

	for iteration := 1; iteration <= opts.MaxIter; iteration++ {
		Logf("INFO", "[tech-loop] Iteration %d/%d", iteration, opts.MaxIter)
		if err := os.Setenv("TECH_ITER", strconv.Itoa(iteration)); err != nil {
			return fmt.Errorf("tech-loop: set TECH_ITER: %w", err)
		}

		if err := dispatch(DispatchOptions{
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeTechnical, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("tech-loop: --technical failed: %w", err)
		}
		if err := dispatch(DispatchOptions{
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeTechScore, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("tech-loop: --tech-score failed: %w", err)
		}

		data, err := os.ReadFile(scoreFile)
		if err != nil {
			return fmt.Errorf("tech-loop: tech-score.md not found: %w", err)
		}
		score, ok := ParseScore(string(data))
		if !ok {
			return fmt.Errorf("tech-loop: could not parse SCORE from tech-score.md")
		}
		Logf("INFO", "[tech-loop] Score: %d (threshold %d)", score, opts.PassScore)

		_ = copyIfExists(filepath.Join(pipelineOut, "tech-review.md"),
			filepath.Join(logsDir, fmt.Sprintf("tech_review_iter_%d.md", iteration)))
		_ = copyIfExists(scoreFile, filepath.Join(logsDir, fmt.Sprintf("tech_score_iter_%d.md", iteration)))

		if score >= opts.PassScore {
			Logf("INFO", "[tech-loop] Accepted score %d", score)
			return nil
		}
		feedback := filepath.Join(opts.StagingDir, "tech_feedback.md")
		if err := copyFile(scoreFile, feedback); err != nil {
			return err
		}
	}
	Logf("INFO", "[tech-loop] Reached max iterations; keeping last tech-review")
	return nil
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
	return os.WriteFile(dst, data, 0o644)
}

func copyIfExists(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return copyFile(src, dst)
}
