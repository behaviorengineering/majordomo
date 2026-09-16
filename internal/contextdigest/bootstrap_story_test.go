package contextdigest

import (
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func TestFormatBootstrapStorySectionQueryServedRepoNotMajordomoBootstrap(t *testing.T) {
	q := formatBootstrapStorySectionQuery("gitboard", bootstrapStorySection{
		ID:          "mission",
		Instruction: "Mission markdown",
		Current:     "# Mission\n",
	}, "")
	if strings.Contains(q, "Majordomo bootstrap") {
		t.Fatalf("query still asks for Majordomo bootstrap: %s", q)
	}
	if !strings.Contains(q, "gitboard") {
		t.Fatalf("query missing served repo id: %s", q)
	}
}

func TestRejectMajordomoAsProduct(t *testing.T) {
	err := rejectMajordomoAsProduct("gitboard", "mission", "Majordomo provides an automated framework for triage.")
	if err == nil {
		t.Fatal("expected rejection")
	}
	if err := rejectMajordomoAsProduct("majordomo", "mission", "Majordomo provides control-plane review."); err != nil {
		t.Fatalf("majordomo repo should allow Majordomo product voice: %v", err)
	}
	if err := rejectMajordomoAsProduct("gitboard", "mission", "gitboard provides a local-first board for forge work."); err != nil {
		t.Fatalf("served-repo voice should pass: %v", err)
	}
}

func TestRequireBootstrapStoryEvidenceNeedsLedgerAfterRefine(t *testing.T) {
	input := BootstrapStoryInput{
		RepoID:                 "gitboard",
		ReadmeSnapshot:         "# gitboard\n",
		TypologyRefinedCatalog: "slices: []\n",
		GeneratedAt:            time.Now().UTC(),
	}
	manifest := contextstore.TypologyManifest{
		RepoID:                   "gitboard",
		Mode:                     contextstore.TypologyModeDiscover,
		RefineStatus:             contextstore.TypologyRefineComplete,
		RefinedSnapshotPath:      "refined_snapshot.yaml",
		SliceObjectiveLedgerPath: "slice_objective_ledger.yaml",
		ArchitecturePath:         "architecture_brief.md",
	}
	err := requireBootstrapStoryEvidence(input, manifest)
	if err == nil {
		t.Fatal("expected missing ledger body to fail")
	}
	if !strings.Contains(err.Error(), "slice_objective_ledger") {
		t.Fatalf("error=%v", err)
	}
	input.TypologySliceObjectiveLedger = "slices:\n  - id: board\n    objective: dto types\n"
	if err := requireBootstrapStoryEvidence(input, manifest); err != nil {
		t.Fatalf("full evidence should pass: %v", err)
	}
}

func TestValidateBootstrapStoryEvidenceMap(t *testing.T) {
	err := validateBootstrapStoryEvidenceMap(BootstrapStoryInput{
		RepoID:                       "gitboard",
		ReadmeSnapshot:               "# x\n",
		TypologyRefinedCatalog:       "slices: []\n",
		TypologySliceObjectiveLedger: "",
	})
	if err == nil {
		t.Fatal("expected empty ledger rejection when refined present")
	}
}
