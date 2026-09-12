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
