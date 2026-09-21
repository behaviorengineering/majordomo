package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/behaviorengineering/typology/pkg/catalog"
)

type countingLedgerCaller struct {
	mu    sync.Mutex
	calls []string // queries in order
}

func (c *countingLedgerCaller) Complete(_ context.Context, _ any, query string) (string, int, int, int, int, error) {
	c.mu.Lock()
	c.calls = append(c.calls, query)
	n := len(c.calls)
	c.mu.Unlock()

	if strings.Contains(query, "extract grounding evidence for one package") {
		pkg := "pkg"
		switch {
		case strings.Contains(query, "internal/board"):
			pkg = "board"
		case strings.Contains(query, "internal/triage"):
			pkg = "triage"
		}
		return fmt.Sprintf("evidence:\n  - %sSymbol\nnotes: %s package notes\n", pkg, pkg), 1, 10, 5, 15, nil
	}

	sliceID := "board"
	if strings.Contains(query, `Slice id: triage`) {
		sliceID = "triage"
	}
	hasFeedback := strings.Contains(query, "validation_feedback (from a prior failed attempt")

	switch sliceID {
	case "board":
		return "evidence: Row, json_tags\nclaims: data_shape\nobjective: Shared board payload shapes.\nverdict: grounded\n", 1, 200, 40, 240, nil
	case "triage":
		if !hasFeedback {
			// First synthesis: overclaim orchestrate (not entailed for adapter role).
			return "evidence: Analyzer.Analyze\nclaims: orchestrate\nobjective: Triage orchestrates analysis.\nverdict: grounded\n", 1, 0, 0, 0, nil
		}
		// Retry with feedback: drop orchestrate.
		return "evidence: Analyzer.Analyze, Response\nclaims: adapt_external\nobjective: Triage analyzes CI log text into a structured response.\nverdict: grounded\n", n, 0, 0, 0, nil
	default:
		return "evidence: x\nclaims: config\nobjective: x\nverdict: grounded\n", 1, 0, 0, 0, nil
	}
}

