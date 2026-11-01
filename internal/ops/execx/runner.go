package execx

import (
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

type result struct {
	stdout string
	stderr string
}

type failure struct {
	err    error
	stderr string
}

func (f *failure) Error() string {
	if f.stderr == "" {
		return f.err.Error()
	}
	return fmt.Sprintf("%v: %s", f.err, f.stderr)
}

func (f *failure) Unwrap() error {
	return f.err
}

// Runner executes external commands with cancellation, bounded retries, and circuit breaking.
type Runner struct {
	breakers sync.Map
}

// New returns an external-command runner.
func New() *Runner {
	return &Runner{}
}

// Run executes a command. The context must carry the caller's deadline.
func (r *Runner) Run(ctx context.Context, name string, args []string, env []string, dir string) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("external command: context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", errors.New("external command: deadline is required")
	}
	if r == nil {
		return "", "", errors.New("external command: runner is required")
	}

	retry := retrypolicy.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool {
			return isRetryable(err)
		}).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		ReturnLastFailure().
		Build()
	breaker := r.breaker(name)
	out, err := failsafe.With(retry, breaker).
		WithContext(ctx).
		Get(func() (result, error) {
			return r.runOnce(ctx, name, args, env, dir)
		})
	return out.stdout, out.stderr, err
}

func (r *Runner) breaker(name string) circuitbreaker.CircuitBreaker[result] {
	if existing, ok := r.breakers.Load(name); ok {
		if breaker, ok := existing.(circuitbreaker.CircuitBreaker[result]); ok {
			return breaker
		}
	}
	created := circuitbreaker.NewBuilder[result]().
		HandleIf(func(_ result, err error) bool {
			return isRetryable(err)
		}).
		WithFailureThreshold(3).
		WithDelay(5 * time.Second).
		Build()
	actual, loaded := r.breakers.LoadOrStore(name, created)
	if !loaded {
		return created
	}
	if breaker, ok := actual.(circuitbreaker.CircuitBreaker[result]); ok {
		return breaker
	}
	return created
}

func (r *Runner) runOnce(ctx context.Context, name string, args []string, env []string, dir string) (result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := result{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		return out, &failure{err: err, stderr: strings.TrimSpace(out.stderr)}
	}
	return out, nil
}

func isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return status.Signal() == syscall.SIGKILL || status.Signal() == syscall.SIGTERM
		}
	}
	var commandErr *failure
	if !errors.As(err, &commandErr) {
		return false
	}
	stderr := strings.ToLower(commandErr.stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"could not resolve host",
		"network is unreachable",
		"temporarily unavailable",
		"service unavailable",
		" 429",
		" 502",
		" 503",
		" 504",
	} {
		if strings.Contains(stderr, marker) {
			return true
		}
	}
	return false
}

// Environment returns the current process environment when no custom environment is needed.
func Environment() []string {
	return os.Environ()
}
