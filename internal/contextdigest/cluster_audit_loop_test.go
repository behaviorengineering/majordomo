package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/strop/evaluation"
	"github.com/behaviorengineering/typology/catalog"
)

func TestParseClusterAuditAnswerThemeRejects(t *testing.T) {
	proposed := []proposedMerge{
		{ID: "git", Packages: []string{"internal/localgit", "internal/remotegit"}, Intent: mergeIntentSlice},
		{ID: "analysis", Packages: []string{"internal/pruneagent", "internal/triage"}, Intent: mergeIntentSlice},
	}
	answer := `
id: git
verdict: overlay
reason: theme word only; different callers
evidence: localgit.Clone; remotegit.Fetch

id: analysis
verdict: reject
reason: theme analysis word; separate jobs
evidence: pruneagent.Prune; triage.Score
`
	got, err := parseClusterAuditAnswer(answer, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Verdict != verdictOverlay || got[1].Verdict != verdictReject {
		t.Fatalf("verdicts=%+v", got)
	}
}

func TestParseClusterAuditAnswerAcceptNeedsEvidence(t *testing.T) {
	proposed := []proposedMerge{{ID: "x", Packages: []string{"a", "b"}, Intent: mergeIntentSlice}}
	got, err := parseClusterAuditAnswer("id: x\nverdict: accept\nreason: ok\n", proposed)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != verdictReject {
		t.Fatalf("want reject without evidence, got %+v", got[0])
	}
}

func TestRunClusterMergeAuditEmptySkipsCaller(t *testing.T) {
	caller := &stubClusterAuditCaller{err: context.Canceled}
	out, err := runClusterMergeAudit(context.Background(), caller, clusterAuditRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 0 {
		t.Fatalf("verdicts=%+v", out.Verdicts)
	}
}

type stubClusterAuditCaller struct {
	answer string
	err    error
	calls  int
}

func (s *stubClusterAuditCaller) Complete(context.Context, any, string) (string, int, int, int, int, error) {
	s.calls++
	return s.answer, 1, 10, 5, 15, s.err
}

func (s *stubClusterAuditCaller) TraceDir() string { return "/tmp/rlm-traces/test" }

type stubClusterAuditor struct {
	result clusterAuditResult
	err    error
	calls  int
}

func (s *stubClusterAuditor) Audit(_ context.Context, req clusterAuditRequest) (clusterAuditResult, error) {
	s.calls++
	if s.err != nil {
		return clusterAuditResult{}, s.err
	}
	out := s.result
	if len(out.Verdicts) == 0 {
		for _, p := range req.Proposed {
			out.Verdicts = append(out.Verdicts, clusterMergeVerdict{
				ID: p.ID, Packages: p.Packages, Verdict: verdictOverlay, Reason: "theme", Evidence: []string{"sym"},
			})
		}
		out.Verdicts = append(out.Verdicts, req.Frozen...)
	}
	return out, nil
}

func TestJudgeTypologyRefineClusterAuditDemotesThemeMerges(t *testing.T) {
	t.Parallel()
	roles := `packages:
  - path: internal/localgit
    role: adapter
    confidence: 0.9
    inspected_stage: 2
  - path: internal/remotegit
    role: adapter
    confidence: 0.9
    inspected_stage: 2
`
	draft := `id: demo
slices:
  - id: localgit
    objective: Local forge adapter.
    owns:
      - path: internal/localgit
  - id: remotegit
    objective: Remote forge adapter.
    owns:
      - path: internal/remotegit
`
	refined := draft
	stub := &stubJudgeGen{
		clusterMD: `# Cluster

## Proposed merges (machine)
- id: git
  packages: [internal/localgit, internal/remotegit]
  intent: slice

## Capability constraints (is / is-not)

- ` + "`internal/localgit`" + ` role=adapter is=[] must_not=[]
- ` + "`internal/remotegit`" + ` role=adapter is=[] must_not=[]
`,
		refined: refined,
		journey: "# Journey\n\n## Status\n\nDraft.\n\n## Technical debt and boundary violations\n\nNone.\n",
	}
	auditor := &stubClusterAuditor{}
	out, err := (JudgeTypologyRefineGenerator{Gen: stub}).Refine(context.Background(), TypologyRefineInput{
		RepoID:            "demo",
		ModuleScope:       ".",
		DraftCatalogYAML:  draft,
		PackageRoles:      roles,
		ArchitectureDraft: "# Draft\n",
		ClusterAuditor:    auditor,
		LedgerBuilder: stubLedgerBuilder{doc: sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{
			{ID: "localgit", OwnedPaths: []string{"internal/localgit"}, Evidence: []string{"Clone"}, Claims: []string{capAdaptExternal}, Objective: "Local forge adapter.", Verdict: "grounded"},
			{ID: "remotegit", OwnedPaths: []string{"internal/remotegit"}, Evidence: []string{"Fetch"}, Claims: []string{capAdaptExternal}, Objective: "Remote forge adapter.", Verdict: "grounded"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if auditor.calls != 1 {
		t.Fatalf("auditor calls=%d", auditor.calls)
	}
	merges, err := parseProposedMergesMachine(out.ClusterProposalMD)
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 1 || merges[0].Intent != mergeIntentNickname {
		t.Fatalf("demoted merges=%+v", merges)
	}
	if !strings.Contains(out.ClusterMergeVerdictsYAML, "overlay") {
		t.Fatalf("verdicts=%q", out.ClusterMergeVerdictsYAML)
	}
}

func TestJudgeTypologyRefineStickyRenameSkipsSecondAudit(t *testing.T) {
	t.Parallel()
	roles := `packages:
  - path: internal/a
    role: adapter
    confidence: 0.9
    inspected_stage: 2
  - path: internal/b
    role: adapter
    confidence: 0.9
    inspected_stage: 2
  - path: internal/c
    role: dto
    confidence: 0.9
    inspected_stage: 2
`
	draft := `id: demo
slices:
  - id: a
    objective: A.
    owns: [{path: internal/a}]
  - id: b
    objective: B.
    owns: [{path: internal/b}]
  - id: c
    objective: C.
    owns: [{path: internal/c}]
`
	flip := &flippingClusterJudge{
		first: `# Cluster
## Proposed merges (machine)
- id: git
  packages: [internal/a, internal/b]
  intent: slice
## Capability constraints (is / is-not)
- ` + "`internal/a`" + `
- ` + "`internal/b`" + `
- ` + "`internal/c`" + `
`,
		second: `# Cluster
## Proposed merges (machine)
- id: forge
  packages: [internal/b, internal/a]
  intent: slice
- id: keep
  packages: [internal/c]
  intent: nickname
## Capability constraints (is / is-not)
- ` + "`internal/a`" + `
- ` + "`internal/b`" + `
- ` + "`internal/c`" + `
`,
		refined: draft,
		journey: "# Journey\n\n## Status\n\nDraft.\n\n## Technical debt and boundary violations\n\nNone.\n",
	}
	auditor := &countingRejectAuditor{}
	_, err := (JudgeTypologyRefineGenerator{Gen: flip}).Refine(context.Background(), TypologyRefineInput{
		RepoID:            "demo",
		ModuleScope:       ".",
		DraftCatalogYAML:  draft,
		PackageRoles:      roles,
		ArchitectureDraft: "# Draft\n",
		ClusterAuditor:    auditor,
		LedgerBuilder: stubLedgerBuilder{doc: sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{
			{ID: "a", OwnedPaths: []string{"internal/a"}, Evidence: []string{"A"}, Claims: []string{capAdaptExternal}, Objective: "A.", Verdict: "grounded"},
			{ID: "b", OwnedPaths: []string{"internal/b"}, Evidence: []string{"B"}, Claims: []string{capAdaptExternal}, Objective: "B.", Verdict: "grounded"},
			{ID: "c", OwnedPaths: []string{"internal/c"}, Evidence: []string{"C"}, Claims: []string{capDataShape}, Objective: "C.", Verdict: "grounded"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if auditor.calls != 1 {
		t.Fatalf("expected one full-list audit (sticky rename), got %d", auditor.calls)
	}
}

type flippingClusterJudge struct {
	first, second, refined, journey string
	clusterCalls                    int
}

func (s *flippingClusterJudge) Generate(_ context.Context, task string, _ map[string]interface{}, _ int) (map[string]interface{}, error) {
	switch task {
	case jmodules.TaskTypologyCluster:
		s.clusterCalls++
		md := s.first
		if s.clusterCalls > 1 {
			md = s.second
		}
		return map[string]interface{}{"cluster_proposal_md": md}, nil
	case jmodules.TaskTypologyRefine:
		return map[string]interface{}{"refined_catalog_yaml": s.refined, "journey_md": s.journey}, nil
	default:
		return map[string]interface{}{}, nil
	}
}

func (s *flippingClusterJudge) Evaluate(context.Context, string, map[string]interface{}, map[string]interface{}, int) (*evaluation.AggregatedEvaluation, error) {
	return &evaluation.AggregatedEvaluation{WeightedScore: 10.0}, nil
}
func (s *flippingClusterJudge) Ready() bool             { return true }
func (s *flippingClusterJudge) TaskModel(string) string { return "stub" }

type countingRejectAuditor struct{ calls int }

func (s *countingRejectAuditor) Audit(_ context.Context, req clusterAuditRequest) (clusterAuditResult, error) {
	s.calls++
	var out []clusterMergeVerdict
	for _, p := range req.Proposed {
		out = append(out, clusterMergeVerdict{
			ID: p.ID, Packages: p.Packages, Verdict: verdictReject, Reason: "theme", Evidence: []string{"e"},
		})
	}
	out = append(out, req.Frozen...)
	return clusterAuditResult{Verdicts: out, TraceDir: "tmp/rlm-traces/typology_cluster_audit"}, nil
}

func TestAssertAcceptedMembershipBlocksLibraryThemeFold(t *testing.T) {
	draft := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "a", Owns: []catalog.Component{{Path: "internal/a"}}},
			{ID: "b", Owns: []catalog.Component{{Path: "internal/b"}}},
		},
	}
	refined := catalog.Typology{
		ID: "demo",
		Libraries: []catalog.Library{
			{ID: "git", Owns: []catalog.Component{{Path: "internal/a"}, {Path: "internal/b"}}},
		},
	}
	issues := assertAcceptedMembership(refined, draft, nil)
	if len(issues) == 0 {
		t.Fatal("expected membership issues")
	}
	split, changed := splitIllegalMembershipToDraftOwners(refined, draft, nil)
	if !changed {
		t.Fatal("expected split")
	}
	if len(split.Libraries) > 0 && len(libraryPackagePaths(split.Libraries[0])) >= 2 {
		t.Fatalf("library still co-owns: %+v", split.Libraries)
	}
}

func TestRLMTraceDirCreatesScratch(t *testing.T) {
	base := t.TempDir()
	dir := rlmTraceDir(base, jmodules.TaskTypologyClusterAudit)
	marker := filepath.Join(dir, "step.jsonl")
	if err := os.WriteFile(marker, []byte(`{"step":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dir, "rlm-traces") {
		t.Fatalf("dir=%q", dir)
	}
}
