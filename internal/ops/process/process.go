// Package process runs external commands with caller deadlines and resilience policies.
package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

const (
	defaultMaxRetries   = 2
	defaultBackoff      = 100 * time.Millisecond
	defaultMaxBackoff   = time.Second
	defaultBreakerDelay = 30 * time.Second
)

// Spec describes one external command invocation.
type Spec struct {
	Name       string
	Args       []string
	Dir        string
	Env        []string
	Stdout     io.Writer
	Stderr     io.Writer
	Dependency string
	Retryable  func(error) bool
}

// Executor runs command specifications.
type Executor struct {
	run      func(context.Context, Spec) error
	breakers sync.Map
}

// NewExecutor creates an executor that invokes OS processes.
func NewExecutor() *Executor {
	return &Executor{run: runCommand}
}

// NewExecutorWithRunner creates an executor backed by runner, for tests.
func NewExecutorWithRunner(runner func(context.Context, Spec) error) *Executor {
	if runner == nil {
		panic("process runner is required")
	}
	return &Executor{run: runner}
}

// Run executes spec with its caller's deadline and resilience policies.
func (e *Executor) Run(ctx context.Context, spec Spec) error {
	if e == nil {
		return errors.New("process executor is required")
	}
	if ctx == nil {
		return errors.New("process context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("process context deadline is required")
	}
	if spec.Name == "" {
		return errors.New("process name is required")
	}

	dependency := spec.Dependency
	if dependency == "" {
		dependency = spec.Name
	}
	breaker := e.breaker(dependency, spec.Retryable)
	retry := retrypolicy.NewBuilder[struct{}]().
		HandleIf(func(_ struct{}, err error) bool {
			return spec.Retryable != nil && spec.Retryable(err)
		}).
		WithBackoff(defaultBackoff, defaultMaxBackoff).
		WithJitterFactor(0.2).
		WithMaxRetries(defaultMaxRetries).
		ReturnLastFailure().
		Build()

	return failsafe.With(breaker, retry).
		WithContext(ctx).
		Run(func() error {
			return e.run(ctx, spec)
		})
}

func (e *Executor) breaker(dependency string, retryable func(error) bool) circuitbreaker.CircuitBreaker[struct{}] {
	if value, ok := e.breakers.Load(dependency); ok {
		breaker, ok := value.(circuitbreaker.CircuitBreaker[struct{}])
		if !ok {
			panic("process breaker has an invalid type")
		}
		return breaker
	}
	candidate := circuitbreaker.NewBuilder[struct{}]().
		HandleIf(func(_ struct{}, err error) bool {
			return retryable != nil && retryable(err)
		}).
		WithFailureThreshold(5).
		WithDelay(defaultBreakerDelay).
		Build()
	var actual any = candidate
	if value, loaded := e.breakers.LoadOrStore(dependency, candidate); loaded {
		actual = value
	}
	breaker, ok := actual.(circuitbreaker.CircuitBreaker[struct{}])
	if !ok {
		panic("process breaker has an invalid type")
	}
	return breaker
}

func runCommand(ctx context.Context, spec Spec) error {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", spec.Name, err)
	}
	return nil
}
