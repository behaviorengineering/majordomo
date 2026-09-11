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
}
