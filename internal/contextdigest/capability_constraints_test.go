package contextdigest

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
)

func TestBuildCapabilityConstraintsDTOAndFillsDTO(t *testing.T) {
	t.Parallel()
	doc := packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/board", Role: roleDTO, Confidence: 0.9},
			{Path: "internal/adapter", Role: roleAdapter, Confidence: 0.9},
		},
		Edges: []packageRoleEdge{
			{From: "internal/adapter", To: "internal/board", Kind: edgeFillsDTO},
		},
	}
	out := buildCapabilityConstraints(doc)
	by := constraintsByPath(out)
	board := by["internal/board"]
	if !containsString(board.Is, capDataShape) {
		t.Fatalf("board is=%v", board.Is)
	}
	if !containsString(board.MustNot, capSynchronizeState) || !containsString(board.MustNot, capMergeAdapters) {
		t.Fatalf("board must_not=%v", board.MustNot)
	}
	if len(board.FilledBy) != 1 || board.FilledBy[0] != "internal/adapter" {
		t.Fatalf("filled_by=%v", board.FilledBy)
	}
	adapter := by["internal/adapter"]
	if !containsString(adapter.MustNot, capOrchestrate) {
		t.Fatalf("adapter must_not missing orchestrate: %v", adapter.MustNot)
	}
}

func TestBuildCapabilityConstraintsOrchestrateEntrypointOnly(t *testing.T) {
	t.Parallel()
	doc := packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/cli", Role: roleEntrypoint, Confidence: 0.9},
			{Path: "internal/pruneagent", Role: roleAggregator, Confidence: 0.8},
			{Path: "internal/mystery", Role: roleUnknown, Confidence: 0},
		},
	}
	out := buildCapabilityConstraints(doc)
	by := constraintsByPath(out)
	cli := by["internal/cli"]
	if !containsString(cli.Is, capOrchestrate) {
		t.Fatalf("entrypoint is=%v want orchestrate", cli.Is)
	}
	if containsString(cli.MustNot, capOrchestrate) {
		t.Fatalf("entrypoint must_not must not include orchestrate: %v", cli.MustNot)
	}
	for _, path := range []string{"internal/pruneagent", "internal/mystery"} {
		c := by[path]
		if !containsString(c.MustNot, capOrchestrate) {
			t.Fatalf("%s must_not=%v want orchestrate", path, c.MustNot)
		}
	}
}

func TestBuildCapabilityConstraintsExecProcessAndFillDTO(t *testing.T) {
	t.Parallel()
	doc := packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/pruneagent", Role: roleAggregator, Confidence: 0.8},
			{Path: "internal/cliexec", Role: roleExecRunner, Confidence: 0.9, Evidence: []string{"imports_os_exec"}},
			{Path: "internal/wrapper", Role: roleAggregator, Confidence: 0.7, Evidence: []string{"imports_os_exec"}},
			{Path: "internal/writer", Role: roleUnknown, Confidence: 0},
			{Path: "internal/adapter", Role: roleAdapter, Confidence: 0.9},
		},
		Edges: []packageRoleEdge{
			{From: "internal/writer", To: "internal/board", Kind: edgeFillsDTO},
		},
	}
	out := buildCapabilityConstraints(doc)
	by := constraintsByPath(out)

	prune := by["internal/pruneagent"]
	if !containsString(prune.MustNot, capExecProcess) || !containsString(prune.MustNot, capFillDTO) {
		t.Fatalf("pruneagent must_not=%v want exec_process and fill_dto", prune.MustNot)
	}
	cli := by["internal/cliexec"]
	if containsString(cli.MustNot, capExecProcess) {
		t.Fatalf("exec_runner must_not must not include exec_process: %v", cli.MustNot)
	}
	wrap := by["internal/wrapper"]
	if containsString(wrap.MustNot, capExecProcess) {
		t.Fatalf("imports_os_exec aggregator must allow exec_process; must_not=%v", wrap.MustNot)
	}
	writer := by["internal/writer"]
	if containsString(writer.MustNot, capFillDTO) {
		t.Fatalf("fills_dto from-edge must allow fill_dto; must_not=%v", writer.MustNot)
	}
	adapter := by["internal/adapter"]
	if containsString(adapter.MustNot, capFillDTO) {
		t.Fatalf("adapter must allow fill_dto; must_not=%v", adapter.MustNot)
	}
	if !containsString(adapter.MustNot, capExecProcess) {
		t.Fatalf("adapter must_not missing exec_process: %v", adapter.MustNot)
	}
}

