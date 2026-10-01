package ops

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCommandRequiresDeadline(t *testing.T) {
	_, err := RunCommand(context.Background(), CommandSpec{Name: "true"})
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunCommandCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	result, err := RunCommand(ctx, CommandSpec{
		Name: "sh",
		Args: []string{"-c", "printf stdout; printf stderr >&2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "stdout" || result.Stderr != "stderr" {
		t.Fatalf("unexpected output: %#v", result)
	}
}
