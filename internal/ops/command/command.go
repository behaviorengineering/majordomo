// Package command runs outbound commands with bounded resilience policies.
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

type result struct {
	stdout string
	stderr string
}

// Run executes an outbound command with retry and circuit-breaker policies.
func Run(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("command context must have a deadline")
	}

	shouldRetry := func(output result, err error) bool {
		return isRetryable(output.stderr, err)
	}
	retry := retrypolicy.NewBuilder[result]().
		HandleIf(shouldRetry).
		ReturnLastFailure().
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.2).
		Build()
	breaker := circuitbreaker.NewBuilder[result]().
		HandleIf(shouldRetry).
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()

	output, err := failsafe.With(retry, breaker).
		WithContext(ctx).
		Get(func() (result, error) {
			return runOnce(ctx, name, args, env, dir)
		})
	return output.stdout, output.stderr, err
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir

	var output result
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output.stdout = stdout.String()
	output.stderr = stderr.String()
	if err != nil {
		return output, fmt.Errorf("run %s: %w", name, err)
	}
	return output, nil
}

func isRetryable(stderr string, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(stderr + " " + err.Error())
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"timed out",
		"timeout",
		"rate limit",
		" 429",
		" 500",
		" 502",
		" 503",
		" 504",
		"signal: killed",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
