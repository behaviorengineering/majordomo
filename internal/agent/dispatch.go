package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Mode selects agent-dispatch.sh behaviour.
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

// DispatchOptions configures a single agent invocation.
type DispatchOptions struct {
	PRNumber   string
	StagingDir string
	OutputDir  string
	Mode       Mode
	// ScriptsDir is the pipelines/scripts directory containing agent-dispatch.sh
	// (or the copilot-dispatch.sh shim). Empty → MAJORDOMO_SCRIPTS / walk up from CWD.
	ScriptsDir string
	// Env extra environment variables (merged onto os.Environ).
	Env []string
	// Timeout caps the subprocess; 0 = no timeout.
	Timeout time.Duration
	// Runner overrides command execution (tests).
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

// ResolveScriptsDir finds pipelines/scripts containing agent-dispatch.sh.
func ResolveScriptsDir(explicit string) (string, error) {
	if explicit != "" {
		if _, ok := dispatchScriptIn(explicit); ok {
			return filepath.Clean(explicit), nil
		}
		return "", fmt.Errorf("%s not found in %s", dispatchScriptPrimary, explicit)
	}
	if v := os.Getenv("MAJORDOMO_SCRIPTS"); v != "" {
		if _, ok := dispatchScriptIn(v); ok {
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
			if _, ok := dispatchScriptIn(cand); ok {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("%s not found (set --scripts-dir or MAJORDOMO_SCRIPTS)", dispatchScriptPrimary)
}

// Dispatch runs agent-dispatch.sh (OpenCode) with the given options.
func Dispatch(opts DispatchOptions) error {
	if opts.PRNumber == "" || opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("dispatch requires pr, staging-dir, and output-dir")
	}
	scripts, err := ResolveScriptsDir(opts.ScriptsDir)
	if err != nil {
		return err
	}
	script, ok := dispatchScriptIn(scripts)
	if !ok {
		return fmt.Errorf("%s not found in %s", dispatchScriptPrimary, scripts)
	}
	args := []string{script, opts.PRNumber, opts.StagingDir, opts.OutputDir}
	if opts.Mode != ModeFiles {
		args = append(args, string(opts.Mode))
	}

	env := append([]string{}, os.Environ()...)
	env = append(env, opts.Env...)

	runner := opts.Runner
	if runner == nil {
		runner = defaultRunner(opts.Timeout)
	}
	return runner("bash", args, env, "")
}

func defaultRunner(timeout time.Duration) func(name string, args []string, env []string, dir string) error {
	return func(name string, args []string, env []string, dir string) error {
		cmd := exec.Command(name, args...)
		cmd.Env = env
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if timeout <= 0 {
			return cmd.Run()
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Run() }()
		select {
		case err := <-done:
			return err
		case <-time.After(timeout):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			// Wait for the runner goroutine so we do not leak it after Kill.
			<-done
			return fmt.Errorf("dispatch timed out after %s", timeout)
		}
	}
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
