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

	failsafe "github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Command describes one process invocation.
type Command struct {
	Name      string
	Args      []string
	Dir       string
	Env       []string
	Operation string
	Remote    bool
}

type result struct {
	stdout string
	stderr string
}

var breakers struct {
	sync.Mutex
	byOperation map[string]circuitbreaker.CircuitBreaker[result]
}

// Run executes a command with the caller's deadline and cancellation.
func Run(ctx context.Context, command Command) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("process: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("process: context deadline is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return "", "", errors.New("process: command name is required")
	}

	operation := command.Operation
	if operation == "" {
		operation = command.Name
	}
	retryPolicy := retrypolicy.NewBuilder[result]().
		HandleIf(func(output result, err error) bool {
			return command.Remote && err != nil && retryable(output.stderr, err)
		}).
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()
	executor := failsafe.With[result](breakerFor(operation), retryPolicy).WithContext(ctx)
	output, err := executor.Get(func() (result, error) {
		stdout, stderr, runErr := runOnce(ctx, command)
		return result{stdout: stdout, stderr: stderr}, runErr
	})
	if err != nil {
		return output.stdout, output.stderr, fmt.Errorf("%s: %w", operation, err)
	}
	return output.stdout, output.stderr, nil
}

func runOnce(ctx context.Context, command Command) (string, string, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = command.Env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func breakerFor(operation string) circuitbreaker.CircuitBreaker[result] {
	breakers.Lock()
	defer breakers.Unlock()
	if breakers.byOperation == nil {
		breakers.byOperation = make(map[string]circuitbreaker.CircuitBreaker[result])
	}
	if breaker, ok := breakers.byOperation[operation]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[result]().
		HandleIf(func(output result, err error) bool {
			return err != nil && retryable(output.stderr, err)
		}).
		WithFailureThreshold(3).
		WithDelay(30 * time.Second).
		WithJitter(time.Second).
		Build()
	breakers.byOperation[operation] = breaker
	return breaker
}

func retryable(stderr string, err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	text := strings.ToLower(stderr + " " + err.Error())
	for _, marker := range []string{
		"timeout",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"could not resolve host",
		"service unavailable",
		"rate limit",
		"429",
		"502",
		"503",
		"504",
		"killed",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
