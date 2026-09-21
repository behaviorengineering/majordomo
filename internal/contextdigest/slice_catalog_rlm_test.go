package contextdigest

import (
	"context"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/pkg/catalog"
)

type stubSliceCatalogRLM struct {
	answers map[string]string
	err     error
}

func (s stubSliceCatalogRLM) Complete(_ context.Context, _ any, query string) (string, int, int, int, int, error) {
	if s.err != nil {
		return "", 0, 0, 0, 0, s.err
	}
	for id, ans := range s.answers {
		if strings.Contains(query, "id: "+id) || strings.Contains(query, "id: "+id+"\n") {
			return ans, 1, 1, 1, 2, nil
		}
	}
	// Fallback: first answer
	for _, ans := range s.answers {
		return ans, 1, 1, 1, 2, nil
	}
	return "", 0, 0, 0, 0, nil
}

func TestResolveLedgerEntryForTargetMatchesByPathOverlap(t *testing.T) {
	t.Parallel()
	doc := sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{
		{ID: "review", OwnedPaths: []string{"internal/staging", "internal/judge/evaluation/summary"}, Objective: "Review ledger sentence."},
		{ID: "context", OwnedPaths: []string{"internal/contextstore"}, Objective: "Context ledger sentence."},
	}}
	byID := map[string]sliceObjectiveLedgerEntry{}
	for _, e := range doc.Slices {
		byID[e.ID] = e
	}
	got := resolveLedgerEntryForTarget(catalogAssembleTarget{
		id:    "judge-eval",
		paths: []string{"internal/judge/evaluation/summary", "internal/staging"},
	}, doc, byID)
	if got.Objective != "Review ledger sentence." {
		t.Fatalf("got=%+v", got)
	}
}

func TestStampLedgerObjectivesOntoCatalogRenamedSlice(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{Slices: []catalog.Slice{{
		ID:        "judge-eval",
		Objective: "Automate pull request review from staged changes through published findings.",
		Owns:      []catalog.Component{{Path: "internal/staging"}},
	}}}
	ledger := sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{{
		ID: "review", OwnedPaths: []string{"internal/staging"},
		Objective: "The review slice executes processes.",
	}}}
	got := stampLedgerObjectivesOntoCatalog(typo, ledger)
	if got.Slices[0].Objective != "The review slice executes processes." {
		t.Fatalf("objective=%q", got.Slices[0].Objective)
	}
}

func TestAssembleSliceCatalogFragmentsParallel(t *testing.T) {
	t.Parallel()
	folded := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "board", Objective: "placeholder", Owns: []catalog.Component{{Path: "internal/board"}}},
			{ID: "ops", Objective: "placeholder", Owns: []catalog.Component{{Path: "internal/ops"}}},
		},
	}
	ledger := sliceObjectiveLedgerDoc{Slices: []sliceObjectiveLedgerEntry{
		{ID: "board", Objective: "Shared board payload shapes.", Verdict: ledgerVerdictGrounded, Evidence: []string{"BoardPayload"}, Claims: []string{capDataShape}},
		{ID: "ops", Objective: "Run operator CLI.", Verdict: ledgerVerdictGrounded, Evidence: []string{"main"}, Claims: []string{capRunCLI}},
	}}
	stub := stubSliceCatalogRLM{answers: map[string]string{
		"board": `id: board
objective: Shared board payload shapes.
owns:
  - path: internal/board
`,
		"ops": `id: ops
objective: Run operator CLI.
owns:
  - path: internal/ops
`,
	}}
	out, err := assembleSliceCatalogFragments(context.Background(), stub, sliceCatalogAssembleRequest{
		FoldedTypo: folded,
		LedgerDoc:  ledger,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Issues) != 0 {
		t.Fatalf("issues=%v", out.Issues)
	}
	if len(out.Fragments) != 2 {
		t.Fatalf("frags=%d", len(out.Fragments))
	}
	byID := map[string]catalog.Slice{}
	for _, f := range out.Fragments {
		byID[f.ID] = f
	}
	if byID["board"].Objective != "Shared board payload shapes." {
		t.Fatalf("board=%q", byID["board"].Objective)
	}
}

