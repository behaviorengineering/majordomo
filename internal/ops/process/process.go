// Package process runs bounded external commands with shared resilience policy.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	failsafe "github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Spec describes an external command invocation.
type Spec struct {
	Name string
	Args []string
	Env  []string
	Dir  string
}

// Runner executes external commands. Execute is injectable for unit tests.
type Runner struct {
	Execute func(context.Context, Spec) (stdout, stderr string, err error)
}

var breakers sync.Map

// Run executes a command with a caller deadline, retry policy, and circuit breaker.
func (r Runner) Run(ctx context.Context, spec Spec) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("process: context deadline is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return "", "", errors.New("process: command name is required")
	}

	execute := r.Execute
	if execute == nil {
		execute = runDefault
	}

	breaker := breakerFor(spec.Name)
	retry := retrypolicy.NewBuilder[any]().
		WithMaxRetries(2).
		WithBackoff(250*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		HandleIf(func(_ any, err error) bool {
			return isRetryable(err)
		}).
		Build()

	var stdout, stderr string
	err := failsafe.With[any](breaker, retry).
		WithContext(ctx).
		Run(func() error {
			var err error
			stdout, stderr, err = execute(ctx, spec)
			return err
		})
	if err != nil {
		return stdout, stderr, fmt.Errorf("%s: %w", spec.Name, err)
	}
	return stdout, stderr, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[any] {
	if value, ok := breakers.Load(name); ok {
		return value.(circuitbreaker.CircuitBreaker[any])
	}
	created := circuitbreaker.NewBuilder[any]().
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()
	actual, _ := breakers.LoadOrStore(name, created)
	return actual.(circuitbreaker.CircuitBreaker[any])
}

func runDefault(ctx context.Context, spec Spec) (string, string, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			err = fmt.Errorf("%w: %s", err, message)
		}
	}
	return stdout.String(), stderr.String(), err
}

func isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return status.Signal() == syscall.SIGKILL || status.Signal() == syscall.SIGTERM
		}
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"connection reset", "connection refused", "network is unreachable", "timed out", "temporary failure", "tls handshake timeout", "signal: killed"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
