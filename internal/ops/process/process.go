// Package process runs outbound commands with cancellation and resilience.
package process

import (
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

// Runner executes one command attempt.
type Runner func(context.Context, string, []string, []string, string) (stdout, stderr string, err error)

// Executor applies retry and circuit-breaker policies to command execution.
type Executor struct {
	runner   Runner
	mu       sync.Mutex
	breakers map[string]circuitbreaker.CircuitBreaker[commandResult]
}

// NewExecutor creates an Executor. A nil runner uses os/exec.
func NewExecutor(runner Runner) *Executor {
	if runner == nil {
		runner = runCommand
	}
	return &Executor{
		runner:   runner,
		breakers: make(map[string]circuitbreaker.CircuitBreaker[commandResult]),
	}
}

// Run executes a command with a caller-supplied deadline.
func (e *Executor) Run(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("process: context deadline is required")
	}
	if strings.TrimSpace(name) == "" {
		return "", "", errors.New("process: command name is required")
	}

	retry := retrypolicy.NewBuilder[commandResult]().
		HandleIf(func(result commandResult, err error) bool {
			return isTransient(result, err)
		}).
		WithMaxRetries(2).
		WithBackoff(250*time.Millisecond, 2*time.Second).
		WithJitter(250 * time.Millisecond).
		ReturnLastFailure().
		Build()
	breaker := e.breakerFor(name)
	result, err := failsafe.With(retry, breaker).WithContext(ctx).Get(func() (commandResult, error) {
		stdout, stderr, runErr := e.runner(ctx, name, args, env, dir)
		if runErr != nil {
			return commandResult{stdout: stdout, stderr: stderr}, &commandError{
				name:   name,
				stderr: stderr,
				err:    runErr,
			}
		}
		return commandResult{stdout: stdout, stderr: stderr}, nil
	})
	return result.stdout, result.stderr, err
}

type commandResult struct {
	stdout string
	stderr string
}

type commandError struct {
	name   string
	stderr string
	err    error
}

func (e *commandError) Error() string {
	if strings.TrimSpace(e.stderr) == "" {
		return fmt.Sprintf("%s: %v", e.name, e.err)
	}
	return fmt.Sprintf("%s: %v: %s", e.name, e.err, strings.TrimSpace(e.stderr))
}

func (e *commandError) Unwrap() error {
	return e.err
}

func (e *Executor) breakerFor(name string) circuitbreaker.CircuitBreaker[commandResult] {
	e.mu.Lock()
	defer e.mu.Unlock()
	if breaker, ok := e.breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[commandResult]().
		HandleIf(func(result commandResult, err error) bool {
			return isTransient(result, err)
		}).
		WithFailureThreshold(3).
		WithDelay(30 * time.Second).
		Build()
	e.breakers[name] = breaker
	return breaker
}

func isTransient(result commandResult, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var commandErr *commandError
	if !errors.As(err, &commandErr) {
		return false
	}
	text := strings.ToLower(commandErr.Error() + " " + result.stderr)
	for _, marker := range []string{
		"timeout",
		"timed out",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"could not resolve host",
		"status 429",
		"status 502",
		"status 503",
		"status 504",
		"signal: killed",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func runCommand(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
