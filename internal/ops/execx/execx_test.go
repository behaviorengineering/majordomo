package execx

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	runner := NewRunner()
	_, err := runner.Run(context.Background(), "test", "sh", []string{"-c", "true"}, nil, "")
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	result, err := NewRunner().Run(ctx, "test", "sh", []string{"-c", "printf output"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "output" {
		t.Fatalf("stdout=%q", result.Stdout)
	}
}
