package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/pkg/catalog"
	"gopkg.in/yaml.v3"
)

func TestParseSliceObjectiveLedgerAnswerStripsBackticks(t *testing.T) {
	t.Parallel()
	_, claims, obj, verdict, err := parseSliceObjectiveLedgerAnswer(`
evidence: Adapter.Fill
claims: ` + "`adapt_external`" + `, ` + "`fill_dto`" + `
objective: Adapt forge payloads into board shapes.
verdict: grounded
`)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "grounded" || obj == "" {
		t.Fatalf("obj=%q verdict=%q", obj, verdict)
	}
	if !containsString(claims, capAdaptExternal) || !containsString(claims, capFillDTO) {
		t.Fatalf("claims=%v", claims)
	}
}

func TestParseSliceObjectiveLedgerAnswerOverclaim(t *testing.T) {
	t.Parallel()
	_, _, _, verdict, err := parseSliceObjectiveLedgerAnswer("verdict: overclaim\nevidence: none\nclaims: synchronize_state\nobjective: sync\n")
	if err != nil || verdict != "overclaim" {
		t.Fatalf("verdict=%q err=%v", verdict, err)
	}
}

func TestValidateLedgerRejectsSyncOnDTO(t *testing.T) {
	t.Parallel()
	roles := packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/board", Role: roleDTO, Evidence: []string{"json_tags"}}},
		Edges:    []packageRoleEdge{{From: "internal/adapter", To: "internal/board", Kind: edgeFillsDTO}},
	}
	constraints := buildCapabilityConstraints(roles)
	ledger := sliceObjectiveLedgerDoc{
		Slices: []sliceObjectiveLedgerEntry{{
			ID: "board", OwnedPaths: []string{"internal/board"},
			Evidence:  []string{"BoardPayload"},
			Claims:    []string{capSynchronizeState},
			Objective: "Central synchronization layer.",
			Verdict:   "grounded",
		}},
	}
	issues := validateLedgerAgainstConstraints(ledger, constraints, roles)
	if len(issues) == 0 {
		t.Fatal("expected disallowed claim against owned is=[]")
	}
	if !strings.Contains(issues[0], "not allowed by owned package is=") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestAppendLedgerObjectiveIssuesMismatch(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID: "board", Objective: "Central synchronization layer.",
			Owns: []catalog.Component{{ID: "b", Path: "internal/board"}},
		}},
	}
	ledger := sliceObjectiveLedgerDoc{
		Slices: []sliceObjectiveLedgerEntry{{
			ID: "board", OwnedPaths: []string{"internal/board"},
			Objective: "Shared board payload shapes.",
			Evidence:  []string{"BoardPayload"}, Claims: []string{capDataShape}, Verdict: "grounded",
		}},
	}
	_, _, issues := alignLedgerToRefinedCatalog(typo, ledger, buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/board", Role: roleDTO}},
	}))
	if len(issues) != 1 || !strings.Contains(issues[0], "does not match any contributing ledger objective") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestSeparateHTTPSurfacesKeepsSharedGatewayOnParent(t *testing.T) {
	t.Parallel()
	parentObj := "The operations slice manages CLI execution, HTTP serving, and configuration."
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID:        "operations",
			Objective: parentObj,
			Owns: []catalog.Component{
				{ID: "cmd", Path: "cmd/majordomo"},
				{ID: "gw", Path: "internal/aigateway"},
			},
			Surfaces: []catalog.Surface{{
				ID:   "operations-cli",
				Kind: catalog.InteractionCLI,
				Components: []catalog.Component{
					{ID: "cli", Path: "internal/cli"},
				},
			}},
		}},
	}
	roles := map[string]packageRoleNode{
		"cmd/majordomo":      {Path: "cmd/majordomo", Role: roleEntrypoint},
		"internal/cli":       {Path: "internal/cli", Role: roleEntrypoint},
		"internal/aigateway": {Path: "internal/aigateway", Role: roleHTTPSurface},
	}
	got := separateHTTPSurfacesFromEntrypoint(typo, roles, nil)
	for _, s := range got.Slices {
		if s.ID == "operations-http" {
			t.Fatalf("shared gateway must stay on operations, got extra slice %+v", got.Slices)
		}
	}
	var ops catalog.Slice
	for _, s := range got.Slices {
		if s.ID == "operations" {
			ops = s
		}
	}
	if ops.ID == "" {
		t.Fatalf("missing operations slice: %+v", got.Slices)
	}
	foundGW := false
	for _, surf := range ops.Surfaces {
		for _, c := range surf.Components {
			if normalizeRolePath(c.Path) == "internal/aigateway" {
				foundGW = true
				if surf.Kind != catalog.InteractionAPI && surf.Kind != catalog.InteractionUI {
					t.Fatalf("gateway surface kind=%s", surf.Kind)
				}
			}
		}
	}
	if !foundGW {
		t.Fatalf("expected aigateway on operations api surface, got %+v", ops)
	}
	ledger := sliceObjectiveLedgerDoc{
		Slices: []sliceObjectiveLedgerEntry{{
			ID:         "operations",
			OwnedPaths: []string{"cmd/majordomo", "internal/cli", "internal/aigateway"},
			Evidence:   []string{"ServeHTTP"},
			Claims:     []string{capServeHTTP, capRunCLI},
			Objective:  parentObj,
			Verdict:    "grounded",
		}},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "cmd/majordomo", Role: roleEntrypoint},
			{Path: "internal/cli", Role: roleEntrypoint},
			{Path: "internal/aigateway", Role: roleHTTPSurface, Evidence: []string{"delivery:http"}},
		},
	})
	_, _, issues := alignLedgerToRefinedCatalog(got, ledger, constraints)
	if len(issues) != 0 {
		t.Fatalf("align issues after keeping gateway on parent: %v", issues)
	}
}

