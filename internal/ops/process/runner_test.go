package process

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresContextDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), Command{Name: "echo"})
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected deadline error, got %v", err)
	}
}

func TestRunExecutesCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, _, err := Run(ctx, Command{
		Name:      "sh",
		Args:      []string{"-c", "printf ready"},
		Operation: "test-process",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ready" {
		t.Fatalf("stdout=%q", stdout)
	}
}
