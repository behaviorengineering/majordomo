package contextdigest

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
)

func TestEvaluateTypologyBoundariesRejectsHollowBindingSlice(t *testing.T) {
	t.Parallel()
	refined := `id: gitboard
slices:
  - id: server
    objective: Wire HTTP handlers to domain services.
  - id: gitboard-http
    objective: Delivery surface separated from the CLI entrypoint.
    surfaces:
      - id: gitboard-http-api
        kind: api
        components:
          - id: server-pkg
            path: internal/server
sliceBindings:
  - from: server
    to: triage
    kind: reads
`
	ok, feedback := evaluateTypologyBoundaries(refined, "# Journey\n", "", "")
	if ok {
		t.Fatal("expected hollow ownership failure")
	}
	if !strings.Contains(feedback, "majordomo_typology_slice_ownership") {
		t.Fatalf("feedback=%q", feedback)
	}
}

func TestCollapseHollowPackageSlicesRemountsBindings(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		ID: "gitboard",
		Slices: []catalog.Slice{
			{ID: "server", Objective: "Hollow leftover."},
			{
				ID:        "gitboard-http",
				Objective: "Delivery surface separated from the CLI entrypoint.",
				Surfaces: []catalog.Surface{{
					ID:   "gitboard-http-api",
					Kind: catalog.InteractionAPI,
					Components: []catalog.Component{
						{ID: "server-pkg", Path: "internal/server"},
					},
				}},
			},
			{ID: "triage", Objective: "Triage logic.", Owns: []catalog.Component{{ID: "triage-core", Path: "internal/triage"}}},
		},
		SliceBindings: []catalog.SliceBinding{
			{From: "server", To: "triage", Kind: catalog.SliceReads},
		},
	}
	out := collapseHollowPackageSlices(typo)
	for _, s := range out.Slices {
		if s.ID == "server" {
			t.Fatalf("hollow server slice should be removed: %+v", out.Slices)
		}
	}
	if len(out.SliceBindings) != 1 {
		t.Fatalf("bindings=%v", out.SliceBindings)
	}
	if out.SliceBindings[0].From != "gitboard-http" || out.SliceBindings[0].To != "triage" {
		t.Fatalf("binding=%v", out.SliceBindings[0])
	}
}

func TestValidateRefinedCatalogYAMLCollapsesHollowHTTPSlice(t *testing.T) {
	t.Parallel()
	draft := `id: gitboard
slices:
  - id: gitboard
    objective: Deliver the gitboard CLI.
    owns:
      - id: cmd-gitboard
        path: cmd/gitboard
      - id: server-pkg
        path: internal/server
  - id: server
    objective: Wire HTTP handlers.
    owns:
      - id: server-pkg
        path: internal/server
  - id: triage
    objective: Triage work items.
    owns:
      - id: triage-core
        path: internal/triage
`
	refined := `id: gitboard
slices:
  - id: gitboard
    objective: Deliver the gitboard CLI.
    owns:
      - id: cmd-gitboard
        path: cmd/gitboard
      - id: server-pkg
        path: internal/server
  - id: server
    objective: Wire HTTP handlers to domain services.
  - id: triage
    objective: Triage work items.
    owns:
      - id: triage-core
        path: internal/triage
sliceBindings:
  - from: server
    to: triage
    kind: reads
`
	roles := `packages:
  - path: cmd/gitboard
    role: entrypoint
    confidence: 0.9
  - path: internal/server
    role: server
    confidence: 0.9
  - path: internal/triage
    role: logic
    confidence: 0.9
`
	out, err := validateRefinedCatalogYAML(refined, draft, "gitboard", roles)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "id: server\n    objective") {
		t.Fatalf("hollow server slice should be collapsed, got %s", out)
	}
	if strings.Contains(out, "from: server") {
		t.Fatalf("hollow server bindings should remount, got %s", out)
	}
	if !strings.Contains(out, "gitboard-http") {
		t.Fatalf("expected gitboard-http slice, got %s", out)
	}
	if !strings.Contains(out, "from: gitboard-http") {
		t.Fatalf("expected bindings remounted to gitboard-http, got %s", out)
	}
	// Mechanical HTTP split must copy the parent objective so ledger alignment
	// can accept a contributing ledger sentence verbatim.
	if !strings.Contains(out, "id: gitboard-http") || !strings.Contains(out, "objective: Deliver the gitboard CLI.") {
		t.Fatalf("expected gitboard-http to copy parent objective, got %s", out)
	}
	if strings.Contains(out, "Delivery surface separated from the CLI entrypoint.") {
		t.Fatalf("must not invent prestige delivery objective, got %s", out)
	}
}

