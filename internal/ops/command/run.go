// Package command provides a cancellable, resilient seam for outbound commands.
package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Result contains the output captured from a command.
type Result struct {
	Stdout string
	Stderr string
}

// Runner executes one command attempt. Tests can inject a runner without
// starting a subprocess.
type Runner func(context.Context, string, []string, []string, string) (Result, error)

var (
	breakersMu sync.Mutex
	breakers   = make(map[string]circuitbreaker.CircuitBreaker[Result])
)

// Run executes a command with a caller-provided deadline and shared resilience
// policies for the named dependency.
func Run(ctx context.Context, dependency, name string, args []string, env []string, dir string, runner Runner) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("command: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("command: context deadline is required")
	}
	if runner == nil {
		runner = runOnce
	}

	retry := retrypolicy.NewBuilder[Result]().
		HandleIf(func(result Result, err error) bool {
			return isRetryable(result, err)
		}).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		WithMaxRetries(2).
		ReturnLastFailure().
		Build()

	result, err := failsafe.With(retry, breakerFor(dependency)).
		WithContext(ctx).
		Get(func() (Result, error) {
			return runner(ctx, name, args, env, dir)
		})
	if err != nil {
		return result, fmt.Errorf("run %s: %w", name, err)
	}
	return result, nil
}

func breakerFor(dependency string) circuitbreaker.CircuitBreaker[Result] {
	breakersMu.Lock()
	defer breakersMu.Unlock()
	if breaker, ok := breakers[dependency]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[Result]().
		WithFailureThreshold(5).
		WithDelay(5 * time.Second).
		Build()
	breakers[dependency] = breaker
	return breaker
}

func isRetryable(result Result, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ETIMEDOUT) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && processWasKilled(exitErr) {
		return true
	}
	stderr := strings.ToLower(result.Stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"timed out",
		"temporarily unavailable",
	} {
		if strings.Contains(stderr, marker) {
			return true
		}
	}
	return false
}

func processWasKilled(err *exec.ExitError) bool {
	status, ok := err.ProcessState.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}

func runOnce(ctx context.Context, name string, args []string, env []string, dir string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{Stdout: stdout.String(), Stderr: stderr.String()}, fmt.Errorf("command %s: %w", name, err)
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}
