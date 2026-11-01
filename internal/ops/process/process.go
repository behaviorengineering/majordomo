// Package process runs outbound commands with cancellation and resilience.
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

type result struct {
	stdout string
	stderr string
	err    error
}

var breakers sync.Map

// Run executes a command under a shared retry and circuit-breaker policy.
func Run(ctx context.Context, dependency, name string, args []string, dir string, env []string) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("process: context deadline is required")
	}
	if err := ctx.Err(); err != nil {
		return "", "", fmt.Errorf("process: context is unusable: %w", err)
	}

	retry := retrypolicy.NewBuilder[result]().
		HandleIf(isRetryable).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.25).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()
	breaker := breakerFor(dependency)

	final, err := failsafe.With(retry).
		Compose(breaker).
		WithContext(ctx).
		Get(func() (result, error) {
			return runOnce(ctx, name, args, dir, env)
		})
	if err != nil {
		return final.stdout, final.stderr, fmt.Errorf("run %s: %w", name, err)
	}
	return final.stdout, final.stderr, final.err
}

func breakerFor(dependency string) circuitbreaker.CircuitBreaker[result] {
	if existing, ok := breakers.Load(dependency); ok {
		breaker, ok := existing.(circuitbreaker.CircuitBreaker[result])
		if !ok {
			panic("process: invalid circuit breaker type")
		}
		return breaker
	}
	created := circuitbreaker.NewBuilder[result]().
		HandleIf(isRetryable).
		WithFailureThreshold(3).
		WithDelay(10 * time.Second).
		Build()
	actual, loaded := breakers.LoadOrStore(dependency, created)
	if !loaded {
		return created
	}
	breaker, ok := actual.(circuitbreaker.CircuitBreaker[result])
	if !ok {
		panic("process: invalid circuit breaker type")
	}
	return breaker
}

func runOnce(ctx context.Context, name string, args []string, dir string, env []string) (result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	current := result{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
	if err != nil {
		return current, err
	}
	return current, nil
}

func isRetryable(current result, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	output := strings.ToLower(current.stderr + "\n" + current.stdout)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporary failure",
		"timed out",
		"timeout",
		"502",
		"503",
		"504",
		"429",
		"signal: killed",
	} {
		if strings.Contains(output, marker) {
			return true
		}
	}
	return false
}
