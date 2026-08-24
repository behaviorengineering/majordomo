package staging

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	MaxCombinedLines      = 500
	MaxDiffLines          = 300
	GitTimeout            = 30 * time.Second
	MaxStageFilenameBytes = 240

	ModeFullAndDiff = "full_and_diff"
	ModeDiffOnly    = "diff_only"
	ModeDiffChunk   = "diff_chunk"

	CrossSkillBatchDir = "batch_000"
	CrossSkillBatchNum = "000"
)

// ErrNothingToReview is returned when exit code 2 should be used.
var ErrNothingToReview = errors.New("nothing to review")

// ErrFatal is a fatal prep error (exit 1).
type ErrFatal struct {
	Msg string
}

func (e *ErrFatal) Error() string { return e.Msg }

func fatalf(format string, args ...any) error {
	return &ErrFatal{Msg: fmt.Sprintf(format, args...)}
}

// Options configures a prep run.
type Options struct {
	BaseBranch        string
	StagingDir        string
	RoutingPath       string
	AgentContextPath  string
	SummaryConfigPath string
	RepoRoot          string // empty = cwd
	BatchSize         int
}

func batchSizeFromEnv() int {
	if v := os.Getenv("MAJORDOMO_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if v := os.Getenv("COPILOT_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 15
}

func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
