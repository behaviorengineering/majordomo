// Package commandexec provides context-bound execution for local and remote commands.
package commandexec

import (
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

// Request describes one command invocation.
type Request struct {
	Name        string
	Args        []string
	Env         []string
	Dir         string
	Stdout      io.Writer
	Stderr      io.Writer
	Retry       bool
	IsRetryable func(Result, error) bool
}

// Result contains the output from a command.
type Result struct {
	Stdout string
	Stderr string
}

type commandError struct {
	result Result
	err    error
}

func (e *commandError) Error() string {
	if strings.TrimSpace(e.result.Stderr) == "" {
		return e.err.Error()
	}
	return fmt.Sprintf("%v: %s", e.err, strings.TrimSpace(e.result.Stderr))
}

func (e *commandError) Unwrap() error {
	return e.err
}

var (
	breakersMu sync.Mutex
	breakers   = map[string]circuitbreaker.CircuitBreaker[Result]{}
)

// Run executes a command after validating its caller-supplied deadline.
func Run(ctx context.Context, req Request) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("command context deadline is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return Result{}, errors.New("command name is required")
	}
	if !req.Retry {
		return runOnce(ctx, req)
	}
	if req.IsRetryable == nil {
		return Result{}, errors.New("retry classifier is required for resilient commands")
	}

	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(result Result, err error) bool {
			return err != nil && req.IsRetryable(result, err)
		}).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxRetries(2).
		Build()
	executor := failsafe.With(getBreaker(req.Name, req.IsRetryable), retry).WithContext(ctx)
	result, err := executor.Get(func() (Result, error) {
		return runOnce(ctx, req)
	})
	if err != nil {
		return result, fmt.Errorf("run %s: %w", req.Name, err)
	}
	return result, nil
}

func runOnce(ctx context.Context, req Request) (Result, error) {
	cmd := exec.CommandContext(ctx, req.Name, req.Args...)
	cmd.Dir = req.Dir
	cmd.Env = req.Env
	var stdout, stderr strings.Builder
	if req.Stdout != nil {
		cmd.Stdout = io.MultiWriter(&stdout, req.Stdout)
	} else {
		cmd.Stdout = &stdout
	}
	if req.Stderr != nil {
		cmd.Stderr = io.MultiWriter(&stderr, req.Stderr)
	} else {
		cmd.Stderr = &stderr
	}
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		return result, &commandError{result: result, err: err}
	}
	return result, nil
}

func getBreaker(name string, isRetryable func(Result, error) bool) circuitbreaker.CircuitBreaker[Result] {
	breakersMu.Lock()
	defer breakersMu.Unlock()
	if breaker, ok := breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		HandleIf(func(result Result, err error) bool {
			return err != nil && isRetryable(result, err)
		}).
		WithFailureThreshold(3).
		Build()
	breakers[name] = breaker
	return breaker
}
