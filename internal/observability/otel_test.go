package observability

import "testing"

func TestResolveConfigDefaultDumpDirUsesTmp(t *testing.T) {
	t.Setenv("MAJORDOMO_INFERENCE_DUMP_DIR", "")
	t.Setenv("MAJORDOMO_OTEL_ENABLED", "1")

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
