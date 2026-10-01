// Package command runs external commands with bounded, classified retries.
package command

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

// Run executes an external command only when the caller supplies a deadline.
func Run(ctx context.Context, dependency, name string, args []string, env []string, dir string) (string, string, error) {
	if ctx == nil {
		return "", "", fmt.Errorf("run %s: context is required", name)
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", fmt.Errorf("run %s: context deadline is required", name)
	}
	if dependency == "" {
		return "", "", fmt.Errorf("run %s: dependency is required", name)
	}

	retry := retrypolicy.NewBuilder[result]().
		HandleIf(func(out result, err error) bool {
			return isRetryable(ctx, out.stderr, err)
		}).
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.25).
		ReturnLastFailure().
		Build()
	breaker := breakerFor(dependency)

	out, err := failsafe.With(breaker, retry).
		WithContext(ctx).
		Get(func() (result, error) {
			return runOnce(ctx, name, args, env, dir)
		})
	if err != nil {
		return out.stdout, out.stderr, fmt.Errorf("run %s: %w", name, err)
	}
	return out.stdout, out.stderr, nil
}

func breakerFor(dependency string) circuitbreaker.CircuitBreaker[result] {
	if existing, ok := breakers.Load(dependency); ok {
		if breaker, ok := existing.(circuitbreaker.CircuitBreaker[result]); ok {
			return breaker
		}
		breakers.Delete(dependency)
	}
	created := circuitbreaker.NewBuilder[result]().
		HandleIf(func(out result, err error) bool {
			return isRetryable(context.Background(), out.stderr, err)
		}).
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()
	actual, loaded := breakers.LoadOrStore(dependency, created)
	if !loaded {
		return created
	}
	if breaker, ok := actual.(circuitbreaker.CircuitBreaker[result]); ok {
		return breaker
	}
	breakers.Store(dependency, created)
	return created
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return result{stdout: stdout.String(), stderr: stderr.String()}, err
}

func isRetryable(ctx context.Context, stderr string, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && strings.Contains(strings.ToLower(err.Error()), "signal:") {
		return true
	}
	text := strings.ToLower(stderr + " " + err.Error())
	for _, marker := range []string{
		"timeout",
		"timed out",
		"connection reset",
		"connection refused",
		"temporary failure",
		"could not resolve host",
		"network is unreachable",
		"429",
		"500",
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