func TestParseAndValidateSliceCatalogFragmentRejectsBadPath(t *testing.T) {
	t.Parallel()
	tTarget := catalogAssembleTarget{
		id:    "board",
		paths: []string{"internal/board"},
		slice: catalog.Slice{ID: "board", Owns: []catalog.Component{{Path: "internal/board"}}},
	}
	_, issue := parseAndValidateSliceCatalogFragment(`id: board
objective: Shared board payload shapes.
owns:
  - path: internal/evil
`, tTarget, sliceObjectiveLedgerEntry{Objective: "Shared board payload shapes."})
	if issue == "" || !strings.Contains(issue, "outside allowed") {
		t.Fatalf("issue=%q", issue)
	}
}

func TestParseAndValidateSliceCatalogFragmentForcesLedgerObjective(t *testing.T) {
	t.Parallel()
	tTarget := catalogAssembleTarget{
		id:    "board",
		paths: []string{"internal/board"},
	}
	frag, issue := parseAndValidateSliceCatalogFragment(`id: board
objective: Wrong prestige sentence.
owns:
  - path: internal/board
`, tTarget, sliceObjectiveLedgerEntry{Objective: "Shared board payload shapes."})
	if issue != "" {
		t.Fatalf("issue=%q", issue)
	}
	if frag.Objective != "Shared board payload shapes." {
		t.Fatalf("objective=%q", frag.Objective)
	}
}

func TestParseAndValidateSliceCatalogFragmentRepairsFlatOwnsAndSurfaces(t *testing.T) {
	t.Parallel()
	tTarget := catalogAssembleTarget{
		id: "operations",
		paths: []string{
			"internal/cli",
			"internal/aigateway",
			"internal/config",
			"internal/outbound",
			"internal/observability",
		},
	}
	raw := `id: operations
objective: prestige wording that must be replaced
owns:
id: cli
path: internal/cli
id: gateway
path: internal/aigateway
id: config
path: internal/config
id: outbound
path: internal/outbound
id: observability
path: internal/observability
surfaces:
id: operations-cli
kind: cli
components:
path: internal/cli
id: operations-api
kind: api
components:
path: internal/aigateway
`
	frag, issue := parseAndValidateSliceCatalogFragment(raw, tTarget, sliceObjectiveLedgerEntry{
		Objective: "Run operator CLI and HTTP gateway.",
	})
	if issue != "" {
		t.Fatalf("issue=%q", issue)
	}
	if frag.Objective != "Run operator CLI and HTTP gateway." {
		t.Fatalf("objective=%q", frag.Objective)
	}
	if len(frag.Owns) < 5 {
		t.Fatalf("owns=%d %+v", len(frag.Owns), frag.Owns)
	}
	if len(frag.Surfaces) != 2 {
		t.Fatalf("surfaces=%d %+v", len(frag.Surfaces), frag.Surfaces)
	}
}

func TestParseAndValidateSliceCatalogFragmentMapsServiceKindToAPI(t *testing.T) {
	t.Parallel()
	tTarget := catalogAssembleTarget{
		id:    "context",
		paths: []string{"internal/contextdigest", "internal/contextgate"},
	}
	raw := `id: context
objective: ignored
owns:
  - path: internal/contextdigest
  - path: internal/contextgate
surfaces:
  - id: context-manager
    kind: service
    components:
      - path: internal/contextdigest
`
	frag, issue := parseAndValidateSliceCatalogFragment(raw, tTarget, sliceObjectiveLedgerEntry{
		Objective: "Context load and gate.",
	})
	if issue != "" {
		t.Fatalf("issue=%q", issue)
	}
	if len(frag.Surfaces) != 1 || frag.Surfaces[0].Kind != catalog.InteractionAPI {
		t.Fatalf("surfaces=%+v", frag.Surfaces)
	}
}

func TestRepairFlatSliceCatalogYAMLLeavesProperLists(t *testing.T) {
	t.Parallel()
	in := `id: board
owns:
  - id: board
    path: internal/board
surfaces:
  - id: board-cli
    kind: cli
    components:
      - path: internal/board
`
	out := repairFlatSliceCatalogYAML(in)
	if !strings.Contains(out, "- id: board") || !strings.Contains(out, "- id: board-cli") {
		t.Fatalf("out=%s", out)
	}
}
