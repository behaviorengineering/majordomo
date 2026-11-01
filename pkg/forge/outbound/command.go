package outbound

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

// CommandResult contains the captured output of an external command.
type CommandResult struct {
	Stdout string
	Stderr string
}

type commandError struct {
	name   string
	cause  error
	stderr string
}

func (e *commandError) Error() string {
	if e.stderr == "" {
		return fmt.Sprintf("run %s: %v", e.name, e.cause)
	}
	return fmt.Sprintf("run %s: %v: %s", e.name, e.cause, e.stderr)
}

func (e *commandError) Unwrap() error {
	return e.cause
}

var commandBreakers sync.Map

// RunCommand executes an external command with a caller deadline and shared
// retry and circuit-breaker policies.
func RunCommand(ctx context.Context, name string, args []string, env []string, dir string) (CommandResult, error) {
	if err := RequireDeadline(ctx); err != nil {
		return CommandResult{}, err
	}
	if strings.TrimSpace(name) == "" {
		return CommandResult{}, errors.New("outbound: command name is required")
	}

	retryPolicy := retrypolicy.NewBuilder[CommandResult]().
		HandleIf(isRetryableCommandFailure).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxAttempts(3).
		ReturnLastFailure().
		Build()
	breaker := commandBreaker(name)

	return failsafe.With(retryPolicy, breaker).
		WithContext(ctx).
		GetWithExecution(func(execution failsafe.Execution[CommandResult]) (CommandResult, error) {
			cmd := exec.CommandContext(execution.Context(), name, args...)
			cmd.Env = env
			cmd.Dir = dir
			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			result := CommandResult{
				Stdout: stdout.String(),
				Stderr: stderr.String(),
			}
			if err != nil {
				return result, &commandError{name: name, cause: err, stderr: result.Stderr}
			}
			return result, nil
		})
}

func commandBreaker(name string) circuitbreaker.CircuitBreaker[CommandResult] {
	if existing, ok := commandBreakers.Load(name); ok {
		breaker, ok := existing.(circuitbreaker.CircuitBreaker[CommandResult])
		if !ok {
			panic("outbound: invalid command circuit breaker")
		}
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[CommandResult]().
		HandleIf(isRetryableCommandFailure).
		WithFailureThreshold(3).
		WithDelay(30 * time.Second).
		WithJitter(5 * time.Second).
		Build()
	actual, loaded := commandBreakers.LoadOrStore(name, breaker)
	if !loaded {
		return breaker
	}
	stored, ok := actual.(circuitbreaker.CircuitBreaker[CommandResult])
	if !ok {
		panic("outbound: invalid command circuit breaker")
	}
	return stored
}

func isRetryableCommandFailure(result CommandResult, err error) bool {
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
	output := strings.ToLower(result.Stderr + "\n" + commandErr.stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"timed out",
		"timeout",
		"temporarily unavailable",
		"remote end hung up",
		"502",
		"503",
		"504",
	} {
		if strings.Contains(output, marker) {
			return true
		}
	}
	return false
}
