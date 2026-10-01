package process

import (
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

type commandResult struct {
	stdout string
	stderr string
}

type commandFailure struct {
	command string
	stderr  string
	err     error
}

func (e *commandFailure) Error() string {
	if e.stderr == "" {
		return fmt.Sprintf("%s: %v", e.command, e.err)
	}
	return fmt.Sprintf("%s: %v: %s", e.command, e.err, e.stderr)
}

func (e *commandFailure) Unwrap() error {
	return e.err
}

var breakers sync.Map

// Options configures one process execution.
type Options struct {
	Context      context.Context
	Dependency   string
	RetryNetwork bool
	Name         string
	Args         []string
	Dir          string
	Env          []string
}

// Run executes a process under the caller's deadline.
func Run(opts Options) (string, string, error) {
	if opts.Context == nil {
		return "", "", errors.New("process context is required")
	}
	if _, ok := opts.Context.Deadline(); !ok {
		return "", "", errors.New("process context deadline is required")
	}

	retryPolicy := retrypolicy.NewBuilder[commandResult]().
		WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, 2*time.Second).
		WithJitter(100 * time.Millisecond).
		HandleIf(func(result commandResult, err error) bool {
			return opts.RetryNetwork && retryable(err)
		}).
		ReturnLastFailure().
		Build()

	executor := failsafe.With(retryPolicy).WithContext(opts.Context)
	if opts.RetryNetwork {
		executor = executor.Compose(breakerFor(opts.Dependency))
	}
	result, err := executor.Get(func() (commandResult, error) {
		return runOnce(opts)
	})
	return result.stdout, result.stderr, err
}

func runOnce(opts Options) (commandResult, error) {
	cmd := exec.CommandContext(opts.Context, opts.Name, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = opts.Env
	var result commandResult
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result.stdout = stdout.String()
	result.stderr = stderr.String()
	if err == nil {
		return result, nil
	}
	if opts.Context.Err() != nil {
		return result, fmt.Errorf("%s: %w", opts.Name, opts.Context.Err())
	}
	return result, &commandFailure{
		command: opts.Name,
		stderr:  strings.TrimSpace(result.stderr),
		err:     err,
	}
}

func breakerFor(dependency string) circuitbreaker.CircuitBreaker[commandResult] {
	if dependency == "" {
		dependency = "process"
	}
	if value, ok := breakers.Load(dependency); ok {
		if breaker, ok := value.(circuitbreaker.CircuitBreaker[commandResult]); ok {
			return breaker
		}
		breakers.Delete(dependency)
	}
	breaker := circuitbreaker.NewBuilder[commandResult]().
		WithFailureThreshold(3).
		WithDelay(30 * time.Second).
		WithJitter(5 * time.Second).
		Build()
	actual, loaded := breakers.LoadOrStore(dependency, breaker)
	if loaded {
		if existing, ok := actual.(circuitbreaker.CircuitBreaker[commandResult]); ok {
			return existing
		}
		breakers.Delete(dependency)
	}
	return breaker
}

func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	var failure *commandFailure
	if !errors.As(err, &failure) {
		return errors.Is(err, context.DeadlineExceeded)
	}
	stderr := strings.ToLower(failure.stderr)
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"could not resolve host",
		"network is unreachable",
		"timed out",
		"temporary failure",
		"unable to access",
	} {
		if strings.Contains(stderr, marker) {
			return true
		}
	}
	return false
}
