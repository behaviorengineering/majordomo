// Package outbound provides a shared retrying HTTP client for forge/SCM APIs.
package outbound

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/behaviorengineering/majordomo/internal/observability"
)

// DefaultTimeout for forge/SCM calls.
const DefaultTimeout = 60 * time.Second

// Client returns an instrumented HTTP client with the given timeout.
func Client(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	c := &http.Client{Timeout: timeout}
	observability.InstrumentHTTPClient(c)
	return c
}

// DoWithRetry performs req with exponential backoff on 429/5xx and transport errors.
func DoWithRetry(client *http.Client, req *http.Request, maxAttempts int) (*http.Response, error) {
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
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt == maxAttempts {
				return nil, lastErr
			}
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return resp, nil
	}
	return nil, lastErr
}
