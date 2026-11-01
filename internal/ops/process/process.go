// Package process runs external commands with bounded, resilient execution.
package process

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Runner executes external commands with per-dependency circuit breakers.
type Runner struct {
	mu       sync.Mutex
	breakers map[string]circuitbreaker.CircuitBreaker[any]
}

// NewRunner creates an external process runner.
func NewRunner() *Runner {
	return &Runner{breakers: make(map[string]circuitbreaker.CircuitBreaker[any])}
}

// Run executes a command with a caller-supplied deadline.
func (r *Runner) Run(
	ctx context.Context,
	dependency, name string,
	args []string,
	env []string,
	dir string,
	stdout, stderr io.Writer,
) error {
	if r == nil {
		return errors.New("process runner is required")
	}
	if ctx == nil {
		return errors.New("process context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("process context deadline is required")
	}
	if dependency == "" {
		return errors.New("process dependency is required")
	}

	retry := retrypolicy.NewBuilder[any]().
		HandleIf(func(_ any, err error) bool { return retryable(err) }).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(50 * time.Millisecond).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()
	breaker := r.breaker(dependency)

	return failsafe.With(breaker, retry).
		WithContext(ctx).
		Run(func() error {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Env = env
			cmd.Dir = dir
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			return cmd.Run()
		})
}

func (r *Runner) breaker(dependency string) circuitbreaker.CircuitBreaker[any] {
	r.mu.Lock()
	defer r.mu.Unlock()
	if breaker, ok := r.breakers[dependency]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[any]().
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		WithJitter(time.Second).
		Build()
	r.breakers[dependency] = breaker
	return breaker
}

func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		status, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
		return ok && status.Signaled()
	}
	return os.IsTimeout(err)
}
