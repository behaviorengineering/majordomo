package process

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresContext(t *testing.T) {
	_, err := Run(Options{Name: "true"})
	if err == nil || !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("expected context error, got %v", err)
	}
}

func TestRunRequiresDeadlineForNetworkedProcess(t *testing.T) {
	_, err := Run(Options{Context: context.Background(), Name: "git", Networked: true})
	if err == nil || !strings.Contains(err.Error(), "requires a deadline") {
		t.Fatalf("expected deadline error, got %v", err)
	}
}

func TestRunCapturesLocalProcessOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := Run(Options{
		Context: ctx,
		Name:    "sh",
		Args:    []string{"-c", "printf stdout; printf stderr >&2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "stdout" || result.Stderr != "stderr" {
		t.Fatalf("unexpected output: %#v", result)
	}
}
