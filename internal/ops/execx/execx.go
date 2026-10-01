// Package execx provides bounded process execution for operator commands.
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

// Result contains captured process output.
type Result struct {
	Stdout string
	Stderr string
}

var networkBreaker = circuitbreaker.NewBuilder[Result]().
	WithFailureThreshold(5).
	WithDelay(30 * time.Second).
	Build()

var networkRetry = retrypolicy.NewBuilder[Result]().
	WithMaxRetries(2).
	WithBackoff(100*time.Millisecond, 2*time.Second).
	WithJitterFactor(0.25).
	HandleIf(func(_ Result, err error) bool {
		return retryable(err)
	}).
	ReturnLastFailure().
	Build()

// Run executes a process with the caller's bounded context.
func Run(ctx context.Context, name string, args []string, env []string, dir string, networked bool) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("process execution requires a context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Result{}, errors.New("process execution requires a deadline")
	}

	run := func() (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		cmd.Dir = dir
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
		if err != nil {
			return result, fmt.Errorf("%s: %w", name, err)
		}
		return result, nil
	}

	if !networked {
		return run()
	}
	return failsafe.With(networkBreaker, networkRetry).Get(run)
}

func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection",
		"could not resolve",
		"early eof",
		"network",
		"temporarily unavailable",
		"timed out",
		"timeout",
		"429",
		"502",
		"503",
		"504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// Environment returns a copy of the current process environment.
func Environment() []string {
	return append([]string(nil), os.Environ()...)
}
