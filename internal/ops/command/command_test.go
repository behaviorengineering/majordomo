package command

import (
	"context"
	"testing"
	"time"
)

func TestRunnerRequiresDeadline(t *testing.T) {
	_, _, err := NewRunner().Run(context.Background(), "echo", []string{"ok"}, nil, "")
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunnerCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := NewRunner().Run(ctx, "printf", []string{"ok"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}
