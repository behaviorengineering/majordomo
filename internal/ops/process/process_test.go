package process

import (
	"context"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(Options{
		Context: context.Background(),
		Name:    "printf",
		Args:    []string{"ok"},
	})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := Run(Options{
		Context: ctx,
		Name:    "printf",
		Args:    []string{"ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}
