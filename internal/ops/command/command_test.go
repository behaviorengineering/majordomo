package command

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	t.Parallel()

	if _, err := Run(context.Background(), "true", nil, nil, ""); err == nil {
		t.Fatal("expected a missing deadline error")
	}
}

func TestRunCapturesOutput(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	output, err := Run(ctx, "sh", []string{"-c", "printf stdout; printf stderr >&2"}, nil, "")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output.Stdout != "stdout" {
		t.Fatalf("stdout = %q, want %q", output.Stdout, "stdout")
	}
	if !strings.Contains(output.Stderr, "stderr") {
		t.Fatalf("stderr = %q, want stderr", output.Stderr)
	}
}
