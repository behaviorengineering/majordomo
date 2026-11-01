// Package process runs external commands with bounded, resilient execution.
package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

const (
	retryDelay       = 100 * time.Millisecond
	retryMaxDelay    = 2 * time.Second
	circuitDelay     = 10 * time.Second
	failureThreshold = 5
)

// Result contains the captured output from one external command.
type Result struct {
	Stdout string
	Stderr string
}

var breakers sync.Map

// Run executes an external command with the caller's deadline and context.
func Run(ctx context.Context, name string, args []string, env []string, dir string) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("process: context deadline is required")
	}

	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			return retryable(err)
		}).
		WithBackoff(retryDelay, retryMaxDelay).
		WithJitter(retryDelay).
		ReturnLastFailure().
		Build()

	breaker := breakerFor(name)
	result, err := failsafe.With(retry, breaker).
		WithContext(ctx).
		Get(func() (Result, error) {
			return runOnce(ctx, name, args, env, dir)
		})
	if err != nil {
		return result, fmt.Errorf("run %q: %w", name, err)
	}
	return result, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[Result] {
	if value, ok := breakers.Load(name); ok {
		return value.(circuitbreaker.CircuitBreaker[Result])
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		HandleIf(func(_ Result, err error) bool {
			return retryable(err)
		}).
		WithFailureThreshold(failureThreshold).
		WithDelay(circuitDelay).
		Build()
	actual, loaded := breakers.LoadOrStore(name, breaker)
	if loaded {
		return actual.(circuitbreaker.CircuitBreaker[Result])
	}
	return breaker
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append([]string(nil), env...)
	cmd.Dir = dir
	var stdout, stderr syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{Stdout: stdout.String(), Stderr: stderr.String()}, ctxErr
		}
		return Result{Stdout: stdout.String(), Stderr: stderr.String()}, err
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 137, 143:
			return true
		}
	}
	return false
}

type syncBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}
