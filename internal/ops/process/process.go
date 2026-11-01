// Package process runs external commands with cancellation and retry policies.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

type commandError struct {
	name   string
	err    error
	stderr string
}

func (e *commandError) Error() string {
	if e.stderr == "" {
		return fmt.Sprintf("%s: %v", e.name, e.err)
	}
	return fmt.Sprintf("%s: %v: %s", e.name, e.err, e.stderr)
}

func (e *commandError) Unwrap() error {
	return e.err
}

var breakers sync.Map

// Run executes a command under shared retry and circuit-breaker policies.
func Run(ctx context.Context, name string, args []string, env []string, dir string, stdout, stderr io.Writer) error {
	if ctx == nil {
		return errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("process: context deadline is required")
	}

	retries := retrypolicy.NewBuilder[any]().
		WithMaxRetries(2).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		HandleIf(func(_ any, err error) bool {
			return isRetryable(err)
		}).
		ReturnLastFailure().
		Build()
	breaker := breakerFor(name)

	return failsafe.With(breaker, retries).
		WithContext(ctx).
		Run(func() error {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Dir = dir
			cmd.Env = env
			cmd.Stdout = stdout
			var captured bytes.Buffer
			if stderr == nil {
				cmd.Stderr = &captured
			} else {
				cmd.Stderr = io.MultiWriter(stderr, &captured)
			}
			if err := cmd.Run(); err != nil {
				return &commandError{name: name, err: err, stderr: strings.TrimSpace(captured.String())}
			}
			return nil
		})
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[any] {
	if value, ok := breakers.Load(name); ok {
		breaker, ok := value.(circuitbreaker.CircuitBreaker[any])
		if !ok {
			panic("process: invalid circuit breaker type")
		}
		return breaker
	}
	created := circuitbreaker.NewBuilder[any]().
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()
	actual := any(created)
	if value, loaded := breakers.LoadOrStore(name, created); loaded {
		actual = value
	}
	breaker, ok := actual.(circuitbreaker.CircuitBreaker[any])
	if !ok {
		panic("process: invalid circuit breaker type")
	}
	return breaker
}

func isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var commandErr *commandError
	if !errors.As(err, &commandErr) {
		return false
	}
	message := strings.ToLower(commandErr.Error())
	for _, marker := range []string{
		"timeout",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"rate limit",
		"429",
		"502",
		"503",
		"504",
		"killed",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
