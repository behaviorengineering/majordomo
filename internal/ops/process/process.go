// Package process runs external commands with explicit cancellation and resilience policies.
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

// Options configures one external process invocation.
type Options struct {
	Context   context.Context
	Name      string
	Args      []string
	Dir       string
	Env       []string
	Networked bool
}

// Result contains the output from one process attempt.
type Result struct {
	Stdout string
	Stderr string
}

var (
	breakerMu sync.Mutex
	breakers  = map[string]circuitbreaker.CircuitBreaker[Result]{}
)

// Run executes a process and applies resilience policies to network-backed commands.
func Run(opts Options) (Result, error) {
	if opts.Context == nil {
		return Result{}, errors.New("process context is required")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return Result{}, errors.New("process name is required")
	}
	if opts.Networked {
		if _, ok := opts.Context.Deadline(); !ok {
			return Result{}, fmt.Errorf("process %q requires a deadline", opts.Name)
		}
		return runNetworked(opts)
	}
	return runAttempt(opts)
}

func runNetworked(opts Options) (Result, error) {
	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(result Result, err error) bool {
			return isRetryable(result, err)
		}).
		WithMaxRetries(2).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()
	breaker := getBreaker(opts.Name)
	return failsafe.With(breaker, retry).WithContext(opts.Context).Get(func() (Result, error) {
		return runAttempt(opts)
	})
}

func getBreaker(name string) circuitbreaker.CircuitBreaker[Result] {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	if breaker, ok := breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		WithJitter(500 * time.Millisecond).
		Build()
	breakers[name] = breaker
	return breaker
}

func runAttempt(opts Options) (Result, error) {
	cmd := exec.CommandContext(opts.Context, opts.Name, opts.Args...)
	cmd.Dir = opts.Dir
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		return result, fmt.Errorf("run %s: %w", opts.Name, err)
	}
	return result, nil
}

func isRetryable(result Result, err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(result.Stderr)
	for _, marker := range []string{
		"authentication failed",
		"permission denied",
		"access denied",
		"not authorized",
		"non-fast-forward",
		"conflict",
		"unknown command",
		"no such file or directory",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}
