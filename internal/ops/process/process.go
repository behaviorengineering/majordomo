// Package process runs external commands with cancellation and resilience.
package process

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

var (
	// ErrContextRequired indicates that command execution was attempted without a context.
	ErrContextRequired = errors.New("process context is required")
	// ErrDeadlineRequired indicates that command execution was attempted without a deadline.
	ErrDeadlineRequired = errors.New("process context deadline is required")
)

type commandResult struct {
	stdout string
	stderr string
}

var breakers sync.Map

// Run executes a command with a caller-provided deadline. Transient network
// failures are retried with backoff and jitter, and repeated transient
// failures open a breaker for the command name.
func Run(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	if ctx == nil {
		return "", "", ErrContextRequired
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", ErrDeadlineRequired
	}

	breaker := breakerFor(name)
	retry := retrypolicy.NewBuilder[commandResult]().
		HandleIf(func(_ commandResult, err error) bool {
			return isTransient(err)
		}).
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.2).
		ReturnLastFailure().
		Build()

	result, err := failsafe.With[commandResult](breaker, retry).
		WithContext(ctx).
		Get(func() (commandResult, error) {
			return runOnce(ctx, name, args, env, dir)
		})
	if err != nil {
		return "", "", fmt.Errorf("run %s: %w", name, err)
	}
	return result.stdout, result.stderr, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[commandResult] {
	if breaker, ok := breakers.Load(name); ok {
		if typed, ok := breaker.(circuitbreaker.CircuitBreaker[commandResult]); ok {
			return typed
		}
		breakers.Delete(name)
	}
	breaker := circuitbreaker.NewBuilder[commandResult]().
		HandleIf(func(_ commandResult, err error) bool {
			return isTransient(err)
		}).
		WithFailureThreshold(5).
		WithDelay(5 * time.Second).
		Build()
	actual, _ := breakers.LoadOrStore(name, breaker)
	typed, ok := actual.(circuitbreaker.CircuitBreaker[commandResult])
	if !ok {
		return breaker
	}
	return typed
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (commandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return commandResult{stdout: stdout.String(), stderr: stderr.String()},
			fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return commandResult{stdout: stdout.String(), stderr: stderr.String()}, nil
}

func isTransient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"network is unreachable",
		"temporary failure",
		"timed out",
		"timeout",
		"signal: killed",
		"429",
		"502",
		"503",
		"504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
