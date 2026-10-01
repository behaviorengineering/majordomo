// Package process runs outbound child processes with caller-owned deadlines.
package process

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

var breakers sync.Map

// Run executes fn with a shared circuit breaker and a bounded retry policy.
// A nil retryable function disables retries and the circuit breaker.
func Run(ctx context.Context, dependency string, retryable func(error) bool, fn func() error) error {
	if ctx == nil {
		return errors.New("process context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("process context deadline is required")
	}
	if fn == nil {
		return errors.New("process function is required")
	}
	if retryable == nil {
		return fn()
	}

	breaker := breakerFor(dependency)
	retry := retrypolicy.NewBuilder[any]().
		HandleIf(func(_ any, err error) bool {
			return retryable(err)
		}).
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.2).
		ReturnLastFailure().
		Build()
	return failsafe.With[any](breaker, retry).
		WithContext(ctx).
		Run(fn)
}

func breakerFor(dependency string) circuitbreaker.CircuitBreaker[any] {
	if dependency == "" {
		dependency = "unknown"
	}
	if existing, ok := breakers.Load(dependency); ok {
		return existing.(circuitbreaker.CircuitBreaker[any])
	}
	created := circuitbreaker.NewBuilder[any]().
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()
	actual, loaded := breakers.LoadOrStore(dependency, created)
	if loaded {
		return actual.(circuitbreaker.CircuitBreaker[any])
	}
	return created
}

// Retryable reports whether an execution failure is likely transient.
func Retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 75, 124, 137:
			return true
		}
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"timed out",
		"timeout",
		" 429",
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
