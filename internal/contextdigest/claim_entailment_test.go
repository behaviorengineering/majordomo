package contextdigest

import (
	"strings"
	"testing"
)

func TestClaimEntailedBoardLikeCases(t *testing.T) {
	t.Parallel()

	roles := packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/board", Role: roleDTO, Evidence: []string{"json_tags"}},
			{Path: "internal/adapter", Role: roleAdapter, Evidence: []string{"exported_funcs"}},
			{Path: "internal/domain", Role: roleUnknown, Evidence: []string{"exported_funcs"}},
			{Path: "internal/kitchen", Role: roleAggregator, Evidence: []string{"orchestration_export"}},
			{Path: "internal/httpapi", Role: roleHTTPSurface, Evidence: []string{"delivery:http"}},
			{Path: "internal/cli", Role: roleEntrypoint, Evidence: []string{"has_main"}},
		},
		Edges: []packageRoleEdge{
			{From: "internal/adapter", To: "internal/board", Kind: edgeFillsDTO},
		},
	}
	constraints := buildCapabilityConstraints(roles)

	cases := []struct {
		name       string
		sliceID    string
		paths      []string
		claims     []string
		wantIssues bool
		wantSubstr string
	}{
		{
			name: "dto_data_shape_ok", sliceID: "board", paths: []string{"internal/board"},
			claims: []string{capDataShape}, wantIssues: false,
		},
		{
			name: "dto_fill_dto_without_edge_fail", sliceID: "board", paths: []string{"internal/board"},
			claims: []string{capFillDTO}, wantIssues: true, wantSubstr: "fill_dto",
		},
		{
			name: "adapter_fill_dto_via_is_ok", sliceID: "adapter", paths: []string{"internal/adapter"},
			claims: []string{capFillDTO}, wantIssues: false,
		},
		{
			name: "adapter_fill_dto_via_edge_ok", sliceID: "adapter", paths: []string{"internal/adapter"},
			claims: []string{capFillDTO, capAdaptExternal}, wantIssues: false,
		},
		{
			name: "dto_own_domain_rules_fail", sliceID: "board", paths: []string{"internal/board"},
			claims: []string{capOwnDomainRules}, wantIssues: true, wantSubstr: "own_domain_rules",
		},
		{
			name: "unknown_triage_own_domain_fail", sliceID: "triage", paths: []string{"internal/domain"},
			claims: []string{capOwnDomainRules}, wantIssues: true, wantSubstr: "own_domain_rules",
		},
		{
			name: "aggregator_own_domain_ok", sliceID: "kitchen", paths: []string{"internal/kitchen"},
			claims: []string{capOwnDomainRules}, wantIssues: false,
		},
		{
			name: "synchronize_state_fail", sliceID: "board", paths: []string{"internal/board"},
			claims: []string{capSynchronizeState}, wantIssues: true, wantSubstr: "synchronize_state",
		},
		{
			name: "server_delivery_http_ok", sliceID: "http", paths: []string{"internal/httpapi"},
			claims: []string{capServeHTTP}, wantIssues: false,
		},
		{
			name: "orchestrate_without_entrypoint_fail", sliceID: "board", paths: []string{"internal/board"},
			claims: []string{capOrchestrate}, wantIssues: true, wantSubstr: "orchestrate",
		},
		{
			name: "orchestrate_entrypoint_ok", sliceID: "cli", paths: []string{"internal/cli"},
			claims: []string{capOrchestrate}, wantIssues: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issues := rejectUnentailedClaims(tc.sliceID, tc.claims, tc.paths, constraints, roles)
			if tc.wantIssues {
				if len(issues) == 0 {
					t.Fatalf("expected issues for claims %v", tc.claims)
				}
				if tc.wantSubstr != "" && !strings.Contains(strings.Join(issues, "\n"), tc.wantSubstr) {
					t.Fatalf("issues=%v want substr %q", issues, tc.wantSubstr)
				}
				return
			}
			if len(issues) != 0 {
				t.Fatalf("unexpected issues=%v", issues)
			}
		})
	}
}

func TestClaimEntailedFillDTOViaEdgeOnlyPackage(t *testing.T) {
	t.Parallel()
	// Package role unknown (empty is) but outbound fills_dto edge entails fill_dto.
	roles := packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/writer", Role: roleUnknown, Evidence: []string{"exported_funcs"}},
			{Path: "internal/board", Role: roleDTO, Evidence: []string{"json_tags"}},
		},
		Edges: []packageRoleEdge{
			{From: "internal/writer", To: "internal/board", Kind: edgeFillsDTO},
		},
	}
	constraints := buildCapabilityConstraints(roles)
	issues := rejectUnentailedClaims("writer", []string{capFillDTO}, []string{"internal/writer"}, constraints, roles)
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
}
