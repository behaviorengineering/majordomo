package executil

import (
	"context"
	"testing"
	"time"
)

func TestRunRequiresContext(t *testing.T) {
	_, _, err := Run(Options{Name: "git"})
	if err == nil {
		t.Fatal("expected missing context error")
	}
}

func TestRunRequiresDeadlineForNetworkedCommands(t *testing.T) {
	_, _, err := Run(Options{
		Context:   context.Background(),
		Name:      "git",
		Networked: true,
	})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunAcceptsDeadlineForLocalCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	if _, _, err := Run(Options{Context: ctx, Name: "true"}); err != nil {
		t.Fatalf("run local command: %v", err)
	}
}
