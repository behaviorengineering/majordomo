package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/strop/evaluation"
)

type stubJudgeGen struct {
	clusterMD string
	refined   string
	journey   string
	calls     int
}

func (s *stubJudgeGen) Generate(_ context.Context, task string, fields map[string]interface{}, _ int) (map[string]interface{}, error) {
	s.calls++
	switch task {
	case jmodules.TaskTypologyCluster:
		return map[string]interface{}{"cluster_proposal_md": s.clusterMD}, nil
	case jmodules.TaskTypologyRefine:
		if ledger, _ := fields["slice_objective_ledger_yaml"].(string); !strings.Contains(ledger, "Shared board payload shapes") {
			return nil, context.Canceled // force visible failure if ledger missing
		}
		return map[string]interface{}{
			"refined_catalog_yaml": s.refined,
			"journey_md":           s.journey,
		}, nil
	default:
		return map[string]interface{}{}, nil
	}
}

func (s *stubJudgeGen) Evaluate(context.Context, string, map[string]interface{}, map[string]interface{}, int) (*evaluation.AggregatedEvaluation, error) {
	return &evaluation.AggregatedEvaluation{WeightedScore: 10.0}, nil
}

func (s *stubJudgeGen) Ready() bool             { return true }
func (s *stubJudgeGen) TaskModel(string) string { return "stub" }

func TestJudgeTypologyRefineUsesLedgerObjectives(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte("## ./internal/board\n\ntype BoardPayload struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	draft := `id: demo
slices:
  - id: board
    objective: placeholder
    owns:
      - id: board-core
        path: internal/board
`
	refined := `id: demo
slices:
  - id: board
    objective: Shared board payload shapes.
    owns:
      - id: board-core
        path: internal/board
`
	roles := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    inspected_stage: 2
edges:
  - from: internal/adapter
    to: internal/board
    kind: fills_dto
`
	constraints, err := marshalConstraints(buildCapabilityConstraints(mustParseRoles(roles)))
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubJudgeGen{
		clusterMD: "# Cluster\n\nKeep board.\n\n## Capability constraints (is / is-not)\n\n- `internal/board` role=dto is=[data_shape] must_not=[synchronize_state]\n",
		refined:   refined,
		journey:   "# Journey\n\n## Status\n\nDraft.\n\n## Technical debt and boundary violations\n\nNone.\n",
	}
	ledger := stubLedgerBuilder{
		doc: sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{{
			ID: "board", OwnedPaths: []string{"internal/board"},
			Evidence: []string{"BoardPayload"}, Claims: []string{capDataShape},
			Objective: "Shared board payload shapes.", Verdict: "grounded",
		}}},
	}
	out, err := (JudgeTypologyRefineGenerator{Gen: stub}).Refine(context.Background(), TypologyRefineInput{
		RepoID: "demo", ModuleScope: ".",
		DraftCatalogYAML:      draft,
		PackageRoles:          roles,
		CapabilityConstraints: constraints,
		ArchitectureDraft:     "# Draft\n",
		ClusterProposalMD:     "",
		AnalysisDir:           dir,
		EvidenceDir:           evidence,
		LedgerBuilder:         ledger,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.ObjectiveLedgerYAML, "Shared board payload shapes") {
		t.Fatalf("ledger=%q", out.ObjectiveLedgerYAML)
	}
	if !strings.Contains(out.ObjectiveClaimsYAML, "data_shape") {
		t.Fatalf("claims=%q", out.ObjectiveClaimsYAML)
	}
	if !strings.Contains(out.RefinedCatalogYAML, "Shared board payload shapes") {
		t.Fatalf("refined=%q", out.RefinedCatalogYAML)
	}
}

func TestJudgeTypologyRefineRetriesOnLedgerOverclaim(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte("## ./internal/board\n\ntype BoardPayload struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	draft := `id: demo
slices:
  - id: board
    objective: placeholder
    owns:
      - id: board-core
        path: internal/board
`
	roles := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    inspected_stage: 2
`
	flipping := &flippingLedgerBuilder{}
	stub := &stubJudgeGen{
		clusterMD: "# Cluster\n\nKeep board.\n\n## Capability constraints (is / is-not)\n\n- `internal/board`\n",
		refined: `id: demo
slices:
  - id: board
    objective: Shared board payload shapes.
    owns:
      - id: board-core
        path: internal/board
`,
		journey: "# Journey\n\n## Status\n\nOk.\n\n## Technical debt and boundary violations\n\nNone.\n",
	}
	out, err := (JudgeTypologyRefineGenerator{Gen: stub}).Refine(context.Background(), TypologyRefineInput{
		RepoID: "demo", ModuleScope: ".",
		DraftCatalogYAML: draft,
		PackageRoles:     roles,
		CapabilityConstraints: func() string {
			s, err := marshalConstraints(buildCapabilityConstraints(mustParseRoles(roles)))
			if err != nil {
				t.Fatal(err)
			}
			return s
		}(),
		ArchitectureDraft: "# Draft\n",
		AnalysisDir:       dir,
		EvidenceDir:       evidence,
		LedgerBuilder:     flipping,
	})
	if err != nil {
		t.Fatal(err)
	}
	if flipping.calls < 2 {
		t.Fatalf("expected retry after overclaim, calls=%d", flipping.calls)
	}
	if !strings.Contains(out.ObjectiveLedgerYAML, "Shared board payload shapes") {
		t.Fatalf("ledger=%q", out.ObjectiveLedgerYAML)
	}
}

type flippingLedgerBuilder struct {
	calls int
}

func (f *flippingLedgerBuilder) BuildSliceLedger(context.Context, sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error) {
	f.calls++
	if f.calls == 1 {
		return sliceObjectiveLedgerDoc{}, []string{"majordomo_typology_role_grounding: slice \"board\" objective overclaims"}, nil
	}
	return sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{{
		ID: "board", OwnedPaths: []string{"internal/board"},
		Evidence: []string{"BoardPayload"}, Claims: []string{capDataShape},
		Objective: "Shared board payload shapes.", Verdict: "grounded",
	}}}, nil, nil
}
