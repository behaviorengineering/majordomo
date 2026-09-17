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
	if strings.Contains(q, "small YAML object") || strings.Contains(q, "markdown: |\n  <full markdown") {
		t.Fatalf("query still asks for YAML markdown envelope: %s", q)
	}
	if !strings.Contains(q, "full markdown body for this section only") {
		t.Fatalf("query missing raw markdown contract: %s", q)
	}
}

func TestBootstrapStorySectionsArchitectureInstruction(t *testing.T) {
	secs := bootstrapStorySections(BootstrapStoryInput{})
	var arch string
	for _, s := range secs {
		if s.ID == "architecture" {
			arch = s.Instruction
			break
		}
	}
	if arch == "" {
		t.Fatal("architecture section missing")
	}
	if strings.Contains(strings.ToLower(arch), "jobs and doors") {
		t.Fatalf("architecture instruction still prefers jobs and doors: %s", arch)
	}
}

func TestParseBootstrapStoryMarkdownAnswer(t *testing.T) {
	t.Run("valid YAML indented markdown field", func(t *testing.T) {
		in := "markdown: |\n  # Title\n  body line\n"
		got, err := parseBootstrapStoryMarkdownAnswer(in)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if strings.Contains(got, "markdown:") {
			t.Fatalf("returned wrapper: %q", got)
		}
		if !strings.HasPrefix(got, "# Title") {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("unindented markdown pipe leak", func(t *testing.T) {
		in := "markdown: |\n# Title\nbody line\n"
		got, err := parseBootstrapStoryMarkdownAnswer(in)
		if err != nil {
			// Fail-closed is acceptable when strip cannot recover.
			if strings.Contains(err.Error(), "envelope") {
				return
			}
			t.Fatalf("unexpected err: %v", err)
		}
		if strings.Contains(got, "markdown: |") {
			t.Fatalf("must not return envelope: %q", got)
		}
		if !strings.Contains(got, "# Title") {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("leading yaml markdown pipe leak", func(t *testing.T) {
		in := "yaml\nmarkdown: |\n# Title\nbody line\n"
		got, err := parseBootstrapStoryMarkdownAnswer(in)
		if err != nil {
			if strings.Contains(err.Error(), "envelope") {
				return
			}
			t.Fatalf("unexpected err: %v", err)
		}
		if strings.Contains(got, "markdown: |") {
			t.Fatalf("must not return envelope: %q", got)
		}
		if !strings.Contains(got, "# Title") {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("raw heading markdown", func(t *testing.T) {
		in := "# Title\nbody line\n"
		got, err := parseBootstrapStoryMarkdownAnswer(in)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != strings.TrimSpace(in) {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("pure markdown pipe empty", func(t *testing.T) {
		_, err := parseBootstrapStoryMarkdownAnswer("markdown: |")
		if err == nil {
			t.Fatal("expected error")
		}
	})
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

func TestRejectMajordomoAsProductAllowsReadingMarkers(t *testing.T) {
	mission := `# Mission
<!-- majordomo-reading-nav:start -->
Reading path: [Prev: README.md](README.md) · [Next: architecture.md](architecture.md) · [TOC](README.md)
<!-- majordomo-reading-nav:end -->
Gitboard serves as a local code-change dashboard for GitLab and GitHub.
`
	if err := rejectMajordomoAsProduct("gitboard", "mission", mission); err != nil {
		t.Fatalf("reading markers must not trip product gate: %v", err)
	}
	if err := rejectMajordomoAsProduct("gitboard", "mission", mission+"\nMajordomo establishes the board.\n"); err == nil {
		t.Fatal("expected real Majordomo product voice to still fail")
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
