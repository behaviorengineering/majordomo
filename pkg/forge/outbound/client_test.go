package outbound_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/pkg/forge/outbound"
)

func TestDoWithRetrySucceedsAfter5xx(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := outbound.DoWithRetry(outbound.Client(0), req, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits=%d", hits.Load())
	}
}

func TestDoWithRetryMissingDeadline(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.invalid/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := outbound.DoWithRetry(outbound.Client(0), req, 2)
	if !errors.Is(err, outbound.ErrMissingDeadline) {
		t.Fatalf("err=%v", err)
	}
	if resp != nil {
		t.Fatal("expected nil response")
	}
}

func TestRequireDeadline(t *testing.T) {
	if err := outbound.RequireDeadline(context.Background()); !errors.Is(err, outbound.ErrMissingDeadline) {
		t.Fatalf("background: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := outbound.RequireDeadline(ctx); err != nil {
		t.Fatal(err)
	}
}
