package process

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	operrors "github.com/behaviorengineering/majordomo/internal/ops/errors"
)

// Options configures one external process invocation.
type Options struct {
	Name      string
	Args      []string
	Env       []string
	Dir       string
	Retryable func(error) bool
}

type result struct {
	stdout string
	stderr string
}

var (
	breakerMu sync.Mutex
	breakers  = map[string]circuitbreaker.CircuitBreaker[result]{}
)

// Run executes a process with a caller-owned deadline and shared resilience policies.
func Run(ctx context.Context, opts Options) (string, string, error) {
	if ctx == nil {
		return "", "", operrors.New(operrors.CodeInvalidArgument, "process.Run", "context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", operrors.New(operrors.CodeInvalidArgument, "process.Run", "context deadline is required")
	}
	if opts.Name == "" {
		return "", "", operrors.New(operrors.CodeInvalidArgument, "process.Run", "name is required")
	}

	retryable := opts.Retryable
	if retryable == nil {
		retryable = func(error) bool { return false }
	}
	retryPolicy := retrypolicy.NewBuilder[result]().
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		HandleIf(func(_ result, err error) bool {
			return retryable(err)
		}).
		ReturnLastFailure().
		Build()
	breaker := breakerFor(opts.Name)

	out, err := failsafe.With(breaker, retryPolicy).
		WithContext(ctx).
		Get(func() (result, error) {
			var stdout, stderr bytes.Buffer
			cmd := exec.CommandContext(ctx, opts.Name, opts.Args...)
			cmd.Env = opts.Env
			cmd.Dir = opts.Dir
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				return result{
					stdout: stdout.String(),
					stderr: stderr.String(),
				}, operrors.Wrap(err, opErrorsCode(err), "process.Run", "run "+opts.Name)
			}
			return result{
				stdout: stdout.String(),
				stderr: stderr.String(),
			}, nil
		})
	if err != nil {
		return out.stdout, out.stderr, err
	}
	return out.stdout, out.stderr, nil
}

func opErrorsCode(err error) operrors.Code {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return operrors.CodeCanceled
	}
	return operrors.CodeInternal
}

func breakerFor(name string) circuitbreaker.CircuitBreaker[result] {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	if breaker, ok := breakers[name]; ok {
		return breaker
	}
	breaker := circuitbreaker.NewBuilder[result]().
		WithFailureThreshold(5).
		WithDelay(30 * time.Second).
		Build()
	breakers[name] = breaker
	return breaker
}

// RetryableProcessError identifies a failed process that may represent a transient remote failure.
func RetryableProcessError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}
