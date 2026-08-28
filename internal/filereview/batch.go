package filereview

import (
	"fmt"
	"os"
	"path/filepath"
)

// JudgeFunc runs the Judge step (today: OpenCode ModeFiles via agent.Dispatch).
type JudgeFunc func() error

// Options configures one file-review batch state machine.
type Options struct {
	StagingDir string // batch staging (contains manifest.json)
	SkillOut   string // <output>/<skill>
	MaxRetries int    // Validate failures; default 2 (3 attempts total with first)
	Judge      JudgeFunc
	Logf       func(format string, args ...any)
}

// Run executes Prepare → Judge → Validate → Assemble with retries on Validate failure.
func Run(opts Options) error {
	if opts.Judge == nil {
		return fmt.Errorf("filereview: Judge required")
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 2
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	manifest := filepath.Join(opts.StagingDir, "manifest.json")
	reviewables, err := LoadReviewables(manifest)
	if err != nil {
		return err
	}
	logf("filereview prepare: %d reviewable(s)", len(reviewables))

	perFile := PerFileDir(opts.SkillOut)
	attempts := opts.MaxRetries + 1
	var lastValidate error
	for attempt := 1; attempt <= attempts; attempt++ {
		logf("filereview judge: attempt %d/%d", attempt, attempts)
		if err := opts.Judge(); err != nil {
			return fmt.Errorf("filereview judge: %w", err)
		}
		reports, err := CollectReports(perFile, reviewables)
		if err != nil {
			lastValidate = err
			logf("filereview validate: %v", err)
			if err := writeFeedback(opts.StagingDir, err); err != nil {
				return err
			}
			continue
		}
		if err := ValidateReports(reviewables, reports); err != nil {
			lastValidate = err
			logf("filereview validate: %v", err)
			if err := writeFeedback(opts.StagingDir, err); err != nil {
				return err
			}
			continue
		}
		if err := Assemble(opts.SkillOut, reports, reviewables); err != nil {
			return err
		}
		logf("filereview assemble: ok (%d report(s))", len(reports))
		return nil
	}
	return fmt.Errorf("filereview: exhausted retries: %w", lastValidate)
}

func writeFeedback(stagingDir string, validateErr error) error {
	path := filepath.Join(stagingDir, "filereview_feedback.md")
	body := "# File-review Validate feedback\n\n" + validateErr.Error() + "\n"
	return os.WriteFile(path, []byte(body), 0o644)
}
