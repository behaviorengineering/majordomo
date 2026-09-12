package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
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
		t.Fatal("expected must_not intersection")
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
	_, _, issues := alignLedgerToRefinedCatalog(typo, ledger)
	if len(issues) != 1 || !strings.Contains(issues[0], "does not match any contributing ledger objective") {
		t.Fatalf("issues=%v", issues)
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
	aligned, claims, issues := alignLedgerToRefinedCatalog(typo, ledger)
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
	// Fail-closed must_not may reject before positive entailment.
	if !strings.Contains(joined, "not entailed") && !strings.Contains(joined, "intersect must_not") {
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
