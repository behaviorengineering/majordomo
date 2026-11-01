// Package command runs external processes with cancellation and retry policy.
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

// Options configures one external process invocation.
type Options struct {
	Context context.Context
	Name    string
	Args    []string
	Env     []string
	Dir     string
	Network bool
}

// Result contains the captured process output.
type Result struct {
	Stdout string
	Stderr string
}

var breakers = struct {
	sync.Mutex
	values map[string]circuitbreaker.CircuitBreaker[Result]
}{
	values: make(map[string]circuitbreaker.CircuitBreaker[Result]),
}

// Run executes an external process and fails closed without a caller deadline.
func Run(opts Options) (Result, error) {
	if opts.Context == nil {
		return Result{}, errors.New("command: context is required")
	}
	if _, ok := opts.Context.Deadline(); !ok {
		return Result{}, errors.New("command: context deadline is required")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return Result{}, errors.New("command: process name is required")
	}

	run := func() (Result, error) {
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(opts.Context, opts.Name, opts.Args...)
		cmd.Dir = opts.Dir
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if opts.Env != nil {
			cmd.Env = opts.Env
		}
		err := cmd.Run()
		result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
		if err != nil {
			return result, fmt.Errorf("%s %s: %w", opts.Name, strings.Join(opts.Args, " "), err)
		}
		return result, nil
	}

	if !opts.Network {
		return run()
	}

	retryPolicy := retrypolicy.NewBuilder[Result]().
		HandleIf(func(result Result, err error) bool {
			return isRetryable(result, err)
		}).
		WithMaxRetries(2).
		WithBackoff(200*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()
	breaker := breakerFor(opts.Name)
	return failsafe.With(retryPolicy, breaker).WithContext(opts.Context).Get(run)
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[Result] {
	breakers.Lock()
	defer breakers.Unlock()
	if breaker, ok := breakers.values[name]; ok {
		return breaker
	}
	created := circuitbreaker.NewBuilder[Result]().
		HandleIf(func(result Result, err error) bool {
			return isRetryable(result, err)
		}).
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()
	breakers.values[name] = created
	return created
}

func isRetryable(result Result, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	text := strings.ToLower(result.Stdout + "\n" + result.Stderr + "\n" + err.Error())
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
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
