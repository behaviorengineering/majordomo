// Package execx runs external commands with cancellation and transient-failure
// resilience.
package execx

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

// Result contains captured command output.
type Result struct {
	Stdout string
	Stderr string
}

var (
	breakerMu sync.Mutex
	breakers  = map[string]circuitbreaker.CircuitBreaker[Result]{}
)

// Run executes a command. Network-backed commands require a caller deadline.
func Run(ctx context.Context, name string, args []string, env []string, dir string, networked bool) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("execute command: context is required")
	}
	if networked {
		if _, ok := ctx.Deadline(); !ok {
			return "", "", errors.New("execute command: deadline is required for network operation")
		}
	}

	retryPolicy := retrypolicy.NewBuilder[Result]().
		HandleIf(isRetryable).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()
	result, err := failsafe.With(retryPolicy, breakerFor(name)).GetWithExecution(
		func(execution failsafe.Execution[Result]) (Result, error) {
			cmd := exec.CommandContext(execution.Context(), name, args...)
			if env != nil {
				cmd.Env = env
			}
			if dir != "" {
				cmd.Dir = dir
			}
			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()
			return Result{Stdout: stdout.String(), Stderr: stderr.String()}, runErr
		},
	)
	if err != nil {
		return result.Stdout, result.Stderr, fmt.Errorf("run %s: %w", name, err)
	}
	return result.Stdout, result.Stderr, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[Result] {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	if breaker, ok := breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		HandleIf(isRetryable).
		WithFailureThreshold(3).
		WithDelay(10 * time.Second).
		Build()
	breakers[name] = breaker
	return breaker
}

func isRetryable(result Result, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	text := strings.ToLower(result.Stderr + " " + err.Error())
	for _, marker := range []string{
		"timeout",
		"timed out",
		"connection reset",
		"connection refused",
		"could not resolve",
		"temporarily unavailable",
		"signal: killed",
		" 429",
		" 500",
		" 502",
		" 503",
		" 504",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
