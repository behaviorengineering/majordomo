package runner

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

// Options configures one external process invocation.
type Options struct {
	Name      string
	Args      []string
	Env       []string
	Dir       string
	Networked bool
}

// Result contains the captured process output.
type Result struct {
	Stdout string
	Stderr string
}

var breakers sync.Map

// Run executes a process with a caller-supplied deadline and bounded resilience policies.
func Run(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("process runner: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("process runner: context deadline is required")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return Result{}, errors.New("process runner: process name is required")
	}

	policies := []failsafe.Policy[*Result]{
		retrypolicy.NewBuilder[*Result]().
			HandleIf(retryable).
			WithBackoff(100*time.Millisecond, 2*time.Second).
			WithJitterFactor(0.25).
			WithMaxRetries(2).
			Build(),
	}
	if opts.Networked {
		policies = append(policies, breakerFor(opts.Name))
	}

	result, err := failsafe.With(policies...).WithContext(ctx).Get(func() (*Result, error) {
		return runOnce(ctx, opts)
	})
	if result == nil {
		return Result{}, err
	}
	return *result, err
}

func runOnce(ctx context.Context, opts Options) (*Result, error) {
	cmd := exec.CommandContext(ctx, opts.Name, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = opts.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := &Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		return result, fmt.Errorf("%s: %w", opts.Name, err)
	}
	return result, nil
}

func retryable(result *Result, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	if exitErr.ProcessState.ExitCode() == 137 {
		return true
	}
	output := strings.ToLower(resultOutput(result))
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"timed out",
		"timeout",
		"remote end hung up",
		"rate limit",
		"429",
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

func resultOutput(result *Result) string {
	if result == nil {
		return ""
	}
	return result.Stdout + "\n" + result.Stderr
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[*Result] {
	if existing, ok := breakers.Load(name); ok {
		return requireBreaker(existing)
	}
	breaker := circuitbreaker.NewBuilder[*Result]().
		HandleIf(retryable).
		WithFailureThreshold(3).
		WithDelay(10 * time.Second).
		Build()
	actual, loaded := breakers.LoadOrStore(name, breaker)
	if loaded {
		return requireBreaker(actual)
	}
	return breaker
}

func requireBreaker(value any) circuitbreaker.CircuitBreaker[*Result] {
	breaker, ok := value.(circuitbreaker.CircuitBreaker[*Result])
	if !ok {
		panic("process runner: invalid circuit breaker")
	}
	return breaker
}
