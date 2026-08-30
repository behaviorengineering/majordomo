package observability_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestFailureDumpOnRootError(t *testing.T) {
	dir := t.TempDir()
	// Force fresh Init via isolated provider (bypass process once).
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(1.0)),
		sdktrace.WithSpanProcessor(observability.NewFailureDumpProcessorForTest(dir, 48, 20)),
	)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "root.cmd")
	span.SetStatus(codes.Error, "boom")
	span.End()
	_ = tp.ForceFlush(context.Background())

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 dump, got %d", len(entries))
	}
	body, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["reason"] != "root_span_error" {
		t.Fatalf("doc=%v", doc)
	}
}

func TestFailureDumpSkipsSuccess(t *testing.T) {
	dir := t.TempDir()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(1.0)),
		sdktrace.WithSpanProcessor(observability.NewFailureDumpProcessorForTest(dir, 48, 20)),
	)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "root.ok")
	span.SetStatus(codes.Ok, "")
	span.End()
	_ = tp.ForceFlush(context.Background())
	time.Sleep(20 * time.Millisecond)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no dump on success, got %d", len(entries))
	}
}
