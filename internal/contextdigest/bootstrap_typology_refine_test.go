package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func TestRefineTypologyEvidenceWritesProposal(t *testing.T) {
	analysis := t.TempDir()
	if err := os.WriteFile(filepath.Join(analysis, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	draftDir := filepath.Join(analysis, "tmp", "typology")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(t.TempDir(), "evidence", "typology")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
`
	if err := os.WriteFile(filepath.Join(draftDir, "typology.yaml"), []byte(draft), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "graph.txt"), []byte("graph: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_contracts.md"), []byte("# Package public contracts\n\n## ./internal/demo\n- hasMain: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftDir, "architecture_draft.md"), []byte("# Draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := contextstore.TypologyManifest{
		RepoID:               "demo",
		SourceSHA:            "abc123",
		GeneratedAt:          time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Mode:                 contextstore.TypologyModeDiscover,
		ModuleScope:          ".",
		SnapshotPath:         "snapshot.yaml",
		ArchitecturePath:     "architecture.md",
		RefineStatus:         contextstore.TypologyRefinePending,
		GraphPath:            "graph.txt",
		PackageContractsPath: "package_contracts.md",
		ClusterProposalPath:  "cluster_proposal.md",
		RefinedSnapshotPath:  "refined_snapshot.yaml",
		JourneyPath:          "journey.md",
	}
	if err := writeTypologyManifest(evidence, manifest); err != nil {
		t.Fatal(err)
	}

	refined := draft
	fake := typologyRefineGeneratorFunc(func(_ context.Context, in TypologyRefineInput) (TypologyRefineOutput, error) {
		if !strings.Contains(in.DraftCatalogYAML, "id: demo") {
			t.Fatalf("draft=%q", in.DraftCatalogYAML)
		}
		if !strings.Contains(in.PackageContracts, "hasMain: false") {
			t.Fatalf("package_contracts=%q", in.PackageContracts)
		}
		return TypologyRefineOutput{
			ClusterProposalMD:  "# Cluster\n\nKeep demo.\n",
			RefinedCatalogYAML: refined,
			JourneyMD:          "# Journey\n\n## Technical debt & boundary violations\n\nNone.\n",
		}, nil
	})
	binary := writeTypologyStub(t)
	if err := refineTypologyEvidence(context.Background(), Options{TypologyBinary: binary}, analysis, evidence, fake, nil); err != nil {
		t.Fatal(err)
	}
	updated, err := contextstore.ParseTypologyManifest(filepath.Join(evidence, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.RefineStatus != contextstore.TypologyRefineComplete {
		t.Fatalf("refine_status=%q", updated.RefineStatus)
	}
	for _, name := range []string{"cluster_proposal.md", "refined_snapshot.yaml", "journey.md", "snapshot.yaml", "architecture.md"} {
		if _, err := os.Stat(filepath.Join(evidence, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(evidence, "draft_snapshot.yaml")); !os.IsNotExist(err) {
		t.Fatalf("draft must not be written to evidence: %v", err)
	}
}

func TestValidateRefinedCatalogYAMLRejectsEmptyObjective(t *testing.T) {
	raw := `id: demo
slices:
  - id: demo
    owns:
      - id: demo-core
        path: internal/demo
`
	if _, err := validateRefinedCatalogYAML(raw, "", "demo"); err == nil {
		t.Fatal("expected structure validation error")
	}
}

func TestValidateRefinedCatalogYAMLDropsDanglingBindings(t *testing.T) {
	raw := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
sliceBindings:
  - from: demo
    to: missing
    kind: reads
`
	out, err := validateRefinedCatalogYAML(raw, raw, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "missing") {
		t.Fatalf("expected dangling binding removed, got %s", out)
	}
}

func TestValidateRefinedCatalogYAMLMovesCLIOntoSurfaces(t *testing.T) {
	raw := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
      - id: demo-cli
        path: cmd/demo
`
	out, err := validateRefinedCatalogYAML(raw, raw, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "path: cmd/demo") && !strings.Contains(out, "surfaces:") {
		t.Fatalf("expected cmd package under surfaces, got %s", out)
	}
	if !strings.Contains(out, "kind: cli") && !strings.Contains(out, "kind: \"cli\"") {
		// YAML may omit quotes
		if !strings.Contains(out, "cli") {
			t.Fatalf("expected cli surface, got %s", out)
		}
	}
}

func TestValidateRefinedCatalogYAMLStripsInventedDocPages(t *testing.T) {
	raw := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
    docs:
      pages:
        - kind: overview
          path: docs/develop/demo/overview.md
        - kind: components
          path: docs/develop/demo/components.md
`
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
`
	out, err := validateRefinedCatalogYAML(raw, draft, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "docs/develop") || strings.Contains(out, "docs:") {
		t.Fatalf("expected docs.pages stripped, got %s", out)
	}
}

func TestValidateRefinedCatalogYAMLRemapsUniqueNearestDraftPaths(t *testing.T) {
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: localgit
        path: ./internal/localgit
      - id: cmd-gitboard
        path: ./cmd/gitboard
`
	refined := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: git-local
        path: ./internal/git/local
    surfaces:
      - id: cli
        kind: cli
        components:
          - id: app
            path: ./internal/gitboard
`
	out, err := validateRefinedCatalogYAML(refined, draft, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "internal/git/local") || strings.Contains(out, "internal/gitboard") {
		t.Fatalf("expected remapped draft paths, got %s", out)
	}
	if !strings.Contains(out, "internal/localgit") || !strings.Contains(out, "cmd/gitboard") {
		t.Fatalf("expected draft paths after remap, got %s", out)
	}
}

func TestValidateRefinedCatalogYAMLRejectsInventedPathsWithoutMatch(t *testing.T) {
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: localgit
        path: ./internal/localgit
`
	refined := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: widget
        path: ./internal/totally-invented-widget
`
	_, err := validateRefinedCatalogYAML(refined, draft, "demo")
	if err == nil {
		t.Fatal("expected invented path error")
	}
	if !strings.Contains(err.Error(), "invented package paths") {
		t.Fatalf("error=%q", err)
	}
	if !strings.Contains(err.Error(), "totally-invented-widget") {
		t.Fatalf("error=%q", err)
	}
}

func TestValidateRefinedCatalogYAMLAllowsDraftPathsWithDotSlash(t *testing.T) {
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: localgit
        path: internal/localgit
`
	refined := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: localgit
        path: ./internal/localgit
`
	if _, err := validateRefinedCatalogYAML(refined, draft, "demo"); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateTypologyBoundariesRequiresDebtWhenFindings(t *testing.T) {
	refined := `id: demo
slices:
  - id: demo
    objective: Demo objective.
    owns:
      - id: demo-core
        path: internal/demo
`
	arch := "## Findings\n\n- `internal/x` imports `internal/y` across slices\n"
	ok, feedback := evaluateTypologyBoundaries(refined, "# Journey\n\nNo debt recorded.\n", arch)
	if ok {
		t.Fatal("expected evaluation failure for missing debt table")
	}
	if !strings.Contains(feedback, "majordomo_typology_debt_when_findings") {
		t.Fatalf("feedback=%q", feedback)
	}
	journey := "# Journey\n\n## Technical debt & boundary violations\n\n| Violation | Severity | Notes |\n| --- | --- | --- |\n| cross-slice import | medium | record for human journey |\n"
	ok, feedback = evaluateTypologyBoundaries(refined, journey, arch)
	if !ok {
		t.Fatalf("expected pass, feedback=%q", feedback)
	}
}

func TestEvaluateTypologyBoundariesRejectsHollowObjectives(t *testing.T) {
	refined := `id: demo
slices:
  - id: board
    objective: Provide board functionality
    owns:
      - id: board-core
        path: internal/board
`
	ok, feedback := evaluateTypologyBoundaries(refined, "# Journey\n", "")
	if ok {
		t.Fatal("expected hollow objective failure")
	}
	if !strings.Contains(feedback, "majordomo_typology_objectives") {
		t.Fatalf("feedback=%q", feedback)
	}
}

func TestEvaluateTypologyBoundariesRejectsExecAdapterCLISurface(t *testing.T) {
	refined := `id: demo
slices:
  - id: git
    objective: Run forge CLIs against local and remote git state.
    surfaces:
      - id: git-cli
        kind: cli
        components:
          - id: cliexec
            path: ./internal/cliexec
`
	ok, feedback := evaluateTypologyBoundaries(refined, "# Journey\n", "")
	if ok {
		t.Fatal("expected exec-adapter CLI surface failure")
	}
	if !strings.Contains(feedback, "majordomo_typology_adapter_surfaces") {
		t.Fatalf("feedback=%q", feedback)
	}
}

func TestEvaluateTypologyBoundariesRejectsJourneyMergeContradiction(t *testing.T) {
	refined := `id: demo
slices:
  - id: demo
    objective: Keep demo packages coherent for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
`
	journey := "Status: Completed refinement of the Typology catalog.\n\n| Slice | Debt | Action |\n| --- | --- | --- |\n| git | companions | Merge into git |\n"
	ok, feedback := evaluateTypologyBoundaries(refined, journey, "")
	if ok {
		t.Fatal("expected journey consistency failure")
	}
	if !strings.Contains(feedback, "majordomo_typology_journey_consistent") {
		t.Fatalf("feedback=%q", feedback)
	}
}

func TestValidateRefinedCatalogYAMLNormalizesTempCatalogID(t *testing.T) {
	raw := `id: majordomo-typology-777262492
slices:
  - id: demo
    objective: Keep demo packages coherent for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
`
	out, err := validateRefinedCatalogYAML(raw, raw, "gitboard")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "id: gitboard") {
		t.Fatalf("expected repo id, got %s", out)
	}
	if strings.Contains(out, "majordomo-typology-") {
		t.Fatalf("temp id remained: %s", out)
	}
}
