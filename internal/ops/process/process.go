// Package process runs external commands with cancellation and resilience.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	failsafe "github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Result contains one command attempt's output.
type Result struct {
	Stdout string
	Stderr string
}

// Runner executes external commands with shared circuit breakers.
type Runner struct {
	mu       sync.Mutex
	breakers map[string]circuitbreaker.CircuitBreaker[Result]
}

// NewRunner creates a process runner.
func NewRunner() *Runner {
	return &Runner{breakers: make(map[string]circuitbreaker.CircuitBreaker[Result])}
}

var defaultRunner = NewRunner()

// DefaultRunner returns the process runner shared by production commands.
func DefaultRunner() *Runner {
	return defaultRunner
}

// Run executes a command with a caller-provided deadline.
func (r *Runner) Run(ctx context.Context, key, name string, args []string, env []string, dir string) (Result, error) {
	if r == nil {
		return Result{}, errors.New("process runner is required")
	}
	if ctx == nil {
		return Result{}, errors.New("process context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, fmt.Errorf("process %s: deadline is required", key)
	}
	if key == "" {
		return Result{}, errors.New("process key is required")
	}
	if name == "" {
		return Result{}, errors.New("process name is required")
	}

	retryPolicy := retrypolicy.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			return retryable(ctx, err)
		}).
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()

	result, err := failsafe.With(r.breaker(key), retryPolicy).
		WithContext(ctx).
		Get(func() (Result, error) {
			return runOnce(ctx, name, args, env, dir)
		})
	if err != nil {
		return Result{}, fmt.Errorf("run %s: %w", key, err)
	}
	return result, nil
}

func (r *Runner) breaker(key string) circuitbreaker.CircuitBreaker[Result] {
	r.mu.Lock()
	defer r.mu.Unlock()

	if breaker, ok := r.breakers[key]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		WithFailureThreshold(3).
		WithDelay(30 * time.Second).
		WithJitter(1 * time.Second).
		Build()
	r.breakers[key] = breaker
	return breaker
}

func runOnce(ctx context.Context, name string, args, env []string, dir string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Result{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func retryable(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == -1 || exitErr.ExitCode() == 137
	}
	return false
}
