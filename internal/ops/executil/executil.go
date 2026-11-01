package executil

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Result contains one external command attempt's captured output.
type Result struct {
	Stdout string
	Stderr string
}

// Options configures one external command execution.
type Options struct {
	Context   context.Context
	Name      string
	Args      []string
	Env       []string
	Dir       string
	Networked bool
}

// Run executes an external command with bounded, classified retry behavior.
func Run(opts Options) (string, string, error) {
	if opts.Context == nil {
		return "", "", fmt.Errorf("execute %s: context is required", opts.Name)
	}
	if opts.Name == "" {
		return "", "", fmt.Errorf("execute: command name is required")
	}
	if opts.Networked {
		if _, ok := opts.Context.Deadline(); !ok {
			return "", "", fmt.Errorf("execute %s: deadline is required for networked command", opts.Name)
		}
	}

	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(result Result, err error) bool {
			return isRetryable(result, err)
		}).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()

	policies := []failsafe.Policy[Result]{retry}
	if opts.Networked {
		breaker := circuitbreaker.NewBuilder[Result]().
			HandleIf(func(result Result, err error) bool {
				return isRetryable(result, err)
			}).
			WithFailureThreshold(3).
			WithDelay(5 * time.Second).
			Build()
		policies = append(policies, breaker)
	}

	result, err := failsafe.With(policies...).WithContext(opts.Context).GetWithExecution(
		func(execution failsafe.Execution[Result]) (Result, error) {
			cmd := exec.CommandContext(execution.Context(), opts.Name, opts.Args...)
			cmd.Env = opts.Env
			cmd.Dir = opts.Dir
			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			return Result{Stdout: stdout.String(), Stderr: stderr.String()}, err
		},
	)
	if err != nil {
		return result.Stdout, result.Stderr, fmt.Errorf("execute %s: %w", opts.Name, err)
	}
	return result.Stdout, result.Stderr, nil
}

func isRetryable(result Result, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return true
		}
	}

	text := strings.ToLower(result.Stdout + "\n" + result.Stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"timed out",
		"timeout",
		" 429 ",
		" 500 ",
		" 502 ",
		" 503 ",
		" 504 ",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
