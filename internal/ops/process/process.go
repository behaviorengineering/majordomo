// Package process runs outbound commands with shared cancellation and
// resilience policies.
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

// Options configures one command execution.
type Options struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

// Result contains the command output.
type Result struct {
	Stdout string
	Stderr string
}

// CommandError preserves the command cause and captured standard error.
type CommandError struct {
	Name   string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	message := fmt.Sprintf("%s: %v", e.Name, e.Err)
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		message += ": " + stderr
	}
	return message
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// Runner executes commands and shares circuit state by command name.
type Runner struct {
	breakers sync.Map
}

// NewRunner creates a command runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Run executes a command with cancellation, retry, and circuit-breaker
// policies. The caller must provide a context with a deadline.
func (r *Runner) Run(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("process: context deadline is required")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return Result{}, errors.New("process: command name is required")
	}

	breaker := r.breaker(opts.Name)
	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			return isRetryable(err)
		}).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()

	return failsafe.With[Result](breaker, retry).
		WithContext(ctx).
		Get(func() (Result, error) {
			return runOnce(ctx, opts)
		})
}

var defaultRunner = NewRunner()

// Run executes a command with the package default runner.
func Run(ctx context.Context, opts Options) (Result, error) {
	return defaultRunner.Run(ctx, opts)
}

func (r *Runner) breaker(name string) circuitbreaker.CircuitBreaker[Result] {
	if breakerValue, ok := r.breakers.Load(name); ok {
		if breaker, ok := breakerValue.(circuitbreaker.CircuitBreaker[Result]); ok {
			return breaker
		}
	}
	candidate := circuitbreaker.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			return isRetryable(err)
		}).
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		WithJitter(5 * time.Second).
		Build()
	actual, loaded := r.breakers.LoadOrStore(name, candidate)
	if loaded {
		if breaker, ok := actual.(circuitbreaker.CircuitBreaker[Result]); ok {
			return breaker
		}
	}
	return candidate
}

func runOnce(ctx context.Context, opts Options) (Result, error) {
	cmd := exec.CommandContext(ctx, opts.Name, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = opts.Env

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		return result, &CommandError{Name: opts.Name, Stderr: result.Stderr, Err: err}
	}
	return result, nil
}

func isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(commandErr.Err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return true
		}
	}

	message := strings.ToLower(commandErr.Stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"network is unreachable",
		"temporary failure",
		"temporarily unavailable",
		"tls handshake timeout",
		"i/o timeout",
		"timeout",
		" 429",
		" 500",
		" 502",
		" 503",
		" 504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
