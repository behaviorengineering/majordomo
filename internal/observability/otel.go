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

// Settings are file-config (or caller-supplied) defaults before env overrides.
type Settings struct {
	Enabled     *bool
	Endpoint    string
	APIKey      string
	ServiceName string
	Insecure    *bool
}

// Config bootstraps the process tracer provider.
type Config struct {
	ServiceName            string
	OTLPEndpoint           string // empty skips OTLP; dumps still work when FailureDumpDir set
	OTLPAPIKey             string // Bearer token for Phoenix/Arize; empty skips auth header
	OTLPInsecure           bool   // plaintext gRPC (typical for local Phoenix)
	FailureDumpDir         string
	FailureDumpMaxAgeHours int
	FailureDumpMaxFiles    int
	Enabled                bool
}

// ResolveConfig merges optional file settings with env defaults for Majordomo tracing.
// Tracing defaults on so failure dumps work without Phoenix.
// Env overrides file when set: MAJORDOMO_OTEL_ENABLED, MAJORDOMO_OTEL_ENDPOINT /
// OTEL_EXPORTER_OTLP_ENDPOINT, MAJORDOMO_OTEL_API_KEY / PHOENIX_API_KEY,
// MAJORDOMO_OTEL_SERVICE_NAME, MAJORDOMO_OTEL_INSECURE.
func ResolveConfig(outputDir string, fromFile ...Settings) Config {
	var file Settings
	if len(fromFile) > 0 {
		file = fromFile[0]
	}

	enabled := true
	if file.Enabled != nil {
		enabled = *file.Enabled
	}
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_ENABLED")); v != "" {
		enabled = !(v == "0" || strings.EqualFold(v, "false"))
	}

	endpoint := strings.TrimSpace(file.Endpoint)
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_ENDPOINT")); v != "" {
		endpoint = v
	} else if v := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); v != "" {
		endpoint = v
	}

	apiKey := strings.TrimSpace(file.APIKey)
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_API_KEY")); v != "" {
		apiKey = v
	} else if v := strings.TrimSpace(os.Getenv("PHOENIX_API_KEY")); v != "" {
		apiKey = v
	}

	svc := strings.TrimSpace(file.ServiceName)
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_SERVICE_NAME")); v != "" {
		svc = v
	}
	if svc == "" {
		svc = DefaultServiceName
	}

	insecure := otlpInsecureDefault(endpoint)
	if file.Insecure != nil {
		insecure = *file.Insecure
	}
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_INSECURE")); v != "" {
		insecure = !(v == "0" || strings.EqualFold(v, "false"))
	}

	dumpDir := strings.TrimSpace(os.Getenv("MAJORDOMO_INFERENCE_DUMP_DIR"))
	if dumpDir == "" {
		if outputDir != "" {
			dumpDir = outputDir + "/logs/inference-failures"
		} else {
			// Cwd-relative scratch when no output dir; keep out of the repo tree.
			dumpDir = "tmp/logs/inference-failures"
		}
	}

	return Config{
		ServiceName:            svc,
		OTLPEndpoint:           endpoint,
		OTLPAPIKey:             apiKey,
		OTLPInsecure:           insecure,
		FailureDumpDir:         dumpDir,
		FailureDumpMaxAgeHours: 48,
		FailureDumpMaxFiles:    20,
		Enabled:                enabled,
	}
}

func otlpInsecureDefault(endpoint string) bool {
	host := strings.ToLower(strings.TrimSpace(endpoint))
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1"
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
			exporterOpts := []otlptracegrpc.Option{
				otlptracegrpc.WithEndpoint(endpoint),
			}
			if cfg.OTLPInsecure {
				exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
			}
			if key := strings.TrimSpace(cfg.OTLPAPIKey); key != "" {
				// gRPC metadata keys must be lowercase for Phoenix auth.
				bearer := key
				if !strings.HasPrefix(strings.ToLower(bearer), "bearer ") {
					bearer = "Bearer " + key
				}
				exporterOpts = append(exporterOpts, otlptracegrpc.WithHeaders(map[string]string{
					"authorization": bearer,
				}))
			}
			exporter, exportErr := otlptracegrpc.New(context.Background(), exporterOpts...)
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
