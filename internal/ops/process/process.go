// Package process runs external commands with caller-owned deadlines and resilience policies.
package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Result contains captured command output.
type Result struct {
	Stdout string
	Stderr string
}

// Runner executes external commands with shared circuit breakers.
type Runner struct {
	breakers sync.Map
}

// NewRunner returns an isolated command runner.
func NewRunner() *Runner {
	return &Runner{}
}

var defaultRunner = NewRunner()

// Default returns the process-wide runner used by production command paths.
func Default() *Runner {
	return defaultRunner
}

// Run executes a command with retry and circuit-breaker policies.
func (r *Runner) Run(ctx context.Context, name string, args []string, env []string, dir string) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("process: context deadline is required")
	}
	if name == "" {
		return Result{}, errors.New("process: command name is required")
	}

	retryPolicy := retrypolicy.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			return retryable(err)
		}).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxAttempts(3).
		Build()
	breaker := r.breaker(name)
	executor := failsafe.With(breaker, retryPolicy).WithContext(ctx)
	result, err := executor.Get(func() (Result, error) {
		return runOnce(ctx, name, args, env, dir)
	})
	if err != nil {
		var commandErr *commandError
		if errors.As(err, &commandErr) {
			return commandErr.result, err
		}
	}
	return result, err
}

func (r *Runner) breaker(name string) circuitbreaker.CircuitBreaker[Result] {
	if value, ok := r.breakers.Load(name); ok {
		return value.(circuitbreaker.CircuitBreaker[Result])
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			return retryable(err)
		}).
		WithFailureThreshold(5).
		WithDelay(5 * time.Second).
		WithJitter(time.Second).
		Build()
	actual, _ := r.breakers.LoadOrStore(name, breaker)
	return actual.(circuitbreaker.CircuitBreaker[Result])
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		return result, &commandError{
			name:   name,
			err:    err,
			result: result,
		}
	}
	return result, nil
}

type commandError struct {
	name   string
	err    error
	result Result
}

func (e *commandError) Error() string {
	return fmt.Sprintf("run %s: %v", e.name, e.err)
}

func (e *commandError) Unwrap() error {
	return e.err
}

func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		err = commandErr.err
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled()
}
