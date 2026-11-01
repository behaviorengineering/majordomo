package command

import (
	"context"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	t.Parallel()

	_, _, err := Run(context.Background(), Options{Name: "true"})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunRequiresName(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := Run(ctx, Options{})
	if err == nil {
		t.Fatal("expected missing command name error")
	}
}
