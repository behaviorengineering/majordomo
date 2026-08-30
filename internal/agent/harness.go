package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
)

// OpenCodeEnv returns os.Environ (or opts.Env) rewritten for an OpenCode child:
// gateway Ensure, dummy OpenAI key, loopback OPENAI_BASE_URL, real keys stripped.
func OpenCodeEnv(parent []string) ([]string, error) {
	if parent == nil {
		parent = os.Environ()
	}
	return aigateway.PrepareChildEnv(parent)
}

// RunOpenCode shells out to agent-dispatch.sh (or copilot-dispatch.sh) with
// Bifrost ChildEnv. Review orchestrate stays on in-process strop Judge;
// use this for legacy OpenCode harness runs only.
func RunOpenCode(opts DispatchOptions) error {
	if opts.PRNumber == "" || opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("opencode harness requires pr, staging-dir, and output-dir")
	}
	scriptsDir, err := ResolveScriptsDir(opts.ScriptsDir)
	if err != nil {
		return err
	}
	script, ok := dispatchScriptIn(scriptsDir)
	if !ok {
		return fmt.Errorf("opencode harness: %s not found in %s", dispatchScriptPrimary, scriptsDir)
	}

	parent := opts.Env
	if parent == nil {
		parent = os.Environ()
	}
	env, err := OpenCodeEnv(parent)
	if err != nil {
		return fmt.Errorf("opencode harness env: %w", err)
	}

	args := []string{opts.PRNumber, opts.StagingDir, opts.OutputDir}
	if opts.Mode != "" && opts.Mode != ModeFiles {
		args = append(args, string(opts.Mode))
	}

	runner := opts.Runner
	if runner == nil {
		runner = defaultScriptRunner(opts.Timeout)
	}
	return runner(script, args, env, "")
}

func defaultScriptRunner(timeout time.Duration) func(name string, args []string, env []string, dir string) error {
	return func(name string, args []string, env []string, dir string) error {
		cmd := exec.Command(name, args...)
		cmd.Env = env
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if timeout > 0 {
			timer := time.AfterFunc(timeout, func() {
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
			})
			defer timer.Stop()
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opencode harness %s: %w", filepath.Base(name), err)
		}
		return nil
	}
}
