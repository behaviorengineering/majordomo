package contextdigest

import (
	"context"
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

func TestParseClusterAuditAnswerAcceptMissingPackageEvidenceDemotesOverlay(t *testing.T) {
	proposed := []proposedMerge{
		{ID: "git-adapters", Packages: []string{"internal/localgit", "internal/remotegit"}, Intent: mergeIntentSlice},
	}
	// PR 41 shape: accept with evidence that only cites localgit.
	answer := `id: git-adapters
verdict: accept
reason: Both packages serve as adapters for git operations
evidence:
internal/localgit: rlm:"Inspector.InspectSync", "BranchSync"
`
	got, err := parseClusterAuditAnswer(answer, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != verdictOverlay {
		t.Fatalf("want overlay when one package missing from evidence, got %+v", got[0])
	}
	if !strings.Contains(strings.ToLower(got[0].Reason), "per-package evidence") {
		t.Fatalf("reason=%q", got[0].Reason)
	}
}

func TestParseClusterAuditAnswerAcceptKeepsMultilineEvidence(t *testing.T) {
	proposed := []proposedMerge{
		{ID: "git-adapters", Packages: []string{"internal/localgit", "internal/remotegit"}, Intent: mergeIntentSlice},
		{ID: "git-aggregators", Packages: []string{"internal/dashboard", "internal/pruneagent"}, Intent: mergeIntentSlice},
	}
	// Shape produced by live RLM: evidence: on its own line, quotes on following lines.
	answer := `id: git-adapters
verdict: accept
reason: complementary git adapters
evidence:
internal/localgit: Inspector.InspectPath
internal/remotegit: github.go, gitlab.go
id: git-aggregators
verdict: reject
reason: different lifecycles
evidence:
internal/dashboard: NewMux
`
	got, err := parseClusterAuditAnswer(answer, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != verdictAccept {
		t.Fatalf("git-adapters want accept, got %+v", got[0])
	}
	if len(got[0].Evidence) < 2 {
		t.Fatalf("git-adapters evidence=%v", got[0].Evidence)
	}
	if got[1].Verdict != verdictReject {
		t.Fatalf("git-aggregators want reject, got %+v", got[1])
	}
}

func TestParseClusterAuditAnswerYAMLDecodeFailsClosed(t *testing.T) {
	proposed := []proposedMerge{{ID: "x", Packages: []string{"a", "b"}, Intent: mergeIntentSlice}}
	// Looks like YAML (document key) but is not a valid merges list; must not fall through to line scrape.
	_, err := parseClusterAuditAnswer("merges:\n  - id: [unterminated\n", proposed)
	if err == nil {
		t.Fatal("expected YAML decode error")
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
		proposedMergesYAML: `- id: git
  packages: [internal/localgit, internal/remotegit]
  intent: slice
`,
		refined: refined,
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
	merges, err := parseProposedMergesYAML(out.ClusterMergeProposalYAML)
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
		firstYAML: `- id: git
  packages: [internal/a, internal/b]
  intent: slice
`,
		secondYAML: `- id: forge
  packages: [internal/b, internal/a]
  intent: slice
- id: keep
  packages: [internal/c]
  intent: nickname
`,
		refined: draft,
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
	firstYAML, secondYAML, refined string
	clusterCalls                   int
}

func (s *flippingClusterJudge) clusterLists(yaml string) map[string]interface{} {
	merges, err := parseProposedMergesYAML(yaml)
	if err != nil {
		return map[string]interface{}{
			"merge_ids": "none", "merge_packages": "none", "merge_intents": "none",
		}
	}
	if len(merges) == 0 {
		return map[string]interface{}{
			"merge_ids": "none", "merge_packages": "none", "merge_intents": "none",
		}
	}
	ids := make([]string, 0, len(merges))
	pkgs := make([]string, 0, len(merges))
	intents := make([]string, 0, len(merges))
	for _, m := range merges {
		ids = append(ids, m.ID)
		pkgs = append(pkgs, strings.Join(m.Packages, ","))
		intents = append(intents, m.Intent)
	}
	return map[string]interface{}{
		"merge_ids":      strings.Join(ids, ","),
		"merge_packages": strings.Join(pkgs, ";"),
		"merge_intents":  strings.Join(intents, ","),
	}
}

func (s *flippingClusterJudge) Generate(_ context.Context, task string, _ map[string]interface{}, _ int) (map[string]interface{}, error) {
	switch task {
	case jmodules.TaskTypologyCluster:
		s.clusterCalls++
		yaml := s.firstYAML
		if s.clusterCalls > 1 {
			yaml = s.secondYAML
		}
		return s.clusterLists(yaml), nil
	case jmodules.TaskTypologyRefine:
		return map[string]interface{}{"refined_catalog_yaml": s.refined}, nil
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
