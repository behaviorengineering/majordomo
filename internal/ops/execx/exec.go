// Package execx runs external commands with bounded, resilient execution.
package execx

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

const (
	retryDelay    = 100 * time.Millisecond
	maxRetryDelay = 2 * time.Second
	circuitDelay  = 5 * time.Second
)

var breakers = struct {
	sync.Mutex
	values map[string]circuitbreaker.CircuitBreaker[struct{}]
}{
	values: make(map[string]circuitbreaker.CircuitBreaker[struct{}]),
}

// Run executes name with args in dir and returns combined output.
func Run(ctx context.Context, name string, args []string, dir string, env []string) (string, error) {
	if ctx == nil {
		return "", errors.New("external command requires a context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", errors.New("external command requires a deadline")
	}

	retryPolicy := retrypolicy.NewBuilder[struct{}]().
		HandleIf(func(_ struct{}, err error) bool {
			return retryable(err)
		}).
		WithBackoff(retryDelay, maxRetryDelay).
		WithJitterFactor(0.25).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()

	breaker := breakerFor(name)
	var output string
	err := failsafe.With[struct{}](breaker, retryPolicy).
		WithContext(ctx).
		RunWithExecution(func(execution failsafe.Execution[struct{}]) error {
			var stdout, stderr bytes.Buffer
			cmd := exec.CommandContext(execution.Context(), name, args...)
			cmd.Dir = dir
			cmd.Env = env
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()
			output = strings.TrimSpace(stdout.String() + stderr.String())
			if runErr != nil {
				return &commandError{
					name:   name,
					cause:  runErr,
					stderr: strings.TrimSpace(stderr.String()),
				}
			}
			return nil
		})
	if err != nil {
		return output, fmt.Errorf("run %s: %w", name, err)
	}
	return output, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[struct{}] {
	breakers.Lock()
	defer breakers.Unlock()
	if breaker, ok := breakers.values[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[struct{}]().
		HandleIf(func(_ struct{}, err error) bool {
			return retryable(err)
		}).
		WithFailureThreshold(5).
		WithDelay(circuitDelay).
		Build()
	breakers.values[name] = breaker
	return breaker
}

func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var commandErr *commandError
	if !errors.As(err, &commandErr) {
		return false
	}
	text := strings.ToLower(commandErr.Error())
	for _, marker := range []string{
		"connection",
		"network",
		"timed out",
		"timeout",
		"temporarily unavailable",
		"could not resolve",
		"remote",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

type commandError struct {
	name   string
	cause  error
	stderr string
}

func (e *commandError) Error() string {
	if e.stderr == "" {
		return fmt.Sprintf("%s: %v", e.name, e.cause)
	}
	return fmt.Sprintf("%s: %v: %s", e.name, e.cause, e.stderr)
}

func (e *commandError) Unwrap() error {
	return e.cause
}
