package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/strop/pkg/evaluation"
	"github.com/behaviorengineering/typology/pkg/catalog"
)

type stubJudgeGen struct {
	mergeIDs           []string
	mergePackages      []string
	mergeIntents       []string
	proposedMergesYAML string
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

func (s *stubJudgeGen) Generate(_ context.Context, task string, _ map[string]interface{}, _ int) (map[string]interface{}, error) {
	s.calls++
	switch task {
	case jmodules.TaskTypologySliceGrouping:
		return s.clusterOut(), nil
	default:
		return map[string]interface{}{}, nil
	}
}

func (s *stubJudgeGen) Evaluate(context.Context, string, map[string]interface{}, map[string]interface{}, int) (*evaluation.AggregatedEvaluation, error) {
	return &evaluation.AggregatedEvaluation{WeightedScore: 10.0}, nil
}

func (s *stubJudgeGen) Ready() bool             { return true }
func (s *stubJudgeGen) TaskModel(string) string { return "stub" }

// stubCatalogAssembler emits one fragment per owned folded slice, copying ledger objectives.
type stubCatalogAssembler struct {
	override map[string]catalog.Slice
	issues   []string
	err      error
}

func (s stubCatalogAssembler) AssembleSlices(_ context.Context, req sliceCatalogAssembleRequest) (sliceCatalogAssembleResult, error) {
	if s.err != nil {
		return sliceCatalogAssembleResult{}, s.err
	}
	if len(s.issues) > 0 {
		return sliceCatalogAssembleResult{Issues: append([]string(nil), s.issues...)}, nil
	}
	ledgerObj := map[string]string{}
	for _, e := range req.LedgerDoc.Slices {
		if id := strings.TrimSpace(e.ID); id != "" {
			ledgerObj[id] = strings.TrimSpace(e.Objective)
		}
	}
	var frags []catalog.Slice
	for _, t := range catalogAssembleTargets(req.FoldedTypo) {
		if o, ok := s.override[t.id]; ok {
			frags = append(frags, o)
			continue
		}
		frag := t.slice
		if obj := ledgerObj[t.id]; obj != "" {
			frag.Objective = obj
		}
		frags = append(frags, frag)
	}
	return sliceCatalogAssembleResult{Fragments: frags}, nil
}

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
	stub := &stubJudgeGen{}
	ledger := stubLedgerBuilder{
		doc: sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{{
			ID: "board", OwnedPaths: []string{"internal/board"},
			Evidence: []string{"BoardPayload"}, Claims: []string{capDataShape},
			Objective: "Shared board payload shapes.", Verdict: "grounded",
		}}},
	}
	out, err := (JudgeTypologySlicePipeline{Gen: stub}).Assemble(context.Background(), TypologySlicePipelineInput{
		RepoID: "demo", ModuleScope: ".",
		DraftCatalogYAML:      draft,
		PackageRoles:          roles,
		CapabilityConstraints: constraints,
		ArchitectureDraft:     "# Draft\n",
		AnalysisDir:           dir,
		EvidenceDir:           evidence,
		LedgerBuilder:         ledger,
		CatalogAssembler:      stubCatalogAssembler{},
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
	stub := &stubJudgeGen{}
	_, err := (JudgeTypologySlicePipeline{Gen: stub}).Assemble(context.Background(), TypologySlicePipelineInput{
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
		CatalogAssembler:  stubCatalogAssembler{},
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
