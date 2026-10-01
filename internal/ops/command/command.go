// Package command runs external processes with bounded, resilient execution.
package command

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	failsafe "github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Output contains the captured output of an external process.
type Output struct {
	Stdout string
	Stderr string
}

var breakers sync.Map

// Run executes a process with the caller's deadline and cancellation signal.
func Run(ctx context.Context, name string, args []string, env []string, dir string) (Output, error) {
	if ctx == nil {
		return Output{}, errors.New("command context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Output{}, errors.New("command context deadline is required")
	}

	retry := retrypolicy.NewBuilder[Output]().
		HandleIf(func(output Output, err error) bool {
			return ctx.Err() == nil && isTransient(name, args, output, err)
		}).
		ReturnLastFailure().
		WithBackoff(100*time.Millisecond, time.Second).
		WithJitterFactor(0.2).
		WithMaxRetries(2).
		Build()
	breakerKey := name
	if isNetworkOperation(name, args) {
		breakerKey += ":network"
	}
	breaker := breakerFor(breakerKey)
	output, err := failsafe.With(breaker, retry).
		WithContext(ctx).
		Get(func() (Output, error) {
			return runOnce(ctx, name, args, env, dir)
		})
	if err != nil {
		return output, fmt.Errorf("run %q: %w", name, err)
	}
	return output, nil
}

func breakerFor(key string) circuitbreaker.CircuitBreaker[Output] {
	if existing, ok := breakers.Load(key); ok {
		if breaker, ok := existing.(circuitbreaker.CircuitBreaker[Output]); ok {
			return breaker
		}
	}
	breaker := circuitbreaker.NewBuilder[Output]().
		HandleIf(func(output Output, err error) bool {
			return strings.HasSuffix(key, ":network") && isTransientProcess(output, err)
		}).
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		Build()
	if actual, loaded := breakers.LoadOrStore(key, breaker); loaded {
		stored, ok := actual.(circuitbreaker.CircuitBreaker[Output])
		if !ok {
			panic("command breaker has an invalid type")
		}
		return stored
	}
	return breaker
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (Output, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func isTransient(name string, args []string, output Output, err error) bool {
	if !isNetworkOperation(name, args) {
		return false
	}
	return isTransientProcess(output, err)
}

func isTransientProcess(output Output, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ProcessState == nil {
		return false
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return true
	}
	stderr := strings.ToLower(output.Stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"timed out",
		"temporary failure",
		"502",
		"503",
		"504",
		"429",
	} {
		if strings.Contains(stderr, marker) {
			return true
		}
	}
	return false
}

func isNetworkOperation(name string, args []string) bool {
	if name != "git" {
		return false
	}
	for _, arg := range args {
		switch arg {
		case "fetch", "pull", "push", "clone":
			return true
		}
	}
	return false
}
