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
		ArchitecturePath: TypologyArchitectureBriefPath,
		RefineStatus:     TypologyRefinePending,
		GraphPath:        "graph.txt",
	}
	err := ValidateTypologyManifest(m)
	if err == nil || !strings.Contains(err.Error(), "package_contracts_path") {
		t.Fatalf("expected package_contracts_path error, got %v", err)
	}
	m.PackageContractsPath = "package_contracts.md"
	err = ValidateTypologyManifest(m)
	if err == nil || !strings.Contains(err.Error(), "package_roles_path") {
		t.Fatalf("expected package_roles_path error, got %v", err)
	}
	m.PackageRolesPath = "package_roles.yaml"
	if err := ValidateTypologyManifest(m); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTypologyManifestRequiresHumanInterventionWhenComplete(t *testing.T) {
	t.Parallel()
	m := TypologyManifest{
		RepoID:                           "demo",
		SourceSHA:                        "abc",
		GeneratedAt:                      time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Mode:                             TypologyModeDiscover,
		ArchitecturePath:                 TypologyArchitectureBriefPath,
		RefineStatus:                     TypologyRefineComplete,
		SnapshotPath:                     "snapshot.yaml",
		GraphPath:                        "graph.txt",
		PackageContractsPath:             "package_contracts.md",
		PackageRolesPath:                 "package_roles.yaml",
		PackageCapabilityConstraintsPath: "package_capability_constraints.yaml",
		SliceObjectiveClaimsPath:         "slice_meaning_claims.yaml",
		SliceObjectiveLedgerPath:         "slice_meaning_ledger.yaml",
		ClusterProposalPath:              "slice_grouping_proposal.yaml",
		RefinedSnapshotPath:              "refined_snapshot.yaml",
		JourneyPath:                      "journey.md",
	}
	err := ValidateTypologyManifest(m)
	if err == nil || !strings.Contains(err.Error(), "human_intervention_path") {
		t.Fatalf("expected human_intervention_path error, got %v", err)
	}
	m.HumanInterventionPath = "human_intervention.md"
	if err := ValidateTypologyManifest(m); err != nil {
		t.Fatal(err)
	}
}
