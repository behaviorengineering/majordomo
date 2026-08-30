package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/judge"
)

// Mode selects Judge step behaviour (strop DispatchMode).
type Mode string

const (
	ModeFiles         Mode = ""
	ModeFinalize      Mode = "--finalize"
	ModeSummary       Mode = "--summary"
	ModeScore         Mode = "--score"
	ModeTechnical     Mode = "--technical"
	ModeTechScore     Mode = "--tech-score"
	ModeProse         Mode = "--prose"
	ModeTechnicalDeep Mode = "--technical-deep"
)

const (
	dispatchScriptPrimary = "agent-dispatch.sh"
	dispatchScriptLegacy  = "copilot-dispatch.sh"
)

// DispatchOptions configures a single Judge invocation.
type DispatchOptions struct {
	PRNumber   string
	StagingDir string
	OutputDir  string
	Mode       Mode
	// ScriptsDir is retained for ResolveScriptsDir callers (tech-deep helpers); unused by strop Judge.
	ScriptsDir string
	// Env is the parent environ for RunOpenCode (nil → os.Environ). Unused by Dispatch.
	Env []string
	// Timeout kills the OpenCode script process when > 0. Unused by in-process Judge.
	Timeout time.Duration
	// Runner overrides script exec for RunOpenCode tests. Unused by Dispatch.
	Runner func(name string, args []string, env []string, dir string) error
}

func dispatchScriptIn(dir string) (string, bool) {
	for _, name := range []string{dispatchScriptPrimary, dispatchScriptLegacy} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// ResolveScriptsDir finds pipelines/scripts (legacy helper paths / SA scripts).
func ResolveScriptsDir(explicit string) (string, error) {
	if explicit != "" {
		if _, ok := dispatchScriptIn(explicit); ok {
			return filepath.Clean(explicit), nil
		}
		// Allow directory even without dispatch script (scripts still hold helpers).
		if info, err := os.Stat(explicit); err == nil && info.IsDir() {
			return filepath.Clean(explicit), nil
		}
		return "", fmt.Errorf("scripts dir not found: %s", explicit)
	}
	if v := os.Getenv("MAJORDOMO_SCRIPTS"); v != "" {
		if info, err := os.Stat(v); err == nil && info.IsDir() {
			return filepath.Clean(v), nil
		}
	}
	candidates := []string{
		"pipelines/scripts",
		".majordomo/pipelines/scripts",
	}
	wd, _ := os.Getwd()
	dir := wd
	for i := 0; i < 8 && dir != ""; i++ {
		for _, rel := range candidates {
			cand := filepath.Join(dir, rel)
			if info, err := os.Stat(cand); err == nil && info.IsDir() {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("pipelines/scripts not found (set --scripts-dir or MAJORDOMO_SCRIPTS)")
}

// Dispatch runs the in-process strop Judge (never OpenCode).
func Dispatch(opts DispatchOptions) error {
	if opts.PRNumber == "" || opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("dispatch requires pr, staging-dir, and output-dir")
	}
	return judge.Dispatch(judge.DispatchOptions{
		PRNumber:   opts.PRNumber,
		StagingDir: opts.StagingDir,
		OutputDir:  opts.OutputDir,
		Mode:       judge.DispatchMode(opts.Mode),
	})
}

// FindScript returns path to a named script under scripts dir.
func FindScript(scriptsDir, name string) (string, error) {
	dir, err := ResolveScriptsDir(scriptsDir)
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%s not found in %s", name, dir)
	}
	return p, nil
}

// Logf prints a timestamped orchestrator/agent log line.
func Logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}

// ParseScore extracts SCORE: N from score markdown text.
func ParseScore(text string) (int, bool) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SCORE:") {
			var n int
			_, err := fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "SCORE:")), "%d", &n)
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}