func TestMissingGroundedObjectivesDetectsAbsent(t *testing.T) {
	t.Parallel()
	typo := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{
				ID:        "board",
				Objective: "Hold board DTOs only; never sync remote state.",
				Owns:      []catalog.Component{{ID: "board-pkg", Path: "internal/board"}},
			},
			{
				ID:        "cli",
				Objective: "Run the CLI entrypoint.",
				Owns:      []catalog.Component{{ID: "cli-pkg", Path: "internal/cli"}},
			},
		},
	}
	constraints := packageCapabilityConstraintsDoc{
		Packages: []packageCapabilityConstraint{
			{Path: "internal/board", Role: roleDTO, Is: []string{capDataShape}, MustNot: []string{capSynchronizeState, capMergeAdapters}},
			{Path: "internal/cli", Role: roleEntrypoint, Is: []string{capRunCLI}, MustNot: []string{capOwnDomainRules}},
		},
	}
	arch := "# Architecture\n\nThe CLI wires commands.\n"
	missing := missingGroundedObjectives(arch, typo, constraints)
	if len(missing) != 2 {
		t.Fatalf("missing=%v want 2", missing)
	}
	ids := map[string]string{}
	for _, m := range missing {
		ids[m.ID] = m.Objective
	}
	if ids["board"] == "" || ids["cli"] == "" {
		t.Fatalf("ids=%v", ids)
	}
}

func TestEnsureArchitectureMarkdownAppendsVerbatimObjectives(t *testing.T) {
	t.Parallel()
	refined := `id: demo
slices:
  - id: board
    objective: Hold board DTOs only; never sync remote state.
    owns:
      - id: board-pkg
        path: internal/board
`
	constraints := `packages:
  - path: internal/board
    role: dto
    is: [data_shape]
    must_not: [synchronize_state, merge_adapters]
`
	arch := "# Architecture\n\nOverview of the product.\n"
	out, err := ensureArchitectureMarkdownKeepsGroundedObjectives(arch, refined, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hold board DTOs only; never sync remote state.") {
		t.Fatalf("expected verbatim objective, got %q", out)
	}
	if !strings.Contains(out, "## Grounded slice objectives") {
		t.Fatalf("expected grounded section, got %q", out)
	}
	if !strings.Contains(out, "- **board**: Hold board DTOs only; never sync remote state.") {
		t.Fatalf("expected board bullet, got %q", out)
	}
}

func TestEnsureArchitectureMarkdownIdempotentWhenPresent(t *testing.T) {
	t.Parallel()
	obj := "Hold board DTOs only; never sync remote state."
	refined := `id: demo
slices:
  - id: board
    objective: ` + obj + `
    owns:
      - id: board-pkg
        path: internal/board
`
	constraints := `packages:
  - path: internal/board
    role: dto
    is: [data_shape]
    must_not: [synchronize_state]
`
	arch := "# Architecture\n\nTeaching note: " + obj + "\n"
	out, err := ensureArchitectureMarkdownKeepsGroundedObjectives(arch, refined, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if out != arch {
		t.Fatalf("expected unchanged markdown when objective already present")
	}
	if strings.Count(out, obj) != 1 {
		t.Fatalf("duplicate objective count=%d in %q", strings.Count(out, obj), out)
	}
	again, err := ensureArchitectureMarkdownKeepsGroundedObjectives(out, refined, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if again != out {
		t.Fatalf("second ensure should be idempotent")
	}
}

func TestValidateBootstrapStorySectionArchitectureMissingObjectives(t *testing.T) {
	t.Parallel()
	input := BootstrapStoryInput{
		RepoID: "gitboard",
		TypologyRefinedCatalog: `id: demo
slices:
  - id: board
    objective: Hold board DTOs only; never sync remote state.
    owns:
      - id: board-pkg
        path: internal/board
`,
		TypologyPackageCapabilityConstraints: `packages:
  - path: internal/board
    role: dto
    is: [data_shape]
    must_not: [synchronize_state]
`,
	}
	err := validateBootstrapStorySection(input, "architecture", "# Architecture\n\nNo objectives here.\n")
	if err == nil {
		t.Fatal("expected missing grounded objectives error")
	}
	if !strings.Contains(err.Error(), "board") {
		t.Fatalf("feedback should name slice id: %v", err)
	}
}
