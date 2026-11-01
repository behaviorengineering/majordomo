package command

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), "printf", []string{"output"}, nil, "")
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunCapturesCommandOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := Run(ctx, "sh", []string{"-c", "printf stdout; printf stderr >&2"}, nil, "")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if stdout != "stdout" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "stderr" {
		t.Fatalf("stderr = %q", stderr)
	}
}