func TestBuildSliceObjectiveLedger_keepsSuccessRetriesFailureWithFeedback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, p := range []string{"internal/board", "internal/triage"} {
		pkg := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(pkg, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkg, "x.go"), []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
		AnalysisDir:     dir,
		EvidenceDir:     evidence,
		DraftTypo:       draft,
		Constraints:     constraints,
		ClusterHintYAML: "# cluster\n",
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
	evidenceCalls := 0
	synthesisCalls := 0
	feedbackCalls := 0
	for _, q := range calls {
		if strings.Contains(q, "extract grounding evidence for one package") {
			evidenceCalls++
			continue
		}
		synthesisCalls++
		if strings.Contains(q, "validation_feedback (from a prior failed attempt") {
			feedbackCalls++
			if !strings.Contains(q, "orchestrate") {
				t.Fatalf("retry feedback missing orchestrate rejection:\n%s", q)
			}
		}
	}
	// Mechanical evidence skips RLM when the package snippet already has symbols.
	// board: 1 synthesis; triage: fail synthesis + retry synthesis
	if evidenceCalls != 0 || synthesisCalls != 3 {
		t.Fatalf("Complete calls evidence=%d synthesis=%d want 0/3; queries:\n%s",
			evidenceCalls, synthesisCalls, strings.Join(calls, "\n---\n"))
	}
	if feedbackCalls != 1 {
		t.Fatalf("validation_feedback in %d queries want 1", feedbackCalls)
	}

	planPath := filepath.Join(dir, filepath.FromSlash(ledgerPlanRelDir), "board.yaml")
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("expected persisted ledger plan: %v", err)
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

type budgetLedgerCaller struct {
	mu       sync.Mutex
	contexts []int
	queries  []string
}

func (c *budgetLedgerCaller) Complete(_ context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	ctxStr, _ := contextPayload.(string)
	c.mu.Lock()
	c.contexts = append(c.contexts, len(ctxStr))
	c.queries = append(c.queries, query)
	c.mu.Unlock()

	if strings.Contains(query, "extract grounding evidence for one package") {
		return "evidence:\n  - Sym.Method\nnotes: package does work\n", 1, 1, 1, 2, nil
	}
	return "evidence: Sym.Method\nclaims: data_shape\nobjective: Large slice holds shared shapes.\nverdict: grounded\n", 1, 1, 1, 2, nil
}

func TestBuildSliceObjectiveLedger_largeSliceStaysWithinContextBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}

	const pkgCount = 8
	var rolesBuilder strings.Builder
	rolesBuilder.WriteString("packages:\n")
	var rlmBuilder strings.Builder
	rlmBuilder.WriteString("# Package RLM context index\n")
	var owns []catalog.Component
	for i := 0; i < pkgCount; i++ {
		p := fmt.Sprintf("internal/pkg%02d", i)
		pkgDir := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "x.go"), []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rolesBuilder.WriteString(fmt.Sprintf(`  - path: %s
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
`, p))
		// Inflate each package snippet well past the old joined-slice size when summed,
		// while keeping each individual call under ledgerMaxContextChars.
		pad := strings.Repeat(fmt.Sprintf("- pad_%02d: %s\n", i, strings.Repeat("x", 1800)), 6)
		rlmBuilder.WriteString(fmt.Sprintf("\n## ./%s\n- package: pkg%02d\n- jsonTags: true\n- mechanicalRole: dto\n%s", p, i, pad))
		owns = append(owns, catalog.Component{ID: fmt.Sprintf("c%d", i), Path: p})
	}
	rolesBuilder.WriteString("edges: []\n")
	if err := os.WriteFile(filepath.Join(evidence, packageRolesRel), []byte(rolesBuilder.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(rlmBuilder.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	joinedLen := len(rlmBuilder.String())
	if joinedLen < ledgerMaxContextChars*2 {
		t.Fatalf("fixture joined context too small (%d); want > %d to prove the split", joinedLen, ledgerMaxContextChars*2)
	}

	draft := catalog.Typology{
		ID:     "demo",
		Slices: []catalog.Slice{{ID: "review", Owns: owns}},
	}
	rolesDoc, err := loadPackageRoles(filepath.Join(evidence, packageRolesRel))
	if err != nil {
		t.Fatal(err)
	}
	constraints := buildCapabilityConstraints(rolesDoc)
	caller := &budgetLedgerCaller{}

	doc, issues, err := buildSliceObjectiveLedger(context.Background(), caller, sliceLedgerBuildRequest{
		AnalysisDir: dir,
		EvidenceDir: evidence,
		DraftTypo:   draft,
		Constraints: constraints,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	if len(doc.Slices) != 1 || doc.Slices[0].ID != "review" {
		t.Fatalf("doc=%+v", doc)
	}
	if err := validateObjectiveLedgerDoc(doc); err != nil {
		t.Fatal(err)
	}

	caller.mu.Lock()
	defer caller.mu.Unlock()
	// Mechanical evidence: only the synthesis call hits Complete.
	if len(caller.contexts) != 1 {
		t.Fatalf("calls=%d want 1 synthesis only", len(caller.contexts))
	}
	for i, n := range caller.contexts {
		if n > ledgerMaxContextChars {
			t.Fatalf("call %d context chars=%d exceeds budget %d (query prefix=%q)",
				i, n, ledgerMaxContextChars, truncateForTest(caller.queries[i], 80))
		}
	}
	planPath := filepath.Join(dir, filepath.FromSlash(ledgerPlanRelDir), "review.yaml")
	body, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "steps:") || !strings.Contains(string(body), "synthesis") {
		t.Fatalf("plan missing steps:\n%s", body)
	}
}

func TestBuildSliceLedgerStepPlan(t *testing.T) {
	t.Parallel()
	plan := buildSliceLedgerStepPlan("ops", []string{"internal/b", "internal/a"})
	if plan.SliceID != "ops" {
		t.Fatalf("id=%q", plan.SliceID)
	}
	if len(plan.Packages) != 2 || plan.Packages[0] != "internal/a" {
		t.Fatalf("packages=%v want sorted", plan.Packages)
	}
	if len(plan.Steps) != 3 || plan.Steps[2] != "synthesis" {
		t.Fatalf("steps=%v", plan.Steps)
	}
}

func TestMechanicalPackageEvidenceNote(t *testing.T) {
	t.Parallel()
	md := `## ./internal/board
- packageDoc: Shared board payload shapes.
- jsonTags: true
- mechanicalRole: dto
- exportedMethods: Board.Load
- exportedFuncs: NewBoard
`
	note := mechanicalPackageEvidenceNote("internal/board", md)
	if note.Notes != "Shared board payload shapes." {
		t.Fatalf("notes=%q", note.Notes)
	}
	joined := strings.Join(note.Evidence, ",")
	if !strings.Contains(joined, "jsonTags:true") || !strings.Contains(joined, "Board.Load") {
		t.Fatalf("evidence=%v", note.Evidence)
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
	if !strings.Contains(q, "distilled per-package evidence") {
		t.Fatalf("missing distilled-evidence guidance:\n%s", q)
	}
}

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
