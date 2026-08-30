package observability

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

// Config bootstraps the process tracer provider.
type Config struct {
	ServiceName            string
	OTLPEndpoint           string // empty skips OTLP; dumps still work when FailureDumpDir set
	FailureDumpDir         string
	FailureDumpMaxAgeHours int
	FailureDumpMaxFiles    int
	Enabled                bool
}

// ResolveConfig reads env defaults for Majordomo tracing.
// Tracing defaults on so failure dumps work without Phoenix.
func ResolveConfig(outputDir string) Config {
	enabled := true
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_ENABLED")); v == "0" || strings.EqualFold(v, "false") {
		enabled = false
	}
	endpoint := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	dumpDir := strings.TrimSpace(os.Getenv("MAJORDOMO_INFERENCE_DUMP_DIR"))
	if dumpDir == "" {
		if outputDir != "" {
			dumpDir = outputDir + "/logs/inference-failures"
		} else {
			dumpDir = "logs/inference-failures"
		}
	}
	svc := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_SERVICE_NAME"))
	if svc == "" {
		svc = DefaultServiceName
	}
	return Config{
		ServiceName:            svc,
		OTLPEndpoint:           endpoint,
		FailureDumpDir:         dumpDir,
		FailureDumpMaxAgeHours: 48,
		FailureDumpMaxFiles:    20,
		Enabled:                enabled,
	}
}

var globalTP *sdktrace.TracerProvider
var globalInit sync.Once
var globalInitErr error

// Init installs the global tracer provider once per process.
func Init(cfg Config) (*sdktrace.TracerProvider, error) {
	globalInit.Do(func() {
		if !cfg.Enabled {
			return
		}
		serviceName := cfg.ServiceName
		if serviceName == "" {
			serviceName = DefaultServiceName
		}
		res, err := resource.New(context.Background(),
			resource.WithAttributes(
				semconv.ServiceNameKey.String(serviceName),
				semconv.ServiceVersionKey.String("1.0.0"),
			),
		)
		if err != nil {
			globalInitErr = fmt.Errorf("otel resource: %w", err)
			return
		}
		opts := []sdktrace.TracerProviderOption{
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.TraceIDRatioBased(1.0)),
		}
		if cfg.FailureDumpDir != "" {
			opts = append(opts, sdktrace.WithSpanProcessor(newFailureDumpProcessor(
				cfg.FailureDumpDir,
				cfg.FailureDumpMaxAgeHours,
				cfg.FailureDumpMaxFiles,
			)))
		}
		if cfg.OTLPEndpoint != "" {
			endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.OTLPEndpoint, "https://"), "http://")
			exporter, exportErr := otlptracegrpc.New(context.Background(),
				otlptracegrpc.WithEndpoint(endpoint),
				otlptracegrpc.WithInsecure(),
			)
			if exportErr != nil {
				globalInitErr = fmt.Errorf("otlp exporter: %w", exportErr)
				return
			}
			opts = append(opts, sdktrace.WithBatcher(
				exporter,
				sdktrace.WithBatchTimeout(2*time.Second),
				sdktrace.WithMaxExportBatchSize(512),
			))
		}
		tp := sdktrace.NewTracerProvider(opts...)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		globalTP = tp
	})
	return globalTP, globalInitErr
}

// Shutdown flushes and shuts down the global tracer provider.
func Shutdown(ctx context.Context) error {
	if globalTP == nil {
		return nil
	}
	err := globalTP.Shutdown(ctx)
	globalTP = nil
	return err
}

// Flush forces pending spans (including failure dumps) to complete.
func Flush(ctx context.Context) error {
	if globalTP == nil {
		return nil
	}
	return globalTP.ForceFlush(ctx)
}
