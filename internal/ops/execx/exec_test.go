package execx

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, err := Run(context.Background(), "command-that-must-not-run", nil, "", nil)
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing-deadline error, got %v", err)
	}
}

func TestRunExecutesWithDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	output, err := Run(ctx, "sh", []string{"-c", "printf ok"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if output != "ok" {
		t.Fatalf("output=%q, want %q", output, "ok")
	}
}
