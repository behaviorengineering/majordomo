// Package executil runs outbound commands with bounded, classified retries.
package executil

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

type result struct {
	stdout string
	stderr string
}

var breakers sync.Map

// Run executes a command under retry and circuit-breaker policies.
func Run(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("command context deadline is required")
	}

	retry := retrypolicy.NewBuilder[*result]().
		HandleIf(func(_ *result, err error) bool {
			return retryable(ctx, err)
		}).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()

	response, err := failsafe.With(retry, breakerFor(name)).
		WithContext(ctx).
		Get(func() (*result, error) {
			return runOnce(ctx, name, args, env, dir)
		})
	if response == nil {
		return "", "", err
	}
	return response.stdout, response.stderr, err
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (*result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return &result{stdout: stdout.String(), stderr: stderr.String()}, err
	}
	return &result{stdout: stdout.String(), stderr: stderr.String()}, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[*result] {
	if existing, ok := breakers.Load(name); ok {
		breaker, ok := existing.(circuitbreaker.CircuitBreaker[*result])
		if !ok {
			panic(fmt.Sprintf("unexpected breaker type for %s", name))
		}
		return breaker
	}
	created := circuitbreaker.NewBuilder[*result]().
		HandleIf(func(_ *result, err error) bool {
			return retryableError(err)
		}).
		WithFailureThreshold(3).
		WithDelay(10 * time.Second).
		WithJitterFactor(0.2).
		Build()
	actual, loaded := breakers.LoadOrStore(name, created)
	if loaded {
		breaker, ok := actual.(circuitbreaker.CircuitBreaker[*result])
		if !ok {
			panic(fmt.Sprintf("unexpected breaker type for %s", name))
		}
		return breaker
	}
	return created
}

func retryable(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	return retryableError(err)
}

func retryableError(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	switch exitErr.ExitCode() {
	case 124, 137:
		return true
	default:
		return false
	}
}
