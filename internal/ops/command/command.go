// Package command runs external processes with cancellation and resilience.
package command

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

// Request describes one external process invocation.
type Request struct {
	Name       string
	Args       []string
	Dir        string
	Env        []string
	Dependency string
	Retryable  func(error) bool
}

type result struct {
	stdout string
	stderr string
}

type executionError struct {
	err    error
	stderr string
}

func (e *executionError) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return fmt.Sprintf("%v: %s", e.err, e.stderr)
}

func (e *executionError) Unwrap() error {
	return e.err
}

// Runner executes external processes using per-dependency circuit breakers.
type Runner struct {
	mu       sync.Mutex
	breakers map[string]circuitbreaker.CircuitBreaker[result]
}

// NewRunner creates a process runner.
func NewRunner() *Runner {
	return &Runner{breakers: make(map[string]circuitbreaker.CircuitBreaker[result])}
}

// Run executes a process after validating the caller's context deadline.
func (r *Runner) Run(ctx context.Context, req Request) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", fmt.Errorf("command %q requires a context deadline", req.Name)
	}
	if strings.TrimSpace(req.Name) == "" {
		return "", "", errors.New("command name is required")
	}

	retryable := req.Retryable
	if retryable == nil {
		retryable = defaultRetryable
	}
	run := func() (result, error) {
		cmd := exec.CommandContext(ctx, req.Name, req.Args...)
		cmd.Dir = req.Dir
		if req.Env != nil {
			cmd.Env = req.Env
		}
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return result{stdout: stdout.String(), stderr: stderr.String()}, &executionError{
				err:    err,
				stderr: strings.TrimSpace(stderr.String()),
			}
		}
		return result{stdout: stdout.String(), stderr: stderr.String()}, nil
	}

	retry := retrypolicy.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool {
			return ctx.Err() == nil && retryable(err)
		}).
		WithBackoff(100*time.Millisecond, time.Second).
		WithJitterFactor(0.2).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()
	dependency := req.Dependency
	if dependency == "" {
		dependency = req.Name
	}
	breaker := r.breaker(dependency, retryable)
	out, err := failsafe.With(breaker, retry).WithContext(ctx).Get(run)
	return out.stdout, out.stderr, err
}

func (r *Runner) breaker(dependency string, retryable func(error) bool) circuitbreaker.CircuitBreaker[result] {
	r.mu.Lock()
	defer r.mu.Unlock()
	if breaker, ok := r.breakers[dependency]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool {
			return retryable(err)
		}).
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		Build()
	r.breakers[dependency] = breaker
	return breaker
}

func defaultRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok {
			return status.Signaled() && (status.Signal() == syscall.SIGKILL || status.Signal() == syscall.SIGTERM)
		}
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"timed out",
		"502",
		"503",
		"429",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
