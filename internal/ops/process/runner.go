// Package process runs bounded external commands for Majordomo operations.
package process

import (
	"bytes"
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

// Result contains captured command output.
type Result struct {
	Stdout string
	Stderr string
}

// Options controls retry and circuit-breaker behavior.
type Options struct {
	// Network marks commands that reach a shared remote dependency.
	Network bool
}

var (
	// ErrMissingDeadline means the caller did not provide an execution budget.
	ErrMissingDeadline = errors.New("process: context deadline is required")
	breakers           sync.Map
)

type commandError struct {
	err       error
	stderr    string
	retryable bool
}

func (e *commandError) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return fmt.Sprintf("%v: %s", e.err, strings.TrimSpace(e.stderr))
}

func (e *commandError) Unwrap() error {
	return e.err
}

// Run executes a command with the caller's context and a bounded failsafe policy.
func Run(ctx context.Context, name string, args []string, env []string, dir string, opts Options) (Result, error) {
	if ctx == nil {
		return Result{}, ErrMissingDeadline
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, ErrMissingDeadline
	}

	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			var commandErr *commandError
			return errors.As(err, &commandErr) && commandErr.retryable
		}).
		WithMaxAttempts(3).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()

	var executor failsafe.Executor[Result]
	if opts.Network {
		executor = failsafe.With[Result](breakerFor(name), retry)
	} else {
		executor = failsafe.With[Result](retry)
	}
	return executor.WithContext(ctx).Get(func() (Result, error) {
		return runOnce(ctx, name, args, env, dir, opts.Network)
	})
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[Result] {
	if current, ok := breakers.Load(name); ok {
		return current.(circuitbreaker.CircuitBreaker[Result])
	}
	created := circuitbreaker.NewBuilder[Result]().
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		WithJitter(5 * time.Second).
		Build()
	current, loaded := breakers.LoadOrStore(name, created)
	if loaded {
		return current.(circuitbreaker.CircuitBreaker[Result])
	}
	return created
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string, network bool) (Result, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{
				Stdout: stdout.String(),
				Stderr: stderr.String(),
			}, &commandError{
				err:       err,
				stderr:    stderr.String(),
				retryable: network && transientOutput(stderr.String()) && ctx.Err() == nil,
			}
	}
	return Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, nil
}

func transientOutput(stderr string) bool {
	message := strings.ToLower(stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"timed out",
		"temporary failure",
		"remote end hung up",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
