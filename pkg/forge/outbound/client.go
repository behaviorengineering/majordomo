// Package outbound provides a shared retrying HTTP client for forge/SCM APIs.
package outbound

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/behaviorengineering/majordomo/pkg/platform/observability"
)

// DefaultTimeout for forge/SCM calls when the caller sets a job deadline.
// It is a secondary http.Client bound only; it MUST NOT substitute for a
// missing request context deadline (see RequireDeadline).
const DefaultTimeout = 60 * time.Second

// ErrMissingDeadline is returned when an outbound hop is asked to run without
// a caller-supplied context deadline.
var ErrMissingDeadline = errors.New("outbound: request context missing deadline")

// RequireDeadline fails closed when ctx has no deadline.
// Process entrypoints set the job budget; leaf clients MUST NOT invent one.
func RequireDeadline(ctx context.Context) error {
	if ctx == nil {
		return ErrMissingDeadline
	}
	if _, ok := ctx.Deadline(); !ok {
		return ErrMissingDeadline
	}
	return nil
}

// Client returns an instrumented HTTP client with the given timeout.
// Callers MUST still put a deadline on the request context before DoWithRetry.
func Client(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	c := &http.Client{Timeout: timeout}
	observability.InstrumentHTTPClient(c)
	return c
}

// DoWithRetry performs req with exponential backoff on 429/5xx and transport errors.
// req.Context() MUST have a deadline; otherwise ErrMissingDeadline is returned
// and no network call is made.
func DoWithRetry(client *http.Client, req *http.Request, maxAttempts int) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("outbound: request is required")
	}
	if err := RequireDeadline(req.Context()); err != nil {
		return nil, err
	}
	if client == nil {
		client = Client(DefaultTimeout)
	}
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	var lastErr error
	backoff := 200 * time.Millisecond
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		r := req.Clone(req.Context())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r.Body = body
		}
		resp, err := client.Do(r)
		if err != nil {
			lastErr = err
			if attempt == maxAttempts {
				return nil, err
			}
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				if closeErr := resp.Body.Close(); closeErr != nil {
					return nil, fmt.Errorf("discard retry response body: %w; close body: %v", err, closeErr)
				}
				return nil, fmt.Errorf("discard retry response body: %w", err)
			}
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("close retry response body: %w", err)
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt == maxAttempts {
				return nil, lastErr
			}
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			continue
		}
		return resp, nil
	}
	return nil, lastErr
}
