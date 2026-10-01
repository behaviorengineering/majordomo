package ops

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

// CommandSpec describes one external process invocation.
type CommandSpec struct {
	Name      string
	Args      []string
	Env       []string
	Dir       string
	Resilient bool
}

// CommandResult contains the captured process output.
type CommandResult struct {
	Stdout string
	Stderr string
}

var (
	breakerMu       sync.Mutex
	commandBreakers = map[string]circuitbreaker.CircuitBreaker[CommandResult]{}
)

// RunCommand executes a process with caller-owned cancellation and bounded retry.
func RunCommand(ctx context.Context, spec CommandSpec) (CommandResult, error) {
	if ctx == nil {
		return CommandResult{}, fmt.Errorf("run %s: context is required", spec.Name)
	}
	if _, ok := ctx.Deadline(); !ok {
		return CommandResult{}, fmt.Errorf("run %s: context deadline is required", spec.Name)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return CommandResult{}, fmt.Errorf("run command: name is required")
	}

	retry := retrypolicy.NewBuilder[CommandResult]().
		HandleIf(isTransient).
		WithBackoff(100*time.Millisecond, time.Second).
		WithJitterFactor(0.2).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()
	if spec.Resilient {
		return failsafe.With(commandBreaker(spec.Name), retry).
			WithContext(ctx).
			Get(func() (CommandResult, error) {
				return runCommandOnce(ctx, spec)
			})
	}
	return failsafe.With(retry).
		WithContext(ctx).
		Get(func() (CommandResult, error) {
			return runCommandOnce(ctx, spec)
		})
}

func runCommandOnce(ctx context.Context, spec CommandSpec) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Env = spec.Env
	cmd.Dir = spec.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, fmt.Errorf("execute %s: %w", spec.Name, err)
	}
	return result, nil
}

func isTransient(result CommandResult, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() < 0 {
		return true
	}
	text := strings.ToLower(result.Stdout + " " + result.Stderr + " " + err.Error())
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"remote end hung up",
		"temporarily unavailable",
		"timed out",
		"try again",
		" 429",
		" 500",
		" 502",
		" 503",
		" 504",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func commandBreaker(name string) circuitbreaker.CircuitBreaker[CommandResult] {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	if breaker, ok := commandBreakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[CommandResult]().
		HandleIf(isTransient).
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		Build()
	commandBreakers[name] = breaker
	return breaker
}
