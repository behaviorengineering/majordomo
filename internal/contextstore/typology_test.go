package contextstore

import (
	"strings"
	"testing"
	"time"
)

func TestValidateTypologyManifestRequiresPackageContractsWhenPending(t *testing.T) {
	t.Parallel()
	m := TypologyManifest{
		RepoID:           "demo",
		SourceSHA:        "abc",
		GeneratedAt:      time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Mode:             TypologyModeDiscover,
		ArchitecturePath: "architecture.md",
		RefineStatus:     TypologyRefinePending,
		GraphPath:        "graph.txt",
	}
	err := ValidateTypologyManifest(m)
	if err == nil || !strings.Contains(err.Error(), "package_contracts_path") {
		t.Fatalf("expected package_contracts_path error, got %v", err)
	}
	m.PackageContractsPath = "package_contracts.md"
	if err := ValidateTypologyManifest(m); err != nil {
		t.Fatal(err)
	}
}
