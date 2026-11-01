// Package command runs external commands with caller-owned time budgets.
package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Options configures one external command invocation.
type Options struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

// Error preserves command stderr for retry classification and diagnostics.
type Error struct {
	Name   string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("%s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("%s: %v: %s", e.Name, e.Err, e.Stderr)
}

func (e *Error) Unwrap() error {
	return e.Err
}

var (
	breakersMu sync.Mutex
	breakers   = map[string]circuitbreaker.CircuitBreaker[any]{}
)

// Run executes an external command with bounded retry and shared circuit
// breaking. The caller must supply a non-nil context with a deadline.
func Run(ctx context.Context, opts Options) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("command context deadline is required")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return "", "", errors.New("command name is required")
	}

	retry := retrypolicy.NewBuilder[any]().
		HandleIf(func(_ any, err error) bool {
			return isRetryable(err)
		}).
		WithMaxAttempts(3).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()
	breaker := getBreaker(opts.Name)

	var stdout, stderr string
	err := failsafe.With[any](breaker, retry).WithContext(ctx).Run(func() error {
		var err error
		stdout, stderr, err = runOnce(ctx, opts)
		if err != nil {
			return err
		}
		return nil
	})
	return stdout, stderr, err
}

func runOnce(ctx context.Context, opts Options) (string, string, error) {
	cmd := exec.CommandContext(ctx, opts.Name, opts.Args...)
	cmd.Dir = opts.Dir
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), &Error{
			Name:   opts.Name,
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    err,
		}
	}
	return stdout.String(), stderr.String(), nil
}

func getBreaker(name string) circuitbreaker.CircuitBreaker[any] {
	breakersMu.Lock()
	defer breakersMu.Unlock()
	if breaker, ok := breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[any]().
		HandleIf(func(_ any, err error) bool {
			return isRetryable(err)
		}).
		WithFailureThreshold(3).
		WithSuccessThreshold(1).
		WithDelay(10 * time.Second).
		WithJitter(1 * time.Second).
		Build()
	breakers[name] = breaker
	return breaker
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	var commandErr *Error
	if !errors.As(err, &commandErr) {
		return false
	}
	text := strings.ToLower(commandErr.Stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"remote end hung up",
		"timed out",
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
