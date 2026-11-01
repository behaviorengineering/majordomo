// Package execx runs outbound processes with shared cancellation and resilience policies.
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

// Options configures one process execution.
type Options struct {
	Env []string
	Dir string
}

type result struct {
	stdout string
	stderr string
}

// CommandError preserves the process identity and captured stderr for callers.
type CommandError struct {
	Name   string
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("%s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("%s: %v: %s", e.Name, e.Err, e.Stderr)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

var breakers = struct {
	sync.Mutex
	values map[string]circuitbreaker.CircuitBreaker[result]
}{
	values: make(map[string]circuitbreaker.CircuitBreaker[result]),
}

// Run executes a process with classified transient retries and a shared circuit breaker.
func Run(ctx context.Context, name string, args []string, opts Options) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("process execution context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("process execution requires a deadline")
	}

	retry := retrypolicy.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool { return isTransient(err) }).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitterFactor(0.2).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()

	breaker := breakerFor(name)
	out, err := failsafe.With(breaker, retry).
		WithContext(ctx).
		Get(func() (result, error) {
			return runOnce(ctx, name, args, opts)
		})
	if err != nil {
		return out.stdout, out.stderr, fmt.Errorf("run %s: %w", name, err)
	}
	return out.stdout, out.stderr, nil
}

func runOnce(ctx context.Context, name string, args []string, opts Options) (result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = opts.Env
	cmd.Dir = opts.Dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := result{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		return out, &CommandError{
			Name:   name,
			Args:   append([]string(nil), args...),
			Stderr: strings.TrimSpace(out.stderr),
			Err:    err,
		}
	}
	return out, nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[result] {
	breakers.Lock()
	defer breakers.Unlock()
	if breaker, ok := breakers.values[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool { return isTransient(err) }).
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		Build()
	breakers.values[name] = breaker
	return breaker
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		return false
	}
	text := strings.ToLower(commandErr.Stderr)
	for _, marker := range []string{
		"timeout",
		"timed out",
		"connection reset",
		"connection refused",
		"temporarily unavailable",
		"could not resolve host",
		"network is unreachable",
		"502",
		"503",
		"504",
		"429",
		"killed",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
