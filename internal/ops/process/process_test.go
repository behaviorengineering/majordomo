package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestRunnerRequiresDeadline(t *testing.T) {
	_, err := NewRunner().Run(context.Background(), Command{Name: os.Args[0]})
	if !errors.Is(err, ErrMissingDeadline) {
		t.Fatalf("Run() error = %v, want %v", err, ErrMissingDeadline)
	}
}

func TestRunnerCapturesCommandOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := NewRunner().Run(ctx, Command{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess", "--"},
		Env:  append(os.Environ(), "GO_WANT_HELPER_PROCESS=1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "process output\n" {
		t.Fatalf("stdout = %q, want %q", result.Stdout, "process output\n")
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "process output")
	os.Exit(0)
}