func TestBuildCapabilityConstraintsPrestigeAndEvidenceFailClosed(t *testing.T) {
	t.Parallel()
	doc := packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/pruneagent", Role: roleUnknown, Confidence: 0},
			{Path: "internal/kitchen", Role: roleAggregator, Confidence: 0.8},
			{Path: "internal/server", Role: roleHTTPSurface, Confidence: 0.9, Evidence: []string{"delivery:http"}},
			{Path: "internal/obs", Role: roleUnknown, Confidence: 0, Evidence: []string{"imports_otel"}},
			{Path: "internal/cfg", Role: roleUnknown, Confidence: 0, Evidence: []string{"config_keys"}},
			{Path: "internal/cli", Role: roleUnknown, Confidence: 0, Evidence: []string{"has_main"}},
		},
	}
	out := buildCapabilityConstraints(doc)
	by := constraintsByPath(out)

	prune := by["internal/pruneagent"]
	for _, code := range []string{
		capOwnDomainRules, capAdaptExternal, capObservability, capAggregateViews,
		capDataShape, capServeHTTP, capRunCLI, capConfig, capSynchronizeState,
	} {
		if !containsString(prune.MustNot, code) {
			t.Fatalf("unknown pruneagent must_not=%v want %s", prune.MustNot, code)
		}
	}
	kitchen := by["internal/kitchen"]
	if containsString(kitchen.MustNot, capOwnDomainRules) {
		t.Fatalf("aggregator must allow own_domain_rules; must_not=%v", kitchen.MustNot)
	}
	if containsString(kitchen.MustNot, capAggregateViews) {
		t.Fatalf("aggregator must allow aggregate_views; must_not=%v", kitchen.MustNot)
	}
	server := by["internal/server"]
	if containsString(server.MustNot, capServeHTTP) || containsString(server.MustNot, capWireHandlers) {
		t.Fatalf("http_surface must allow serve/wire; must_not=%v", server.MustNot)
	}
	if !containsString(server.MustNot, capObservability) {
		t.Fatalf("http_surface without otel must forbid observability; must_not=%v", server.MustNot)
	}
	obs := by["internal/obs"]
	if containsString(obs.MustNot, capObservability) {
		t.Fatalf("imports_otel must allow observability; must_not=%v", obs.MustNot)
	}
	cfg := by["internal/cfg"]
	if containsString(cfg.MustNot, capConfig) {
		t.Fatalf("config_keys must allow config; must_not=%v", cfg.MustNot)
	}
	cli := by["internal/cli"]
	if containsString(cli.MustNot, capRunCLI) {
		t.Fatalf("has_main must allow run_cli; must_not=%v", cli.MustNot)
	}
}

func TestAppendConstraintClaimIssuesRejectsSyncOnDTO(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{
			{
				ID:        "board",
				Objective: "Shared board shapes for adapters.",
				Owns:      []catalog.Component{{ID: "board-core", Path: "internal/board"}},
			},
		},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/board", Role: roleDTO}},
		Edges:    []packageRoleEdge{{From: "internal/adapter", To: "internal/board", Kind: edgeFillsDTO}},
	})
	claims := sliceObjectiveClaimsDoc{
		Slices: []sliceObjectiveClaim{{ID: "board", Claims: []string{capSynchronizeState}}},
	}
	issues := appendConstraintClaimIssues(typo, constraints, claims, nil)
	if len(issues) == 0 {
		t.Fatal("expected claim intersection failure")
	}
	if !strings.Contains(issues[0], "majordomo_typology_role_grounding") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestAppendConstraintClaimIssuesAcceptsDataShape(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{
			{
				ID:        "board",
				Objective: "Shared board JSON shapes filled by adapters.",
				Owns:      []catalog.Component{{ID: "board-core", Path: "internal/board"}},
			},
		},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/board", Role: roleDTO}},
	})
	claims := sliceObjectiveClaimsDoc{
		Slices: []sliceObjectiveClaim{{ID: "board", Claims: []string{capDataShape}}},
	}
	issues := appendConstraintClaimIssues(typo, constraints, claims, nil)
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
}

func TestClusterProposalHasCapabilityConstraints(t *testing.T) {
	t.Parallel()
	doc := packageCapabilityConstraintsDoc{
		Packages: []packageCapabilityConstraint{{
			Path: "internal/board", Role: roleDTO, Is: []string{capDataShape}, MustNot: []string{capSynchronizeState},
		}},
	}
	ok, fb := clusterProposalHasCapabilityConstraints("# Cluster\n\nNo section.\n", doc)
	if ok {
		t.Fatal("expected missing section")
	}
	if fb == "" {
		t.Fatal("expected feedback")
	}
	md := formatCapabilityConstraintsMarkdown(doc)
	ok, fb = clusterProposalHasCapabilityConstraints(md, doc)
	if !ok {
		t.Fatalf("feedback=%q", fb)
	}
}

func TestParseObjectiveClaimsYAMLRejectsUnknownCode(t *testing.T) {
	t.Parallel()
	_, err := parseObjectiveClaimsYAML(`slices:
  - id: board
    claims: [not_a_real_code]
`)
	if err == nil {
		t.Fatal("expected unknown code error")
	}
}
