package process

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), Options{Name: "true"})
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("err=%v, want missing deadline error", err)
	}
}

func TestRunRequiresName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	_, _, err := Run(ctx, Options{})
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err=%v, want missing name error", err)
	}
}
