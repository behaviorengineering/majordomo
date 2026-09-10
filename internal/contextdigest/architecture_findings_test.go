package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/contextgate"
	"github.com/behaviorengineering/typology/catalog"
)

func TestExtractArchitectureFindings(t *testing.T) {
	arch := `# Architecture

## Intended

Something.

## Findings

The following findings need a correction or an explicit boundary-debt decision:

- ` + "`localgit`" + `: SliceBinding localgit -> gitboard missing but cross-slice import exists
- ` + "`pruneagent`" + `: SliceBinding pruneagent -> gitboard missing
- ` + "`remotegit`" + `: SliceBinding remotegit -> gitboard missing

1. Read the relevant catalog rows and the evidence named by each finding.
2. Fix the code or catalog when the boundary is wrong.
`
	got := extractArchitectureFindings(arch)
	if len(got) != 3 {
		t.Fatalf("findings=%v", got)
	}
	if !strings.Contains(got[0], "localgit") {
		t.Fatalf("first=%q", got[0])
	}
}

func TestParseMissingSliceBinding(t *testing.T) {
	from, to, ok := parseMissingSliceBinding("`triage`: SliceBinding triage -> platform-config missing but cross-slice import exists")
	if !ok || from != "triage" || to != "platform-config" {
		t.Fatalf("got from=%q to=%q ok=%v", from, to, ok)
	}
	if _, _, ok := parseMissingSliceBinding("unmapped package internal/foo"); ok {
		t.Fatal("expected non-binding finding to fail parse")
	}
}

func TestApplyEvidencedLibraryBindingsAddsOnlyLibraryTargets(t *testing.T) {
	typo := catalog.Typology{
		Slices: []catalog.Slice{
			{ID: "triage", Objective: "Triage work.", Owns: []catalog.Component{{ID: "triage-core", Path: "internal/triage", Layer: catalog.LayerDomain}}},
			{ID: "board", Objective: "Board domain.", Owns: []catalog.Component{{ID: "board-core", Path: "internal/board", Layer: catalog.LayerDomain}}},
		},
		Libraries: []catalog.Library{
			{ID: "platform-config", Purpose: "Shared config.", Owns: []catalog.Component{{ID: "internal-config", Path: "internal/config"}}},
		},
	}
	findings := []string{
		"`triage`: SliceBinding triage -> platform-config missing but cross-slice import exists",
		"`board`: SliceBinding board -> triage missing but cross-slice import exists",
	}
	out, changed := applyEvidencedLibraryBindings(typo, findings)
	if !changed {
		t.Fatal("expected library binding to be added")
	}
	if len(out.SliceBindings) != 1 {
		t.Fatalf("bindings=%v", out.SliceBindings)
	}
	if out.SliceBindings[0].From != "triage" || out.SliceBindings[0].To != "platform-config" {
		t.Fatalf("binding=%v", out.SliceBindings[0])
	}
	if out.SliceBindings[0].Kind != catalog.SliceReads {
		t.Fatalf("kind=%q", out.SliceBindings[0].Kind)
	}
	// Idempotent
	out2, changed2 := applyEvidencedLibraryBindings(out, findings)
	if changed2 {
		t.Fatal("expected no change on second pass")
	}
	if len(out2.SliceBindings) != 1 {
		t.Fatalf("bindings after second pass=%v", out2.SliceBindings)
	}
}

func TestExtractArchitectureFindingsFromDriftHeading(t *testing.T) {
	arch := `# Architecture

## Drift and design questions

- ` + "`localgit`" + `: SliceBinding localgit -> gitboard missing
- ` + "`pruneagent`" + `: SliceBinding pruneagent -> gitboard missing

## Agent review protocol

Ignore this.
`
	got := extractArchitectureFindings(arch)
	if len(got) != 2 {
		t.Fatalf("findings=%v", got)
	}
	if !architectureHasFindings(arch) {
		t.Fatal("expected architectureHasFindings true for Drift heading")
	}
}

func TestValidateHumanInterventionOutputsRejectsCompleteStatus(t *testing.T) {
	findings := []string{"`localgit`: SliceBinding missing"}
	journey := "## Status\nRefinement complete.\n\n## Technical debt\n\n| Package | Debt |\n| --- | --- |\n| localgit | binding |\n"
	err := validateHumanInterventionOutputs(findings, journey, "localgit needs a human decision", "- localgit")
	if err == nil {
		t.Fatal("expected complete status rejection")
	}
}

func TestValidateHumanInterventionOutputsRequiresCoverage(t *testing.T) {
	findings := []string{"`localgit`: SliceBinding missing", "`remotegit`: SliceBinding missing"}
	journey := "## Status\nOpen.\n\n## Technical debt\n\n| Package | Debt |\n| --- | --- |\n| localgit | binding |\n"
	err := validateHumanInterventionOutputs(findings, journey, "localgit only", "- localgit")
	if err == nil {
		t.Fatal("expected remotegit coverage failure")
	}
}

func TestDigestPRBodyIncludesPriority(t *testing.T) {
	body, err := digestPRBody(0, "abc123", contextgate.Sidecar{}, "- localgit needs a binding decision")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "## Priority: human decisions") {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "localgit needs a binding decision") {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "teaching context") {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "started the context cursor") {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(body, "Context digest catch-up") {
		t.Fatalf("mechanical catch-up line still present: %s", body)
	}
}

func TestDigestPRBodyCatchupWalk(t *testing.T) {
	body, err := digestPRBody(3, "def456", contextgate.Sidecar{Status: contextgate.StatusDone}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "walked 3 default-branch commit") {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "the conversation is complete") {
		t.Fatalf("body=%s", body)
	}
}

func TestLoadPRPriorityMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, prPriorityRel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- decide binding\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadPRPriorityMarkdown(dir)
	if got != "- decide binding" {
		t.Fatalf("got=%q", got)
	}
}