func TestSeparateHTTPSurfacesSplitsWhenEntrypointIsSoleImporter(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID:        "demo",
			Objective: "Demo CLI and HTTP folded together.",
			Owns: []catalog.Component{
				{ID: "cmd", Path: "cmd/demo"},
				{ID: "srv", Path: "internal/server"},
			},
		}},
	}
	roles := map[string]packageRoleNode{
		"cmd/demo":        {Path: "cmd/demo", Role: roleEntrypoint},
		"internal/server": {Path: "internal/server", Role: roleHTTPSurface},
	}
	importers := map[string][]string{
		"internal/server": {"cmd/demo"},
	}
	got := separateHTTPSurfacesFromEntrypoint(typo, roles, importers)
	var parent, httpSlice catalog.Slice
	for _, s := range got.Slices {
		switch s.ID {
		case "demo":
			parent = s
		case "demo-http":
			httpSlice = s
		}
	}
	if parent.ID == "" || httpSlice.ID == "" {
		t.Fatalf("expected demo and demo-http, got %+v", got.Slices)
	}
	for _, c := range parent.Owns {
		if normalizeRolePath(c.Path) == "internal/server" {
			t.Fatalf("server still on parent owns: %+v", parent)
		}
	}
	found := false
	for _, surf := range httpSlice.Surfaces {
		for _, c := range surf.Components {
			if normalizeRolePath(c.Path) == "internal/server" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected server on demo-http, got %+v", httpSlice)
	}
}

