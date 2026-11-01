package execx

import (
	"context"
	"errors"
	"testing"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), "git", []string{"status"}, Options{})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestIsTransientClassifiesProcessErrors(t *testing.T) {
	transient := &CommandError{Err: errors.New("exit status 1"), Stderr: "connection reset by peer"}
	if !isTransient(transient) {
		t.Fatal("expected connection reset to be transient")
	}

	permanent := &CommandError{Err: errors.New("exit status 1"), Stderr: "authentication failed"}
	if isTransient(permanent) {
		t.Fatal("expected authentication failure to be permanent")
	}
}
