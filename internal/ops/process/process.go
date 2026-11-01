// Package process runs external commands with cancellation and resilience policies.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

var ErrMissingDeadline = errors.New("process: context deadline is required")

type Command struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

type Result struct {
	Stdout string
	Stderr string
}

type CommandRunner interface {
	Run(context.Context, Command) (Result, error)
}

type Runner struct {
	mu       sync.Mutex
	breakers map[string]circuitbreaker.CircuitBreaker[any]
}

func NewRunner() *Runner {
	return &Runner{breakers: make(map[string]circuitbreaker.CircuitBreaker[any])}
}

var defaultRunner = NewRunner()

func DefaultRunner() CommandRunner {
	return defaultRunner
}

func (r *Runner) Run(ctx context.Context, command Command) (Result, error) {
	if ctx == nil {
		return Result{}, ErrMissingDeadline
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, ErrMissingDeadline
	}
	if command.Name == "" {
		return Result{}, errors.New("process: command name is required")
	}

	dependency := command.Name
	breaker := r.breaker(dependency)
	retry := retrypolicy.NewBuilder[any]().
		HandleIf(func(_ any, err error) bool {
			return isRetryable(err)
		}).
		WithMaxRetries(2).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.25).
		ReturnLastFailure().
		Build()

	var result Result
	err := failsafe.With(retry, breaker).WithContext(ctx).Run(func() error {
		var runErr error
		result, runErr = runOnce(ctx, command)
		return runErr
	})
	if err != nil {
		return result, fmt.Errorf("%s: %w", command.Name, err)
	}
	return result, nil
}

func (r *Runner) breaker(dependency string) circuitbreaker.CircuitBreaker[any] {
	r.mu.Lock()
	defer r.mu.Unlock()
	if breaker, ok := r.breakers[dependency]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[any]().
		HandleIf(func(_ any, err error) bool {
			return isRetryable(err)
		}).
		WithFailureThreshold(5).
		WithDelay(5 * time.Second).
		WithJitterFactor(0.25).
		Build()
	r.breakers[dependency] = breaker
	return breaker
}

func runOnce(ctx context.Context, command Command) (Result, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = command.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Result{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		return ok && status.Signaled()
	}
	return false
}