func TestAlignLedgerToRefinedCatalogMergesByPackagePath(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID:        "git",
			Objective: "Local git adapters for board shapes.",
			Owns: []catalog.Component{
				{ID: "local", Path: "internal/localgit"},
				{ID: "remote", Path: "internal/remotegit"},
			},
		}},
	}
	ledger := sliceObjectiveLedgerDoc{
		Slices: []sliceObjectiveLedgerEntry{
			{
				ID: "localgit", OwnedPaths: []string{"internal/localgit"},
				Evidence: []string{"Clone"}, Claims: []string{capAdaptExternal},
				Objective: "Local git adapters for board shapes.", Verdict: "grounded",
			},
			{
				ID: "remotegit", OwnedPaths: []string{"internal/remotegit"},
				Evidence: []string{"Fetch"}, Claims: []string{capAdaptExternal, capFillDTO},
				Objective: "Remote git adapters.", Verdict: "grounded",
			},
		},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/localgit", Role: roleAdapter},
			{Path: "internal/remotegit", Role: roleAdapter},
		},
		Edges: []packageRoleEdge{
			{From: "internal/localgit", To: "internal/board", Kind: edgeFillsDTO},
			{From: "internal/remotegit", To: "internal/board", Kind: edgeFillsDTO},
		},
	})
	aligned, claims, issues := alignLedgerToRefinedCatalog(typo, ledger, constraints)
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	if len(aligned.Slices) != 1 || aligned.Slices[0].ID != "git" {
		t.Fatalf("aligned=%+v", aligned)
	}
	if !containsString(claims.Slices[0].Claims, capFillDTO) || !containsString(claims.Slices[0].Claims, capAdaptExternal) {
		t.Fatalf("claims=%v", claims.Slices[0].Claims)
	}
}

func TestAlignLedgerToRefinedCatalogDropsClaimsForbiddenByRefinedOwns(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{
			{
				ID:        "board",
				Objective: "Provide JSON data types shared across the dashboard UI.",
				Owns:      []catalog.Component{{ID: "board", Path: "internal/board"}},
			},
			{
				ID:        "dashboard",
				Objective: "Provide JSON data types shared across the dashboard UI.",
				Owns:      []catalog.Component{{ID: "dashboard", Path: "internal/dashboard"}},
			},
		},
	}
	// Wide draft ledger entry covers both paths; after refine split, each slice
	// keeps only claims allowed by its owned package is=[] priors.
	ledger := sliceObjectiveLedgerDoc{
		Slices: []sliceObjectiveLedgerEntry{{
			ID:         "ui",
			OwnedPaths: []string{"internal/board", "internal/dashboard"},
			Evidence:   []string{"BoardPayload", "Service.Collect"},
			Claims:     []string{capDataShape, capAggregateViews},
			Objective:  "Provide JSON data types shared across the dashboard UI.",
			Verdict:    "grounded",
		}},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/board", Role: roleDTO},
			{Path: "internal/dashboard", Role: roleAggregator},
		},
	})
	aligned, claims, issues := alignLedgerToRefinedCatalog(typo, ledger, constraints)
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	byID := map[string][]string{}
	for _, c := range claims.Slices {
		byID[c.ID] = c.Claims
	}
	if got := byID["board"]; len(got) != 1 || got[0] != capDataShape {
		t.Fatalf("board claims=%v want [data_shape]; aligned=%+v", got, aligned)
	}
	if got := byID["dashboard"]; len(got) != 1 || got[0] != capAggregateViews {
		t.Fatalf("dashboard claims=%v want [aggregate_views]; aligned=%+v", got, aligned)
	}
	claimIssues := appendConstraintClaimIssues(typo, constraints, claims, nil)
	if len(claimIssues) != 0 {
		t.Fatalf("claim issues after filter: %v", claimIssues)
	}
}

func TestAlignLedgerToRefinedCatalogFallsBackToOwnedIs(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID:        "judge-eval",
			Objective: "The review slice executes processes.",
			Owns: []catalog.Component{
				{Path: "internal/judge/evaluation/summary"},
			},
		}},
	}
	ledger := sliceObjectiveLedgerDoc{
		Slices: []sliceObjectiveLedgerEntry{{
			ID:         "review",
			OwnedPaths: []string{"internal/judge/evaluation/summary", "internal/staging"},
			Evidence:   []string{"Summary"},
			Claims:     []string{capExecProcess, capDataShape},
			Objective:  "The review slice executes processes.",
			Verdict:    "grounded",
		}},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/judge/evaluation/summary", Role: roleEntrypoint},
		},
	})
	aligned, claims, issues := alignLedgerToRefinedCatalog(typo, ledger, constraints)
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	if len(aligned.Slices) != 1 {
		t.Fatalf("aligned=%+v", aligned)
	}
	if len(claims.Slices[0].Claims) == 0 {
		t.Fatalf("expected owned-is fallback claims, got empty; aligned=%+v constraints is from entrypoint", aligned)
	}
}

