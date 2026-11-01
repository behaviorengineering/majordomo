package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
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
	Stdout  io.Writer
	Stderr  io.Writer
	Retry   bool
	RetryIf func(error) bool
}

var (
	breakersMu sync.Mutex
	breakers   = map[string]circuitbreaker.CircuitBreaker[any]{}
)

// Run executes a process with caller cancellation and optional resilient retry.
func Run(opts Options) error {
	if opts.Context == nil {
		return errors.New("process context is required")
	}
	if opts.Name == "" {
		return errors.New("process name is required")
	}
	if opts.Retry {
		if _, ok := opts.Context.Deadline(); !ok {
			return errors.New("process context deadline is required for retry")
		}
		retryIf := opts.RetryIf
		if retryIf == nil {
			retryIf = Retryable
		}
		retryPolicy := retrypolicy.NewBuilder[any]().
			HandleIf(func(_ any, err error) bool {
				return retryIf(err)
			}).
			WithMaxRetries(2).
			WithBackoff(250*time.Millisecond, 2*time.Second).
			WithJitterFactor(0.25).
			ReturnLastFailure().
			Build()
		return failsafe.With[any](retryPolicy, breakerFor(opts.Name)).
			WithContext(opts.Context).
			Run(func() error {
				return runOnce(opts)
			})
	}
	return runOnce(opts)
}

// Retryable identifies failures that are safe to retry for an external process.
func Retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == 137 || exitErr.ExitCode() == 143
	}
	return false
}

func runOnce(opts Options) error {
	cmd := exec.CommandContext(opts.Context, opts.Name, opts.Args...)
	cmd.Env = opts.Env
	cmd.Dir = opts.Dir
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", opts.Name, err)
	}
	return nil
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[any] {
	breakersMu.Lock()
	defer breakersMu.Unlock()
	if breaker, ok := breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[any]().
		WithFailureThreshold(3).
		WithDelay(30 * time.Second).
		Build()
	breakers[name] = breaker
	return breaker
}
