package observability

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const httpServiceLabel = "majordomo"

func httpClientSpanName(r *http.Request) string {
	if r == nil || r.URL == nil {
		return httpServiceLabel + " CLIENT HTTP"
	}
	return httpServiceLabel + " CLIENT " + r.Method + " " + r.URL.Host + r.URL.Path
}

type errorDetailTransport struct {
	base http.RoundTripper
}

func (t *errorDetailTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	span := trace.SpanFromContext(req.Context())
	redactSpanRequestURL(span, req)
	started := time.Now()
	resp, err := t.base.RoundTrip(req)
	if !span.IsRecording() {
		return resp, redactHTTPError(err)
	}
	redactSpanRequestURL(span, req)
	span.SetAttributes(attribute.Int64("http.duration_ms", time.Since(started).Milliseconds()))
	if err != nil {
		redacted := redactHTTPError(err)
		span.SetAttributes(
			attribute.String("error.class", classifyHTTPError(redacted)),
			attribute.String("error.message", redactURLsInText(redacted.Error())),
			attribute.Bool("timeout", isTimeout(redacted)),
		)
		return resp, redacted
	}
	if resp != nil && resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, resp.Status)
		span.SetAttributes(attribute.Bool("http.error", true))
	}
	return resp, err
}

func classifyHTTPError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if isTimeout(err) {
		return "timeout"
	}
	return "http_error"
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTimeout(urlErr.Err)
	}
	return false
}

func withErrorDetail(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(*errorDetailTransport); ok {
		return base
	}
	return &errorDetailTransport{base: base}
}

// WrapRoundTripper injects W3C traceparent on outbound HTTP.
func WrapRoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(*otelhttp.Transport); ok {
		return base
	}
	return otelhttp.NewTransport(withErrorDetail(base),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return httpClientSpanName(r)
		}),
		otelhttp.WithSpanOptions(trace.WithAttributes(
			attribute.String("openinference.span.kind", "CHAIN"),
			attribute.String("http.io", "client"),
		)),
	)
}

// InstrumentHTTPClient wraps client.Transport so LLM/forge calls join the active span.
func InstrumentHTTPClient(client *http.Client) {
	if client == nil {
		return
	}
	client.Transport = WrapRoundTripper(client.Transport)
}
