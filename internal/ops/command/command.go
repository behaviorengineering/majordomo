// Package command runs external processes with bounded, resilient execution.
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
	err    error
}

var (
	breakerMu sync.Mutex
	breakers  = map[string]circuitbreaker.CircuitBreaker[result]{}
)

// Run executes a process under a caller deadline and resilient retry policy.
func Run(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("command context deadline is required")
	}
	if strings.TrimSpace(name) == "" {
		return "", "", errors.New("command name is required")
	}

	retry := retrypolicy.NewBuilder[result]().
		HandleIf(isRetryable).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()
	executor := failsafe.With[result](breakerFor(name), retry).WithContext(ctx)
	out, err := executor.GetWithExecution(func(exec failsafe.Execution[result]) (result, error) {
		return runOnce(exec.Context(), name, args, env, dir)
	})
	if err != nil {
		return out.stdout, out.stderr, fmt.Errorf("run %s: %w", name, err)
	}
	return out.stdout, out.stderr, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[result] {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	if breaker, ok := breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[result]().
		HandleIf(isRetryable).
		WithFailureThreshold(3).
		WithSuccessThreshold(2).
		WithDelay(30 * time.Second).
		Build()
	breakers[name] = breaker
	return breaker
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return result{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}, err
}

func isRetryable(out result, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	text := strings.ToLower(out.stderr + " " + err.Error())
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporary failure",
		"timed out",
		"timeout",
		"tls handshake",
		"remote end hung up",
		"429",
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
