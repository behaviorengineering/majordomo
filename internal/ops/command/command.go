// Package command runs outbound processes with cancellation and resilience.
package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

type result struct {
	stdout string
	stderr string
}

var breakers sync.Map

// Run executes a process with a caller-provided deadline.
func Run(ctx context.Context, dependency, name string, args []string, dir string, env []string) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("command: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("command: context deadline is required")
	}

	policy := retryPolicy(ctx)
	breaker := breakerFor(dependency)
	out, err := failsafe.With(breaker, policy).
		WithContext(ctx).
		Get(func() (result, error) {
			return runOnce(ctx, name, args, dir, env)
		})
	if err != nil {
		return out.stdout, out.stderr, fmt.Errorf("%s: %w", dependency, err)
	}
	return out.stdout, out.stderr, nil
}

func retryPolicy(ctx context.Context) retrypolicy.RetryPolicy[result] {
	return retrypolicy.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool {
			return ctx.Err() == nil && retryable(err)
		}).
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.25).
		ReturnLastFailure().
		Build()
}

func breakerFor(dependency string) circuitbreaker.CircuitBreaker[result] {
	if breaker, ok := breakers.Load(dependency); ok {
		return asBreaker(breaker)
	}
	created := circuitbreaker.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool {
			return retryable(err)
		}).
		WithFailureThreshold(5).
		WithDelay(5 * time.Second).
		Build()
	actual, loaded := breakers.LoadOrStore(dependency, created)
	if loaded {
		return asBreaker(actual)
	}
	return created
}

func asBreaker(value any) circuitbreaker.CircuitBreaker[result] {
	breaker, ok := value.(circuitbreaker.CircuitBreaker[result])
	if !ok {
		panic("command: invalid circuit breaker type")
	}
	return breaker
}

func retryable(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ProcessState != nil && exitErr.ExitCode() < 0
	}
	return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED)
}

func runOnce(ctx context.Context, name string, args []string, dir string, env []string) (result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return result{stdout: stdout.String(), stderr: stderr.String()}, err
	}
	return result{stdout: stdout.String(), stderr: stderr.String()}, nil
}
