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

type humanInterventionGeneratorFunc func(context.Context, HumanInterventionInput) (HumanInterventionOutput, error)

func (f humanInterventionGeneratorFunc) Generate(ctx context.Context, input HumanInterventionInput) (HumanInterventionOutput, error) {
	return f(ctx, input)
}

func TestFlagHumanInterventionWritesPrioritiesAndWeaknesses(t *testing.T) {
	ctxRoot := t.TempDir()
	evidence := filepath.Join(ctxRoot, "evidence", "typology")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	arch := `# Architecture

## Findings

- ` + "`localgit`" + `: SliceBinding localgit -> gitboard missing
- ` + "`pruneagent`" + `: SliceBinding pruneagent -> gitboard missing
- ` + "`remotegit`" + `: SliceBinding remotegit -> gitboard missing
`
	for name, body := range map[string]string{
		contextstore.TypologyArchitectureBriefPath: arch,
		"refined_snapshot.yaml":                    "id: demo\nslices: []\n",
		"journey.md":                               "# Journey\n\n## Status\n\nRefinement complete.\n",
		"cluster_proposal.md":                      "# Cluster\n",
	} {
		if err := os.WriteFile(filepath.Join(evidence, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := contextstore.TypologyManifest{
		RepoID:               "demo",
		SourceSHA:            "abc",
		GeneratedAt:          time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Mode:                 contextstore.TypologyModeDiscover,
		ArchitecturePath:     contextstore.TypologyArchitectureBriefPath,
		RefineStatus:         contextstore.TypologyRefinePending,
		SnapshotPath:         "snapshot.yaml",
		GraphPath:            "graph.txt",
		PackageContractsPath: "package_contracts.md",
		ClusterProposalPath:  "cluster_proposal.md",
		RefinedSnapshotPath:  "refined_snapshot.yaml",
		JourneyPath:          "journey.md",
	}
	if err := writeTypologyManifest(evidence, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "snapshot.yaml"), []byte("id: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "graph.txt"), []byte("g\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_contracts.md"), []byte("# c\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := humanInterventionGeneratorFunc(func(_ context.Context, in HumanInterventionInput) (HumanInterventionOutput, error) {
		if !strings.Contains(in.FindingsList, "localgit") || !strings.Contains(in.FindingsList, "remotegit") {
			t.Fatalf("findings_list=%q", in.FindingsList)
		}
		return HumanInterventionOutput{
			JourneyMD: `# Journey

## Status

Open. Binding decisions required.

## Technical debt and boundary violations

| Finding | Decision needed |
| --- | --- |
| localgit | approve SliceBinding or temporary debt |
| pruneagent | approve SliceBinding or temporary debt |
| remotegit | approve SliceBinding or temporary debt |
`,
			HumanInterventionMD: "# Human intervention\n\n- localgit\n- pruneagent\n- remotegit\n",
			WeaknessesSeedMD:    "# Weaknesses\n\n- localgit binding\n- pruneagent binding\n- remotegit binding\n",
			PRPriorityMD:        "- localgit needs binding decision\n- pruneagent needs binding decision\n- remotegit needs binding decision\n",
		}, nil
	})
	if err := flagHumanIntervention(context.Background(), evidence, fake, nil); err != nil {
		t.Fatal(err)
	}
	updated, err := contextstore.ParseTypologyManifest(filepath.Join(evidence, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.HumanInterventionPath != "human_intervention.md" {
		t.Fatalf("path=%q", updated.HumanInterventionPath)
	}
	assertFileContains(t, filepath.Join(evidence, "human_intervention.md"), "localgit")
	assertFileContains(t, filepath.Join(evidence, "pr_priority.md"), "remotegit")
	assertFileContains(t, filepath.Join(ctxRoot, "weaknesses.md"), "pruneagent")
	journey, err := os.ReadFile(filepath.Join(evidence, "journey.md"))
	if err != nil {
		t.Fatal(err)
	}
	if journeyStatusClaimsComplete(string(journey)) {
		t.Fatalf("journey still claims complete: %s", journey)
	}
}

func TestFlagHumanInterventionSkipsFallback(t *testing.T) {
	evidence := filepath.Join(t.TempDir(), "evidence", "typology")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := contextstore.TypologyManifest{
		RepoID:           "demo",
		SourceSHA:        "abc",
		Mode:             contextstore.TypologyModeFallback,
		ArchitecturePath: contextstore.TypologyArchitectureBriefPath,
		RefineStatus:     contextstore.TypologyRefineSkipped,
	}
	if err := writeTypologyManifest(evidence, manifest); err != nil {
		t.Fatal(err)
	}
	called := false
	fake := humanInterventionGeneratorFunc(func(_ context.Context, _ HumanInterventionInput) (HumanInterventionOutput, error) {
		called = true
		return HumanInterventionOutput{}, nil
	})
	if err := flagHumanIntervention(context.Background(), evidence, fake, nil); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("fallback must skip intervention generator")
	}
}
