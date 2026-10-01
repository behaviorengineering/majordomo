// Package process runs outbound commands with caller-owned budgets and resilience policies.
package process

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"

	failsafe "github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Output contains one command attempt's captured output.
type Output struct {
	Stdout string
	Stderr string
}

// Runner executes commands with a breaker per dependency key.
type Runner struct {
	mu       sync.Mutex
	breakers map[string]circuitbreaker.CircuitBreaker[Output]
}

// NewRunner creates a command runner with isolated breaker state.
func NewRunner() *Runner {
	return &Runner{breakers: make(map[string]circuitbreaker.CircuitBreaker[Output])}
}

// Run executes a command under retry and circuit-breaker policies.
func (r *Runner) Run(ctx context.Context, dependency, name string, args []string, env []string, dir string) (Output, error) {
	if ctx == nil {
		return Output{}, errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Output{}, errors.New("process: context deadline is required")
	}
	if r == nil {
		return Output{}, errors.New("process: runner is required")
	}

	retryPolicy := retrypolicy.NewBuilder[Output]().
		HandleIf(func(_ Output, err error) bool {
			return isTransient(err)
		}).
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()

	breaker := r.breaker(dependency)
	return failsafe.With(retryPolicy, breaker).
		WithContext(ctx).
		Get(func() (Output, error) {
			return runOnce(ctx, name, args, env, dir)
		})
}

func (r *Runner) breaker(dependency string) circuitbreaker.CircuitBreaker[Output] {
	r.mu.Lock()
	defer r.mu.Unlock()
	if breaker, ok := r.breakers[dependency]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[Output]().
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()
	r.breakers[dependency] = breaker
	return breaker
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (Output, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	var stdout, stderr captureBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Output{Stdout: stdout.String(), Stderr: stderr.String()}, fmt.Errorf("%s: %w", name, err)
	}
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func isTransient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		return ok && status.Signaled()
	}
	return false
}

type captureBuffer struct {
	data []byte
}

func (b *captureBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *captureBuffer) String() string {
	return string(b.data)
}
