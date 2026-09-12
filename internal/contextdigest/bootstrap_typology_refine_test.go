package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/typology/catalog"
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
	if err := os.WriteFile(filepath.Join(evidence, "package_roles.yaml"), []byte("packages:\n  - path: internal/demo\n    role: unknown\n    confidence: 0\n    inspected_stage: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftDir, "architecture_draft.md"), []byte("# Draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(analysis, "README.md"), []byte("# Demo product\n\nRun `demo serve` to open the UI.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := contextstore.TypologyManifest{
		RepoID:               "demo",
		SourceSHA:            "abc123",
		GeneratedAt:          time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Mode:                 contextstore.TypologyModeDiscover,
		ModuleScope:          ".",
		SnapshotPath:         "snapshot.yaml",
		ArchitecturePath:     contextstore.TypologyArchitectureBriefPath,
		RefineStatus:         contextstore.TypologyRefinePending,
		GraphPath:            "graph.txt",
		PackageContractsPath: "package_contracts.md",
		PackageRolesPath:     "package_roles.yaml",
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
		if !strings.Contains(in.PackageRoles, "internal/demo") {
			t.Fatalf("package_roles=%q", in.PackageRoles)
		}
		if !strings.Contains(in.CapabilityConstraints, "internal/demo") {
			t.Fatalf("capability_constraints=%q", in.CapabilityConstraints)
		}
		if !strings.Contains(in.ReadmeSnapshot, "demo serve") {
			t.Fatalf("readme_snapshot=%q", in.ReadmeSnapshot)
		}
		return TypologyRefineOutput{
			ClusterProposalMD:  "# Cluster\n\nKeep demo.\n\n## Capability constraints (is / is-not)\n\n- `internal/demo` role=unknown is=[] must_not=[]\n",
			RefinedCatalogYAML: refined,
			JourneyMD:          "# Journey\n\n## Technical debt & boundary violations\n\nNone.\n",
			ObjectiveLedgerYAML: `slices:
  - id: demo
    owned_paths: [internal/demo]
    evidence: [DemoType]
    claims: [data_shape]
    objective: Demo bounded context for refine tests.
    verdict: grounded
`,
			ObjectiveClaimsYAML: `slices:
  - id: demo
    claims: [data_shape]
`,
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
	if updated.HumanInterventionPath != "human_intervention.md" {
		t.Fatalf("human_intervention_path=%q", updated.HumanInterventionPath)
	}
	if updated.PackageCapabilityConstraintsPath != "package_capability_constraints.yaml" {
		t.Fatalf("constraints_path=%q", updated.PackageCapabilityConstraintsPath)
	}
	if updated.SliceObjectiveClaimsPath != "slice_objective_claims.yaml" {
		t.Fatalf("claims_path=%q", updated.SliceObjectiveClaimsPath)
	}
	if updated.SliceObjectiveLedgerPath != "slice_objective_ledger.yaml" {
		t.Fatalf("ledger_path=%q", updated.SliceObjectiveLedgerPath)
	}
	for _, name := range []string{"cluster_proposal.md", "refined_snapshot.yaml", "journey.md", "snapshot.yaml", contextstore.TypologyArchitectureBriefPath, "human_intervention.md", "package_capability_constraints.yaml", "slice_objective_claims.yaml", "slice_objective_ledger.yaml"} {
		if _, err := os.Stat(filepath.Join(evidence, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(evidence, "pr_priority.md")); !os.IsNotExist(err) {
		t.Fatalf("pr_priority.md must be absent when architecture has no findings: %v", err)
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
	if _, err := validateRefinedCatalogYAML(raw, "", "demo", ""); err == nil {
		t.Fatal("expected structure validation error")
	}
}

func TestAnnotateCatalogYAMLErrorIncludesSnippetAndDump(t *testing.T) {
	raw := "id: demo\nslices:\n  id: broken\n"
	err := annotateCatalogYAMLError("typology refine load catalog", raw, fmt.Errorf("parse catalog: yaml: line 3: did not find expected '-' indicator"))
	if err == nil {
		t.Fatal("expected annotated error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "dump=") {
		t.Fatalf("expected dump path, got %q", msg)
	}
	if !strings.Contains(msg, ">    3 |") || !strings.Contains(msg, "id: broken") {
		t.Fatalf("expected line-3 snippet, got %q", msg)
	}
	dump := strings.TrimSpace(strings.Split(strings.Split(msg, "dump=")[1], "\n")[0])
	data, readErr := os.ReadFile(dump)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != raw {
		t.Fatalf("dump mismatch: %q", data)
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
	out, err := validateRefinedCatalogYAML(raw, raw, "demo", "")
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
	roles := `packages:
  - path: cmd/demo
    role: entrypoint
    confidence: 0.9
    evidence: [has_main]
    inspected_stage: 1
  - path: internal/demo
    role: unknown
    confidence: 0
    inspected_stage: 2
`
	out, err := validateRefinedCatalogYAML(raw, raw, "demo", roles)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "kind: cli") || !strings.Contains(out, "cmd/demo") {
		t.Fatalf("expected cmd package under surfaces, got %s", out)
	}
	if strings.Contains(out, "path: cmd/demo") && !strings.Contains(out, "surfaces:") {
		t.Fatalf("expected cmd package under surfaces, got %s", out)
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
	out, err := validateRefinedCatalogYAML(raw, draft, "demo", "")
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
	out, err := validateRefinedCatalogYAML(refined, draft, "demo", "")
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
	_, err := validateRefinedCatalogYAML(refined, draft, "demo", "")
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
	if _, err := validateRefinedCatalogYAML(refined, draft, "demo", ""); err != nil {
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
	ok, feedback := evaluateTypologyBoundaries(refined, "# Journey\n\nNo debt recorded.\n", arch, "")
	if ok {
		t.Fatal("expected evaluation failure for missing debt table")
	}
	if !strings.Contains(feedback, "majordomo_typology_debt_when_findings") {
		t.Fatalf("feedback=%q", feedback)
	}
	journey := "# Journey\n\n## Technical debt & boundary violations\n\n| Violation | Severity | Notes |\n| --- | --- | --- |\n| cross-slice import | medium | record for human journey |\n"
	ok, feedback = evaluateTypologyBoundaries(refined, journey, arch, "")
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
	ok, feedback := evaluateTypologyBoundaries(refined, "# Journey\n", "", "")
	if ok {
		t.Fatal("expected hollow objective failure")
	}
	if !strings.Contains(feedback, "majordomo_typology_objectives") {
		t.Fatalf("feedback=%q", feedback)
	}
}

func TestMechanicalPreClusterSeedsDeliveryAndLibraries(t *testing.T) {
	t.Parallel()
	roles := `packages:
  - path: cmd/demo
    role: entrypoint
    confidence: 0.9
    evidence: [has_main]
    inspected_stage: 1
  - path: internal/server
    role: server
    confidence: 0.9
    evidence: ["delivery:ui", "go_embed", "embeds_static"]
    inspected_stage: 1
  - path: internal/grpcserver
    role: server
    confidence: 0.9
    evidence: ["delivery:grpc", "imports_grpc", "grpc_service"]
    inspected_stage: 1
  - path: internal/board
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
  - path: internal/config
    role: config
    confidence: 0.9
    evidence: [imports_yaml, yaml_tags]
    inspected_stage: 1
  - path: internal/cliexec
    role: exec_runner
    confidence: 0.9
    evidence: [imports_os_exec]
    inspected_stage: 1
  - path: internal/agent
    role: aggregator
    confidence: 0.9
    evidence: [exported_logic]
    inspected_stage: 2
  - path: internal/analyze
    role: aggregator
    confidence: 0.9
    evidence: [exported_logic]
    inspected_stage: 2
  - path: internal/ledger
    role: aggregator
    confidence: 0.9
    evidence: [exported_logic]
    inspected_stage: 2
edges:
  - from: internal/agent
    to: internal/ledger
    kind: imports
  - from: internal/analyze
    to: internal/ledger
    kind: imports
  - from: cmd/demo
    to: internal/server
    kind: serves_server
`
	out := mechanicalPreCluster(mustParseRoles(roles))
	for _, needle := range []string{
		"Delivery surfaces",
		"Door walks",
		"`cmd/demo`",
		"`internal/server`",
		"`internal/grpcserver`",
		"Library candidates",
		"`dto`: `internal/board`",
		"`config`: `internal/config`",
		"`exec_runner`: `internal/cliexec`",
		"Unreached",
		"`internal/agent`",
		"entrypoint` and `server` are distinct delivery doors",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("mechanical seed missing %q:\n%s", needle, out)
		}
	}
}

func TestMechanicalPreClusterDoorWalkGitboardStyle(t *testing.T) {
	t.Parallel()
	roles := `packages:
  - path: cmd/gitboard
    role: entrypoint
    confidence: 0.9
    inspected_stage: 1
  - path: internal/server
    role: server
    confidence: 0.9
    inspected_stage: 1
  - path: internal/board
    role: dto
    confidence: 0.9
    inspected_stage: 1
  - path: internal/config
    role: config
    confidence: 0.9
    inspected_stage: 1
  - path: internal/cliexec
    role: exec_runner
    confidence: 0.9
    inspected_stage: 1
  - path: internal/dashboard
    role: aggregator
    confidence: 0.9
    inspected_stage: 2
  - path: internal/llm
    role: adapter
    confidence: 0.9
    inspected_stage: 2
  - path: internal/observability
    role: observability
    confidence: 0.9
    inspected_stage: 1
edges:
  - from: cmd/gitboard
    to: internal/server
    kind: serves_server
  - from: cmd/gitboard
    to: internal/config
    kind: reads_config
  - from: cmd/gitboard
    to: internal/board
    kind: imports
  - from: cmd/gitboard
    to: internal/cliexec
    kind: uses_runner
  - from: internal/server
    to: internal/dashboard
    kind: imports
  - from: internal/server
    to: internal/board
    kind: imports
  - from: internal/server
    to: internal/config
    kind: reads_config
  - from: internal/server
    to: internal/observability
    kind: imports
  - from: internal/dashboard
    to: internal/llm
    kind: imports
`
	out := mechanicalPreCluster(mustParseRoles(roles))
	for _, needle := range []string{
		"Door-private packages",
		"Shared across doors",
		"`internal/board`",
		"`internal/config`",
		"`internal/cliexec`",
		"`internal/dashboard`",
		"`internal/llm`",
		"Product slice seeds",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("mechanical seed missing %q:\n%s", needle, out)
		}
	}
	// CLI must not claim dashboard via serves_server flood.
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "- `cmd/gitboard`:") && strings.Contains(trim, "internal/dashboard") {
			t.Fatalf("CLI door must not claim dashboard private:\n%s", out)
		}
	}
}

func TestMechanicalPreClusterUnreachedWithoutDoors(t *testing.T) {
	t.Parallel()
	roles := `packages:
  - path: internal/agent
    role: aggregator
    confidence: 0.9
    evidence: [exported_logic]
    inspected_stage: 2
  - path: internal/analyze
    role: aggregator
    confidence: 0.9
    evidence: [exported_logic]
    inspected_stage: 2
  - path: internal/ledger
    role: aggregator
    confidence: 0.9
    evidence: [exported_logic]
    inspected_stage: 2
edges:
  - from: internal/agent
    to: internal/ledger
    kind: imports
  - from: internal/analyze
    to: internal/ledger
    kind: imports
`
	out := mechanicalPreCluster(mustParseRoles(roles))
	if !strings.Contains(out, "Unreached") {
		t.Fatalf("expected unreached section:\n%s", out)
	}
	if !strings.Contains(out, "`internal/agent`") || !strings.Contains(out, "`internal/analyze`") || !strings.Contains(out, "`internal/ledger`") {
		t.Fatalf("expected aggregators listed as unreached:\n%s", out)
	}
	if strings.Contains(out, "seed 1:") {
		t.Fatalf("expected no door-private product seeds without doors:\n%s", out)
	}
}

func TestScrubForbiddenHTTPEntrypointMerges(t *testing.T) {
	roles := `packages:
  - path: cmd/demo
    role: entrypoint
    confidence: 0.9
    inspected_stage: 1
  - path: internal/server
    role: server
    confidence: 0.9
    inspected_stage: 1
`
	in := "# Proposal\n\nMerge `internal/server` into `cmd/demo` because cmd is the sole importer.\n"
	out, note := scrubForbiddenHTTPEntrypointMerges(in, roles)
	if note == "" {
		t.Fatal("expected scrub note")
	}
	if !strings.Contains(out, "Mechanical override") {
		t.Fatalf("expected override block, got:\n%s", out)
	}
	if !strings.Contains(out, "MUST NOT merge `internal/server`") {
		t.Fatalf("expected explicit server reject, got:\n%s", out)
	}
	clean, note2 := scrubForbiddenHTTPEntrypointMerges("# Proposal\n\nKeep server separate.\n", roles)
	if note2 != "" {
		t.Fatalf("unexpected note for clean proposal: %q md=%s", note2, clean)
	}
}

func TestValidateRefinedCatalogYAMLDemotesExecAdapterOffCLISurface(t *testing.T) {
	raw := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
    surfaces:
      - id: demo-cli
        kind: cli
        components:
          - id: demo-cmd
            path: cmd/demo
          - id: cliexec
            path: ./internal/cliexec
`
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
      - id: cliexec
        path: ./internal/cliexec
      - id: demo-cmd
        path: cmd/demo
`
	roles := `packages:
  - path: cmd/demo
    role: entrypoint
    confidence: 0.9
    evidence: [has_main]
    inspected_stage: 1
  - path: internal/cliexec
    role: exec_runner
    confidence: 0.8
    evidence: [imports_os_exec, exports_run_surface]
    inspected_stage: 2
  - path: internal/demo
    role: unknown
    confidence: 0
    inspected_stage: 2
`
	out, err := validateRefinedCatalogYAML(raw, draft, "demo", roles)
	if err != nil {
		t.Fatal(err)
	}
	ok, feedback := evaluateTypologyBoundaries(out, "# Journey\n\n## Status\n\nOpen.\n\n## Technical debt & boundary violations\n\n| Violation | Severity | Notes |\n| --- | --- | --- |\n| sample | low | recorded |\n", "## Findings\n\n- sample\n", roles)
	if !ok {
		t.Fatalf("expected sanitize to demote cliexec off kind: cli, feedback=%s\nout=%s", feedback, out)
	}
	tmp, err := os.CreateTemp("", "majordomo-demote-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(out); err != nil {
		_ = tmp.Close()
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	owned := false
	onCLI := false
	for _, s := range typo.Slices {
		for _, c := range s.Owns {
			if normalizeCatalogPath(c.Path) == "internal/cliexec" {
				owned = true
			}
		}
		for _, surf := range s.Surfaces {
			if surf.Kind != catalog.InteractionCLI {
				continue
			}
			for _, c := range surf.Components {
				if normalizeCatalogPath(c.Path) == "internal/cliexec" {
					onCLI = true
				}
			}
		}
	}
	if !owned {
		t.Fatalf("expected cliexec retained under owns, got %s", out)
	}
	if onCLI {
		t.Fatalf("expected cliexec demoted off kind: cli, got %s", out)
	}
}

func TestValidateRefinedCatalogYAMLSeparatesHTTPSurfaceFromEntrypoint(t *testing.T) {
	raw := `id: demo
slices:
  - id: demo
    objective: Demo CLI and HTTP folded together.
    owns: []
    surfaces:
      - id: demo-cli
        kind: cli
        components:
          - id: demo-cmd
            path: cmd/demo
      - id: demo-api
        kind: api
        components:
          - id: demo-server
            path: internal/server
`
	draft := `id: demo
slices:
  - id: demo
    objective: Demo CLI and HTTP folded together.
    owns:
      - id: demo-cmd
        path: cmd/demo
      - id: demo-server
        path: internal/server
`
	roles := `packages:
  - path: cmd/demo
    role: entrypoint
    confidence: 0.9
    evidence: [has_main]
    inspected_stage: 1
  - path: internal/server
    role: server
    confidence: 0.9
    evidence: [go_embed, embeds_static]
    inspected_stage: 1
`
	out, err := validateRefinedCatalogYAML(raw, draft, "demo", roles)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := os.CreateTemp("", "majordomo-http-sep-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(out); err != nil {
		_ = tmp.Close()
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range typo.Slices {
		hasEntry, hasHTTP := false, false
		check := func(path string) {
			switch normalizeCatalogPath(path) {
			case "cmd/demo":
				hasEntry = true
			case "internal/server":
				hasHTTP = true
			}
		}
		for _, c := range s.Owns {
			check(c.Path)
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				check(c.Path)
			}
		}
		if hasEntry && hasHTTP {
			t.Fatalf("server still shares slice %q with entrypoint:\n%s", s.ID, out)
		}
	}
	foundHTTP := false
	for _, s := range typo.Slices {
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				if normalizeCatalogPath(c.Path) == "internal/server" {
					foundHTTP = true
					if surf.Kind != catalog.InteractionUI && surf.Kind != catalog.InteractionAPI {
						t.Fatalf("expected http surface kind api/ui, got %s", surf.Kind)
					}
				}
			}
		}
	}
	if !foundHTTP {
		t.Fatalf("expected internal/server retained on an HTTP surface, got:\n%s", out)
	}
}

func TestValidateRefinedCatalogYAMLRestoresOmittedExecAdapter(t *testing.T) {
	raw := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
    surfaces:
      - id: demo-cli
        kind: cli
        components:
          - id: demo-cmd
            path: cmd/demo
`
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
      - id: cliexec
        path: ./internal/cliexec
    surfaces:
      - id: demo-cli
        kind: cli
        components:
          - id: demo-cmd
            path: cmd/demo
`
	roles := `packages:
  - path: cmd/demo
    role: entrypoint
    confidence: 0.9
    evidence: [has_main]
    inspected_stage: 1
  - path: internal/cliexec
    role: exec_runner
    confidence: 0.8
    evidence: [imports_os_exec, exports_run_surface]
    inspected_stage: 2
`
	out, err := validateRefinedCatalogYAML(raw, draft, "demo", roles)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := os.CreateTemp("", "majordomo-restore-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(out); err != nil {
		_ = tmp.Close()
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	owned := false
	onCLI := false
	for _, s := range typo.Slices {
		for _, c := range s.Owns {
			if normalizeCatalogPath(c.Path) == "internal/cliexec" {
				owned = true
			}
		}
		for _, surf := range s.Surfaces {
			if surf.Kind != catalog.InteractionCLI {
				continue
			}
			for _, c := range surf.Components {
				if normalizeCatalogPath(c.Path) == "internal/cliexec" {
					onCLI = true
				}
			}
		}
	}
	if !owned {
		t.Fatalf("expected omitted cliexec restored under owns, got %s", out)
	}
	if onCLI {
		t.Fatalf("expected restored cliexec not on kind: cli, got %s", out)
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
	roles := `packages:
  - path: internal/cliexec
    role: exec_runner
    confidence: 0.8
    evidence: [imports_os_exec, exports_run_surface]
    inspected_stage: 2
`
	ok, feedback := evaluateTypologyBoundaries(refined, "# Journey\n", "", roles)
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
	ok, feedback := evaluateTypologyBoundaries(refined, journey, "", "")
	if ok {
		t.Fatal("expected journey consistency failure")
	}
	if !strings.Contains(feedback, "majordomo_typology_journey_consistent") {
		t.Fatalf("feedback=%q", feedback)
	}
	if !strings.Contains(feedback, "set Status to Open") {
		t.Fatalf("feedback missing fix hint: %q", feedback)
	}
}

func TestReconcileJourneyStatusWithDebtClearsContradiction(t *testing.T) {
	t.Parallel()
	refined := `id: demo
slices:
  - id: demo
    objective: Keep demo packages coherent for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
`
	journey := "Status: Completed refinement of the Typology catalog.\n\n| Slice | Debt | Action |\n| --- | --- | --- |\n| git | companions | Merge into git |\n"
	fixed := reconcileJourneyStatusWithDebt(journey)
	if journeyStatusClaimsComplete(fixed) {
		t.Fatalf("status still claims complete:\n%s", fixed)
	}
	if !journeyDebtStillSaysMerge(fixed) {
		t.Fatal("expected Merge into debt to remain")
	}
	ok, feedback := evaluateTypologyBoundaries(refined, fixed, "", "")
	if !ok {
		t.Fatalf("expected pass after reconcile, feedback=%q", feedback)
	}

	section := "## Status\n\nRefinement complete.\n\n## Decisions\n\nKept companions separate.\n\n## Technical debt\n\n| Slice | Action |\n| --- | --- |\n| git | Merge into git |\n"
	fixedSection := reconcileJourneyStatusWithDebt(section)
	if journeyStatusClaimsComplete(fixedSection) {
		t.Fatalf("section status still complete:\n%s", fixedSection)
	}
	ok, feedback = evaluateTypologyBoundaries(refined, fixedSection, "", "")
	if !ok {
		t.Fatalf("expected section pass after reconcile, feedback=%q", feedback)
	}
}

func TestJourneyStatusClaimsCompleteIgnoresDebtWording(t *testing.T) {
	t.Parallel()
	open := "Status: Open\n\nDebt: finish the incomplete merge story later.\n"
	if journeyStatusClaimsComplete(open) {
		t.Fatalf("open status should not count as complete: %q", open)
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
	out, err := validateRefinedCatalogYAML(raw, raw, "gitboard", "")
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

func TestValidateRefinedCatalogYAMLDropsInventedPurposeLessLibraryAndDuplicateOwns(t *testing.T) {
	draft := `id: gitboard
slices:
  - id: gitboard
    objective: Deliver the gitboard CLI and UI.
    owns:
      - id: internal-config
        path: ./internal/config
`
	refined := `id: gitboard
slices:
  - id: gitboard
    objective: Deliver the gitboard CLI and UI.
    owns:
      - id: internal-config
        path: ./internal/config
libraries:
  - id: technical-core
    owns:
      - id: internal-config
        path: ./internal/config
`
	out, err := validateRefinedCatalogYAML(refined, draft, "gitboard", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "technical-core") {
		t.Fatalf("expected invented purpose-less library dropped, got %s", out)
	}
	if !strings.Contains(out, "internal-config") {
		t.Fatalf("expected slice to keep internal-config, got %s", out)
	}
}

func TestValidateRefinedCatalogYAMLKeepsDraftLibraryPurposeAndSliceBinding(t *testing.T) {
	draft := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
libraries:
  - id: platform-config
    purpose: Shared configuration helpers with no product knowledge.
    owns:
      - id: internal-config
        path: internal/config
`
	refined := `id: demo
slices:
  - id: demo
    objective: Demo bounded context for refine tests.
    owns:
      - id: demo-core
        path: internal/demo
libraries:
  - id: platform-config
    owns:
      - id: internal-config
        path: internal/config
sliceBindings:
  - from: demo
    to: platform-config
    kind: reads
`
	out, err := validateRefinedCatalogYAML(refined, draft, "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "platform-config") {
		t.Fatalf("expected draft library kept, got %s", out)
	}
	if !strings.Contains(out, "Shared configuration helpers") {
		t.Fatalf("expected purpose filled from draft, got %s", out)
	}
	if !strings.Contains(out, "to: platform-config") {
		t.Fatalf("expected slice-to-library binding kept, got %s", out)
	}
}

func TestValidateRefinedCatalogYAMLUniquifiesDuplicateSurfaceIDs(t *testing.T) {
	raw := `id: demo
slices:
  - id: dashboard
    objective: Aggregate adapters into the product job map.
    owns:
      - id: dashboard-core
        path: internal/dashboard
    surfaces:
      - id: dashboard-ui
        kind: ui
        components:
          - id: dashboard-web
            path: internal/dashboard/web
      - id: dashboard-ui
        kind: api
        components:
          - id: dashboard-server
            path: internal/server
`
	out, err := validateRefinedCatalogYAML(raw, raw, "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "refined.yaml")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int{}
	for _, s := range typo.Slices {
		if s.ID != "dashboard" {
			continue
		}
		for _, surf := range s.Surfaces {
			ids[surf.ID]++
		}
	}
	for id, n := range ids {
		if n != 1 {
			t.Fatalf("surface id %q appears %d times in %s", id, n, out)
		}
	}
	if len(ids) < 2 {
		t.Fatalf("expected both ui and api surfaces kept with distinct ids, got %s", out)
	}
}
