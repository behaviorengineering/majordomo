// Package executil runs external commands with bounded, classified retries.
package executil

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

// Request describes one external command invocation.
type Request struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type commandPolicies struct {
	retry   retrypolicy.RetryPolicy[struct{}]
	breaker circuitbreaker.CircuitBreaker[struct{}]
}

// Runner executes commands with a policy set per dependency name.
type Runner struct {
	mu       sync.Mutex
	policies map[string]commandPolicies
}

// NewRunner creates a command runner with isolated dependency policies.
func NewRunner() *Runner {
	return &Runner{policies: make(map[string]commandPolicies)}
}

// Run executes a command and retries only classified transient failures.
func (r *Runner) Run(ctx context.Context, request Request) error {
	if r == nil {
		return fmt.Errorf("command runner is required")
	}
	if ctx == nil {
		return fmt.Errorf("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return fmt.Errorf("command context deadline is required")
	}
	if strings.TrimSpace(request.Name) == "" {
		return fmt.Errorf("command name is required")
	}
	policies := r.policiesFor(request.Name)
	return failsafe.With(policies.retry, policies.breaker).
		WithContext(ctx).
		Run(func() error {
			return runOnce(ctx, request)
		})
}

func (r *Runner) policiesFor(name string) commandPolicies {
	r.mu.Lock()
	defer r.mu.Unlock()
	if policies, ok := r.policies[name]; ok {
		return policies
	}
	isTransient := func(_ struct{}, err error) bool {
		return isRetryable(err)
	}
	policies := commandPolicies{
		retry: retrypolicy.NewBuilder[struct{}]().
			HandleIf(isTransient).
			WithMaxRetries(2).
			WithBackoff(100*time.Millisecond, 2*time.Second).
			WithJitter(100 * time.Millisecond).
			ReturnLastFailure().
			Build(),
		breaker: circuitbreaker.NewBuilder[struct{}]().
			HandleIf(isTransient).
			WithFailureThreshold(3).
			WithDelay(10 * time.Second).
			WithJitter(500 * time.Millisecond).
			Build(),
	}
	r.policies[name] = policies
	return policies
}

type commandError struct {
	err    error
	stderr string
}

func (e *commandError) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return fmt.Sprintf("%v: %s", e.err, e.stderr)
}

func (e *commandError) Unwrap() error {
	return e.err
}

func runOnce(ctx context.Context, request Request) error {
	cmd := exec.CommandContext(ctx, request.Name, request.Args...)
	cmd.Dir = request.Dir
	cmd.Env = request.Env
	var stderr bytes.Buffer
	if request.Stdout == nil {
		request.Stdout = io.Discard
	}
	if request.Stderr == nil {
		request.Stderr = io.Discard
	}
	cmd.Stdout = request.Stdout
	cmd.Stderr = io.MultiWriter(request.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &commandError{err: err, stderr: strings.TrimSpace(stderr.String())}
	}
	return nil
}

func isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"timeout",
		"timed out",
		"connection reset",
		"connection refused",
		"temporary failure",
		"could not resolve host",
		"network is unreachable",
		"unexpected eof",
		"signal: killed",
		" 429",
		" 502",
		" 503",
		" 504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
