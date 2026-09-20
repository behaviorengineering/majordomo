package contextdigest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/behaviorengineering/strop/pkg/stepplan"
)

func TestRunSliceLedgerViaStepPlanResumeSkipsEvidence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "board")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "x.go"), []byte("package board\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	constraints := buildCapabilityConstraints(roles)
	target := ledgerSliceTarget{id: "board", paths: []string{"internal/board"}}
	byPath := constraintsByPath(constraints)

	ctxMD, _ := os.ReadFile(filepath.Join(evidence, "package_rlm_context.md"))
	whole := string(ctxMD)

	// Seed a completed evidence checkpoint as if a prior run died before synthesis.
	store, err := stepplan.NewFileStore(filepath.Join(dir, filepath.FromSlash(ledgerStepplanRelDir)))
	if err != nil {
		t.Fatal(err)
	}
	steps, pathByStep, err := buildLedgerStepplanSteps(dir, target.id, target.paths, whole)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := stepplan.NewPlan(ledgerPlanID(target.id), "slice_grounding", steps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SavePlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	note := packageEvidenceNote{
		Path:     "internal/board",
		Evidence: []string{"BoardPayload"},
		Notes:    "board shapes",
	}
	body, err := json.Marshal(note)
	if err != nil {
		t.Fatal(err)
	}
	evStep := plan.Steps[0]
	cp, err := stepplan.NewCompleteCheckpoint(plan.ID, evStep, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStep(context.Background(), cp); err != nil {
		t.Fatal(err)
	}
	_ = pathByStep

	caller := &countingLedgerResumeCaller{}
	entry, issue, err := runSliceLedgerViaStepPlan(
		context.Background(),
		caller,
		sliceLedgerBuildRequest{AnalysisDir: dir, EvidenceDir: evidence, Constraints: constraints},
		target,
		whole,
		byPath,
		roles,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if issue != "" {
		t.Fatalf("issue=%s", issue)
	}
	if entry.ID != "board" || entry.Verdict != ledgerVerdictGrounded {
		t.Fatalf("entry=%+v", entry)
	}
	for _, q := range caller.queries() {
		if strings.Contains(q, "extract grounding evidence for one package") {
			t.Fatalf("expected evidence step to be skipped on resume; queries=%v", caller.queries())
		}
	}
	foundSynth := false
	for _, q := range caller.queries() {
		if strings.Contains(q, "write the grounded meaning") {
			foundSynth = true
		}
	}
	if !foundSynth {
		t.Fatalf("expected synthesis call; queries=%v", caller.queries())
	}
}

type countingLedgerResumeCaller struct {
	mu    sync.Mutex
	calls []string
}

func (c *countingLedgerResumeCaller) Complete(_ context.Context, _ any, query string) (string, int, int, int, int, error) {
	c.mu.Lock()
	c.calls = append(c.calls, query)
	c.mu.Unlock()
	if strings.Contains(query, "extract grounding evidence for one package") {
		return "evidence:\n  - BoardPayload\nnotes: board\n", 1, 0, 0, 0, nil
	}
	return "evidence: BoardPayload\nclaims: data_shape\nobjective: Shared board payload shapes.\nverdict: grounded\n", 1, 0, 0, 0, nil
}

func (c *countingLedgerResumeCaller) queries() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.calls))
	copy(out, c.calls)
	return out
}
