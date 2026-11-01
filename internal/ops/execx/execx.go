// Package execx runs outbound processes with shared cancellation and resilience policy.
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	osExec "os/exec"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

var (
	breakers sync.Map
	retries  = retrypolicy.NewBuilder[any]().
			HandleIf(retryable).
			WithMaxRetries(2).
			WithBackoff(100*time.Millisecond, 2*time.Second).
			WithJitter(100 * time.Millisecond).
			ReturnLastFailure().
			Build()
)

// Run executes an outbound process under a dependency-scoped retry and breaker policy.
func Run(ctx context.Context, dependency, name string, args, env []string, dir string) (stdout, stderr string, err error) {
	if ctx == nil {
		return "", "", errors.New("execx: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("execx: context deadline is required")
	}
	if err := ctx.Err(); err != nil {
		return "", "", fmt.Errorf("execx: context is unavailable: %w", err)
	}

	breaker := breakerFor(dependency)
	var lastStdout, lastStderr string
	err = failsafe.With(retries, breaker).
		WithContext(ctx).
		RunWithExecution(func(execution failsafe.Execution[any]) error {
			if err := execution.Context().Err(); err != nil {
				return err
			}

			var out bytes.Buffer
			var stderrBuffer bytes.Buffer
			cmd := osExec.CommandContext(execution.Context(), name, args...)
			cmd.Env = env
			cmd.Dir = dir
			cmd.Stdout = &out
			cmd.Stderr = &stderrBuffer
			runErr := cmd.Run()
			lastStdout = out.String()
			lastStderr = stderrBuffer.String()
			if runErr != nil {
				if contextErr := execution.Context().Err(); contextErr != nil {
					return contextErr
				}
				return fmt.Errorf("%s: %w", dependency, runErr)
			}
			return nil
		})
	return lastStdout, lastStderr, err
}

func breakerFor(dependency string) circuitbreaker.CircuitBreaker[any] {
	if existing, ok := breakers.Load(dependency); ok {
		if breaker, ok := existing.(circuitbreaker.CircuitBreaker[any]); ok {
			return breaker
		}
	}

	breaker := circuitbreaker.NewBuilder[any]().
		HandleIf(retryable).
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		Build()
	if existing, loaded := breakers.LoadOrStore(dependency, breaker); loaded {
		if existingBreaker, ok := existing.(circuitbreaker.CircuitBreaker[any]); ok {
			return existingBreaker
		}
	}
	return breaker
}

func retryable(_ any, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var networkErr net.Error
	if errors.As(err, &networkErr) {
		return networkErr.Timeout() || networkErr.Temporary()
	}

	var exitErr *osExec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 124, 137, 143:
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "signal: killed") || strings.Contains(message, "timed out")
}
