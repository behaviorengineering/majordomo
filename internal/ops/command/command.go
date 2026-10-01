// Package command runs external processes with bounded, classified resilience.
package command

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Runner executes external commands for one operation.
type Runner struct {
	retryPolicy retrypolicy.RetryPolicy[result]
	breaker     circuitbreaker.CircuitBreaker[result]
}

type result struct {
	stdout string
	stderr string
}

// NewRunner creates a runner with bounded retries and a circuit breaker.
func NewRunner() *Runner {
	retryPolicy := retrypolicy.NewBuilder[result]().
		HandleIf(retryable).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()
	breaker := circuitbreaker.NewBuilder[result]().
		HandleIf(retryable).
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		WithJitter(500 * time.Millisecond).
		Build()
	return &Runner{retryPolicy: retryPolicy, breaker: breaker}
}

// Run executes name with the caller's deadline and returns captured output.
func (r *Runner) Run(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	if r == nil {
		return "", "", errors.New("command runner is required")
	}
	if ctx == nil {
		return "", "", errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("command context deadline is required")
	}
	if name == "" {
		return "", "", errors.New("command name is required")
	}

	out, err := failsafe.With(r.retryPolicy, r.breaker).
		WithContext(ctx).
		Get(func() (result, error) {
			if err := ctx.Err(); err != nil {
				return result{}, err
			}
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Env = env
			cmd.Dir = dir
			stdout, stderr, err := run(cmd)
			if err != nil {
				return result{stdout: stdout, stderr: stderr}, fmt.Errorf("%s: %w", name, err)
			}
			return result{stdout: stdout, stderr: stderr}, nil
		})
	if err != nil {
		return out.stdout, out.stderr, err
	}
	return out.stdout, out.stderr, nil
}

func run(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func retryable(_ result, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	message := strings.ToLower(exitErr.Error())
	return strings.Contains(message, "killed") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "temporarily unavailable")
}
