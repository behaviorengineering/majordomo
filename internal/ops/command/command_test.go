package command

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	runner := NewRunner()
	_, _, err := runner.Run(context.Background(), Request{Name: "true"})
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := NewRunner().Run(ctx, Request{
		Name: "sh",
		Args: []string{"-c", "printf stdout; printf stderr >&2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "stdout" || stderr != "stderr" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}