func TestAlignLedgerToRefinedCatalogMarksUnclaimedWhenIsEmpty(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID:        "prune-analyzer",
			Objective: "Investigates local-only branches.",
			Owns:      []catalog.Component{{ID: "p", Path: "internal/pruneagent"}},
		}},
	}
	ledger := sliceObjectiveLedgerDoc{
		Slices: []sliceObjectiveLedgerEntry{{
			ID:         "pruneagent",
			OwnedPaths: []string{"internal/pruneagent"},
			Evidence:   []string{"Service.Investigate"},
			Claims:     []string{capAdaptExternal},
			Objective:  "Investigates local-only branches.",
			Verdict:    "grounded",
			Source:     ledgerSourceRLM,
		}},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/pruneagent", Role: roleUnknown}},
	})
	aligned, claims, issues := alignLedgerToRefinedCatalog(typo, ledger, constraints)
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	if len(aligned.Slices) != 1 || aligned.Slices[0].ID != "prune-analyzer" {
		t.Fatalf("aligned=%+v", aligned)
	}
	if len(aligned.Slices[0].Claims) != 0 || aligned.Slices[0].Source != ledgerSourceUnclaimed {
		t.Fatalf("want unclaimed empty claims, got %+v", aligned.Slices[0])
	}
	if len(claims.Slices) != 1 || len(claims.Slices[0].Claims) != 0 {
		t.Fatalf("claims=%+v", claims)
	}
	if err := validateObjectiveLedgerDoc(aligned); err != nil {
		t.Fatal(err)
	}
}

func TestAppendConstraintClaimIssuesAllowsMixedOwnedIs(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID:        "ui",
			Objective: "Shared shapes and aggregated dashboard views.",
			Owns: []catalog.Component{
				{ID: "board", Path: "internal/board"},
				{ID: "dashboard", Path: "internal/dashboard"},
			},
		}},
	}
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{
			{Path: "internal/board", Role: roleDTO},
			{Path: "internal/dashboard", Role: roleAggregator},
		},
	})
	claims := sliceObjectiveClaimsDoc{
		Slices: []sliceObjectiveClaim{{ID: "ui", Claims: []string{capDataShape, capAggregateViews}}},
	}
	if issues := appendConstraintClaimIssues(typo, constraints, claims, nil); len(issues) != 0 {
		t.Fatalf("mixed owns should allow each package is=[] claim, got %v", issues)
	}
}

