package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/strop/pkg/evaluation"
)

type stubJudgeGen struct {
	mergeIDs           []string
	mergePackages      []string
	mergeIntents       []string
	proposedMergesYAML string
	refined            string
	ledgerNeedle       string // when set, refine Generate requires this substring in the ledger field
	calls              int
}

func (s *stubJudgeGen) clusterOut() map[string]interface{} {
	ids, pkgs, intents := append([]string(nil), s.mergeIDs...), append([]string(nil), s.mergePackages...), append([]string(nil), s.mergeIntents...)
	if len(ids) == 0 && strings.TrimSpace(s.proposedMergesYAML) != "" {
		merges, err := parseProposedMergesYAML(s.proposedMergesYAML)
		if err == nil {
			for _, m := range merges {
				ids = append(ids, m.ID)
				pkgs = append(pkgs, strings.Join(m.Packages, ","))
				intents = append(intents, m.Intent)
			}
		}
	}
	if len(ids) == 0 {
		return map[string]interface{}{
			"merge_ids": "none", "merge_packages": "none", "merge_intents": "none",
		}
	}
	return map[string]interface{}{
		"merge_ids":      strings.Join(ids, ","),
		"merge_packages": strings.Join(pkgs, ";"),
		"merge_intents":  strings.Join(intents, ","),
	}
}

func (s *stubJudgeGen) Generate(_ context.Context, task string, fields map[string]interface{}, _ int) (map[string]interface{}, error) {
	s.calls++
	switch task {
	case jmodules.TaskTypologyCluster:
		return s.clusterOut(), nil
	case jmodules.TaskTypologyRefine:
		if needle := strings.TrimSpace(s.ledgerNeedle); needle != "" {
			ledger, ok := fields["slice_objective_ledger_yaml"].(string)
			if !ok || !strings.Contains(ledger, needle) {
				return nil, context.Canceled // force visible failure if ledger missing
			}
		}
		return map[string]interface{}{
			"refined_catalog_yaml": s.refined,
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
		refined:      refined,
		ledgerNeedle: "Shared board payload shapes",
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

func TestJudgeTypologyRefineFailsClosedOnResidualLedgerIssues(t *testing.T) {
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
	once := &flippingLedgerBuilder{}
	stub := &stubJudgeGen{
		refined: `id: demo
slices:
  - id: board
    objective: Shared board payload shapes.
    owns:
      - id: board-core
        path: internal/board
`,
	}
	_, err := (JudgeTypologyRefineGenerator{Gen: stub}).Refine(context.Background(), TypologyRefineInput{
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
		LedgerBuilder:     once,
	})
	if err == nil {
		t.Fatal("expected residual ledger issues to fail Refine")
	}
	if !strings.Contains(err.Error(), "objective overclaims") {
		t.Fatalf("err=%v", err)
	}
	// Ledger memory retries live inside BuildSliceLedger; Refine calls the builder once.
	if once.calls != 1 {
		t.Fatalf("BuildSliceLedger calls=%d want 1", once.calls)
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
