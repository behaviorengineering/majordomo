package dispatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/behaviorengineering/majordomo/pkg/platform/aigateway"
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
		if opts.Context == nil {
			return fmt.Errorf("opencode harness: context is required")
		}
		if _, ok := opts.Context.Deadline(); !ok {
			return fmt.Errorf("opencode harness: context deadline is required")
		}
		ctx := opts.Context
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}
		runner = defaultScriptRunner(ctx)
	}
	return runner(script, args, env, "")
}

func defaultScriptRunner(ctx context.Context) func(name string, args []string, env []string, dir string) error {
	return func(name string, args []string, env []string, dir string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opencode harness %s: %w", filepath.Base(name), err)
		}
		return nil
	}
}