func writeTestPackageRoles(t *testing.T, evidenceDir string, roles packageRolesDoc) {
	t.Helper()
	data, err := yaml.Marshal(&roles)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, packageRolesRel), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildSliceObjectiveLedgerOverclaim(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(`# Package RLM context index

## ./internal/board

package board

type BoardPayload struct {
  ID string `+"`json:\"id\"`"+`
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/board", Role: roleDTO, Evidence: []string{"json_tags"}}},
	}
	writeTestPackageRoles(t, evidence, roles)
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID: "board", Owns: []catalog.Component{{ID: "b", Path: "internal/board"}},
		}},
	}
	constraints := buildCapabilityConstraints(roles)
	_, issues, err := buildSliceObjectiveLedger(context.Background(), stubSliceLedgerCaller{
		answer: "verdict: overclaim\nreason: prestige\n",
	}, sliceLedgerBuildRequest{
		AnalysisDir: dir, EvidenceDir: evidence, DraftTypo: typo, Constraints: constraints,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0], "overclaim") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestBuildSliceObjectiveLedgerAcceptsUnclaimedUnknown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(`# Package RLM context index

## ./internal/pruneagent

package pruneagent
`), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/pruneagent", Role: roleUnknown}},
	}
	writeTestPackageRoles(t, evidence, roles)
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID: "pruneagent", Owns: []catalog.Component{{ID: "p", Path: "internal/pruneagent"}},
		}},
	}
	constraints := buildCapabilityConstraints(roles)
	doc, issues, err := buildSliceObjectiveLedger(context.Background(), stubSliceLedgerCaller{
		answer: "evidence: Service.Investigate\nclaims: none\nobjective: Investigates local-only branches.\nverdict: grounded\n",
	}, sliceLedgerBuildRequest{
		AnalysisDir: dir, EvidenceDir: evidence, DraftTypo: typo, Constraints: constraints,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	if len(doc.Slices) != 1 || len(doc.Slices[0].Claims) != 0 {
		t.Fatalf("doc=%+v", doc)
	}
	if doc.Slices[0].Source != ledgerSourceUnclaimed {
		t.Fatalf("source=%q", doc.Slices[0].Source)
	}
}

func TestBuildSliceObjectiveLedgerGrounded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(`# Package RLM context index

## ./internal/board

package board

type BoardPayload struct{}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/board", Role: roleDTO, Evidence: []string{"json_tags"}}},
	}
	writeTestPackageRoles(t, evidence, roles)
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID: "board", Owns: []catalog.Component{{ID: "b", Path: "internal/board"}},
		}},
	}
	constraints := buildCapabilityConstraints(roles)
	doc, issues, err := buildSliceObjectiveLedger(context.Background(), stubSliceLedgerCaller{
		answer: "evidence: BoardPayload\nclaims: data_shape\nobjective: Shared board payload shapes.\nverdict: grounded\n",
	}, sliceLedgerBuildRequest{
		AnalysisDir: dir, EvidenceDir: evidence, DraftTypo: typo, Constraints: constraints,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	if len(doc.Slices) != 1 || doc.Slices[0].Objective != "Shared board payload shapes." {
		t.Fatalf("doc=%+v", doc)
	}
}

func TestBuildSliceObjectiveLedgerRejectsUnentailedFillDTO(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(`# Package RLM context index

## ./internal/board

package board

type BoardPayload struct{}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/board", Role: roleDTO, Evidence: []string{"json_tags"}}},
	}
	writeTestPackageRoles(t, evidence, roles)
	typo := catalog.Typology{
		Slices: []catalog.Slice{{
			ID: "board", Owns: []catalog.Component{{ID: "b", Path: "internal/board"}},
		}},
	}
	constraints := buildCapabilityConstraints(roles)
	_, issues, err := buildSliceObjectiveLedger(context.Background(), stubSliceLedgerCaller{
		answer: "evidence: BoardPayload\nclaims: fill_dto\nobjective: Board fills DTOs.\nverdict: grounded\n",
	}, sliceLedgerBuildRequest{
		AnalysisDir: dir, EvidenceDir: evidence, DraftTypo: typo, Constraints: constraints,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("expected fill_dto rejection")
	}
	joined := strings.Join(issues, "\n")
	if !strings.Contains(joined, "fill_dto") {
		t.Fatalf("issues=%v", issues)
	}
	// Fail-closed is=[] / must_not may reject before positive entailment.
	if !strings.Contains(joined, "not entailed") &&
		!strings.Contains(joined, "intersect must_not") &&
		!strings.Contains(joined, "not allowed by owned package is=") {
		t.Fatalf("issues=%v", issues)
	}
}

type stubLedgerBuilder struct {
	doc    sliceObjectiveLedgerDoc
	issues []string
	err    error
}

func (s stubLedgerBuilder) BuildSliceLedger(context.Context, sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error) {
	return s.doc, s.issues, s.err
}
