package command

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), "test", "sh", []string{"-c", "true"}, "", nil)
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunExecutesWithCallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := Run(ctx, "test-success", "sh", []string{"-c", "printf ok"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}
