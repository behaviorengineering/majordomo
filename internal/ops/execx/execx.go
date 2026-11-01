// Package execx runs external commands with bounded, classified retries.
package execx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Result contains the output from a successful command attempt.
type Result struct {
	Stdout string
	Stderr string
}

// CommandError preserves command output while retaining the process error.
type CommandError struct {
	Name   string
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("%s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("%s: %v: %s", e.Name, e.Err, e.Stderr)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// Runner executes commands through a circuit breaker keyed by dependency.
type Runner struct {
	mu       sync.Mutex
	breakers map[string]circuitbreaker.CircuitBreaker[Result]
}

// NewRunner creates a command runner with isolated dependency breakers.
func NewRunner() *Runner {
	return &Runner{breakers: make(map[string]circuitbreaker.CircuitBreaker[Result])}
}

// Run executes a command after requiring a caller-provided deadline.
func (r *Runner) Run(ctx context.Context, dependency, name string, args, env []string, dir string) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("execx: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("execx: context deadline is required")
	}
	if dependency == "" {
		return Result{}, errors.New("execx: dependency is required")
	}
	if name == "" {
		return Result{}, errors.New("execx: command name is required")
	}

	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool { return isTransient(err) }).
		WithBackoff(100*time.Millisecond, time.Second).
		WithJitterFactor(0.2).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()

	return failsafe.With(r.breaker(dependency), retry).
		WithContext(ctx).
		Get(func() (Result, error) {
			return runOnce(ctx, name, args, env, dir)
		})
}

func (r *Runner) breaker(dependency string) circuitbreaker.CircuitBreaker[Result] {
	r.mu.Lock()
	defer r.mu.Unlock()
	if breaker, ok := r.breakers[dependency]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool { return isTransient(err) }).
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		Build()
	r.breakers[dependency] = breaker
	return breaker
}

func runOnce(ctx context.Context, name string, args, env []string, dir string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, &CommandError{
			Name:   name,
			Args:   append([]string(nil), args...),
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    err,
		}
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func isTransient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(commandErr.Err, &exitErr) && exitErr.ExitCode() == -1 {
		return true
	}
	text := strings.ToLower(commandErr.Stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"timed out",
		"timeout",
		"try again",
		"502",
		"503",
		"504",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
