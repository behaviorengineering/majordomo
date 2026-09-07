package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/contextgate"
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
	body := digestPRBody(0, "abc123", contextgate.Sidecar{}, "- localgit needs a binding decision")
	if !strings.Contains(body, "## Priority: human decisions") {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "localgit needs a binding decision") {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "Bootstrapped context cursor") {
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
