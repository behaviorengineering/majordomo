package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
)

type countingLedgerCaller struct {
	mu    sync.Mutex
	calls []string // queries in order
}

func (c *countingLedgerCaller) Complete(_ context.Context, _ any, query string) (string, int, error) {
	c.mu.Lock()
	c.calls = append(c.calls, query)
	n := len(c.calls)
	c.mu.Unlock()

	sliceID := "board"
	if strings.Contains(query, `Slice id: triage`) {
		sliceID = "triage"
	}
	hasFeedback := strings.Contains(query, "validation_feedback (from a prior failed attempt")

	switch sliceID {
	case "board":
		return "evidence: Row, json_tags\nclaims: data_shape\nobjective: Shared board payload shapes.\nverdict: grounded\n", 1, nil
	case "triage":
		if !hasFeedback {
			// First attempt: overclaim orchestrate (not entailed for unknown role).
			return "evidence: Analyzer.Analyze\nclaims: orchestrate\nobjective: Triage orchestrates analysis.\nverdict: grounded\n", 1, nil
		}
		// Retry with feedback: drop orchestrate.
		return "evidence: Analyzer.Analyze, Response\nclaims: adapt_external\nobjective: Triage analyzes CI log text into a structured response.\nverdict: grounded\n", n, nil
	default:
		return "evidence: x\nclaims: config\nobjective: x\nverdict: grounded\n", 1, nil
	}
}

func TestBuildSliceObjectiveLedger_keepsSuccessRetriesFailureWithFeedback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	roles := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
  - path: internal/triage
    role: adapter
    confidence: 0.8
    evidence: [exported_funcs]
    inspected_stage: 2
edges: []
`
	if err := os.WriteFile(filepath.Join(evidence, packageRolesRel), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	rlm := `# Package RLM context index

## ./internal/board
- package: board
- jsonTags: true
- mechanicalRole: dto

## ./internal/triage
- package: triage
- mechanicalRole: adapter
- exportedFuncs: New
- exportedMethods: Analyzer.Analyze
`
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(rlm), 0o644); err != nil {
		t.Fatal(err)
	}

	draft := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "board", Owns: []catalog.Component{{ID: "b", Path: "internal/board"}}},
			{ID: "triage", Owns: []catalog.Component{{ID: "t", Path: "internal/triage"}}},
		},
	}
	rolesDoc, err := loadPackageRoles(filepath.Join(evidence, packageRolesRel))
	if err != nil {
		t.Fatal(err)
	}
	constraints := buildCapabilityConstraints(rolesDoc)
	caller := &countingLedgerCaller{}

	doc, issues, err := buildSliceObjectiveLedger(context.Background(), caller, sliceLedgerBuildRequest{
		AnalysisDir: dir,
		EvidenceDir: evidence,
		DraftTypo:   draft,
		Constraints: constraints,
		ClusterMD:   "# cluster\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	if len(doc.Slices) != 2 {
		t.Fatalf("slices=%d want 2: %+v", len(doc.Slices), doc.Slices)
	}

	caller.mu.Lock()
	calls := append([]string(nil), caller.calls...)
	caller.mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("Complete calls=%d want 3 (board once, triage fail, triage retry); queries:\n%s",
			len(calls), strings.Join(calls, "\n---\n"))
	}
	feedbackCalls := 0
	for _, q := range calls {
		if strings.Contains(q, "validation_feedback (from a prior failed attempt") {
			feedbackCalls++
			if !strings.Contains(q, "orchestrate") {
				t.Fatalf("retry feedback missing orchestrate rejection:\n%s", q)
			}
		}
	}
	if feedbackCalls != 1 {
		t.Fatalf("validation_feedback in %d queries want 1", feedbackCalls)
	}

	byID := map[string]sliceObjectiveLedgerEntry{}
	for _, s := range doc.Slices {
		byID[s.ID] = s
	}
	if got := strings.Join(byID["board"].Claims, ","); got != "data_shape" {
		t.Fatalf("board claims=%q", got)
	}
	if got := strings.Join(byID["triage"].Claims, ","); got != "adapt_external" {
		t.Fatalf("triage claims=%q (retry should have dropped orchestrate)", got)
	}
}

func TestFilterIssuesForSlice(t *testing.T) {
	t.Parallel()
	issues := []string{
		`majordomo_typology_role_grounding: slice "triage" claim "orchestrate" is not entailed`,
		`majordomo_typology_role_grounding: slice "pruneagent" claim "orchestrate" is not entailed`,
	}
	got := filterIssuesForSlice(issues, "triage")
	if !strings.Contains(got, "triage") || strings.Contains(got, "pruneagent") {
		t.Fatalf("got=%q", got)
	}
}

func TestFormatSliceObjectiveLedgerQueryIncludesClaimPolicy(t *testing.T) {
	t.Parallel()
	constraints := buildCapabilityConstraints(packageRolesDoc{
		Packages: []packageRoleNode{{Path: "internal/pruneagent", Role: roleAggregator}},
	})
	byPath := constraintsByPath(constraints)
	block := formatConstraintRowsForPaths([]string{"internal/pruneagent"}, byPath)
	q := formatSliceObjectiveLedgerQuery("pruneagent", []string{"internal/pruneagent"}, block, "# cluster\n", "")
	if !strings.Contains(q, "Claim policy (deterministic; MUST follow)") {
		t.Fatalf("missing claim policy fragment:\n%s", q)
	}
	if !strings.Contains(q, "imports_os_exec") || !strings.Contains(q, "fills_dto") {
		t.Fatalf("policy missing entailment hints:\n%s", q)
	}
	if !strings.Contains(q, "internal/pruneagent") || !strings.Contains(q, "must_not=") {
		t.Fatalf("missing constraint rows:\n%s", q)
	}
}
