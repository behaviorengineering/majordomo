package observability

import "testing"

func TestResolveConfigDefaultDumpDirUsesTmp(t *testing.T) {
	t.Setenv("MAJORDOMO_INFERENCE_DUMP_DIR", "")
	t.Setenv("MAJORDOMO_OTEL_ENABLED", "1")
	t.Setenv("MAJORDOMO_OTEL_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	cfg := ResolveConfig("")
	if cfg.FailureDumpDir != "tmp/logs/inference-failures" {
		t.Fatalf("FailureDumpDir = %q, want tmp/logs/inference-failures", cfg.FailureDumpDir)
	}
}

func TestResolveConfigDumpDirUsesOutputDir(t *testing.T) {
	t.Setenv("MAJORDOMO_INFERENCE_DUMP_DIR", "")

	cfg := ResolveConfig("/work/out")
	want := "/work/out/logs/inference-failures"
	if cfg.FailureDumpDir != want {
		t.Fatalf("FailureDumpDir = %q, want %q", cfg.FailureDumpDir, want)
	}
}

func TestResolveConfigDumpDirEnvOverride(t *testing.T) {
	t.Setenv("MAJORDOMO_INFERENCE_DUMP_DIR", "/custom/dumps")

	cfg := ResolveConfig("/work/out")
	if cfg.FailureDumpDir != "/custom/dumps" {
		t.Fatalf("FailureDumpDir = %q, want /custom/dumps", cfg.FailureDumpDir)
	}
}

func TestResolveConfigFromFileSettings(t *testing.T) {
	t.Setenv("MAJORDOMO_OTEL_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("MAJORDOMO_OTEL_API_KEY", "")
	t.Setenv("PHOENIX_API_KEY", "")
	t.Setenv("MAJORDOMO_OTEL_ENABLED", "")
	t.Setenv("MAJORDOMO_OTEL_INSECURE", "")

	enabled := true
	cfg := ResolveConfig("", Settings{
		Enabled:  &enabled,
		Endpoint: "localhost:4317",
		APIKey:   "phoenix-secret",
	})
	if cfg.OTLPEndpoint != "localhost:4317" {
		t.Fatalf("OTLPEndpoint=%q", cfg.OTLPEndpoint)
	}
	if cfg.OTLPAPIKey != "phoenix-secret" {
		t.Fatalf("OTLPAPIKey=%q", cfg.OTLPAPIKey)
	}
	if !cfg.OTLPInsecure {
		t.Fatal("OTLPInsecure want true for localhost")
	}
	if !cfg.Enabled {
		t.Fatal("Enabled want true")
	}
}

func TestResolveConfigEnvOverridesFile(t *testing.T) {
	t.Setenv("MAJORDOMO_OTEL_ENDPOINT", "collector.example:4317")
	t.Setenv("MAJORDOMO_OTEL_API_KEY", "from-env")
	t.Setenv("MAJORDOMO_OTEL_INSECURE", "0")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("PHOENIX_API_KEY", "")

	cfg := ResolveConfig("", Settings{
		Endpoint: "localhost:4317",
		APIKey:   "from-file",
	})
	if cfg.OTLPEndpoint != "collector.example:4317" {
		t.Fatalf("OTLPEndpoint=%q", cfg.OTLPEndpoint)
	}
	if cfg.OTLPAPIKey != "from-env" {
		t.Fatalf("OTLPAPIKey=%q", cfg.OTLPAPIKey)
	}
	if cfg.OTLPInsecure {
		t.Fatal("OTLPInsecure want false when MAJORDOMO_OTEL_INSECURE=0")
	}
}

func TestOTLPInsecureDefault(t *testing.T) {
	if !otlpInsecureDefault("localhost:4317") {
		t.Fatal("localhost should be insecure")
	}
	if !otlpInsecureDefault("http://127.0.0.1:4317") {
		t.Fatal("127.0.0.1 should be insecure")
	}
	if otlpInsecureDefault("phoenix.example.com:4317") {
		t.Fatal("remote host should default secure")
	}
}
