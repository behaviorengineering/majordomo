# Port to typology (majordomo pilot)

Working notes from curing a raw `typology discover` catalog so humans can navigate the code, and from writing a full architecture narrative by hand so we can see what Typology must teach an LLM to generate automatically. Feed these into typology when we are ready to upstream.

## Problem

`discover` produced **28 slices / 33 packages** (one slice per package). That is accurate as an import inventory, but too fine-grained for a first architecture map. Humans need fewer bounded contexts, with merge rationale, before slice-walk or docs.

Even after a clean 3-slice catalog and filled DocPages, **emit + overview/components contracts still do not produce “how the system is put together.”** Operators expected Typology to document and validate current architecture; today it validates inventory and leaves narrative assembly to humans/agents outside the tool.

The journey went through three stages of clarity:
1. **Raw discover (28 slices):** Mechanical package-to-slice 1:1 mapping. Unusable as an architecture overview.
2. **Intermediate cluster pass (20 slices):** Merged obvious companion packages, but still treated sequential workflow steps as separate peer slices.
3. **True bounded contexts (3 slices):** Consolidated around domain lifecycles (`review`, `context`, `operations`).

Hand-written narrative that Typology did not generate: [`architecture.md`](../docs/architecture/architecture.md).

## What we did by hand (candidate product features)

1. **Coupling summary before walk**
   - Build top-level package import graph (in/out degree, leaves, hubs).
   - Present merge candidates with one-line package purpose + who imports whom.
   - Gate human approval of clusters before rewriting the draft catalog.

2. **Merge heuristics that worked here**
   - **Sole importer:** `cluster` only imported by `staging` → merge into `staging`.
   - **Same job family:** `contextstore` + `contextdigest` + `contextgate` (+ `agenting`) → `context`.
   - **Same product concern:** `sa` + `satools` → `sa`.
   - **Manifest companion:** `diff` → `staging`.
   - **Forge side-effects:** `outbound` + `status` next to publish paths (here folded into `review` publish/status, with outbound under `operations`).
   - **Do not trust name similarity alone:** `agent` vs `agenting`.

3. **Domain concepts vs anti-patterns**
   - Temporal pipeline stages are not slices.
   - Capabilities (judge, gateway, workspace) are not pillars.
   - Horizontal `cli` / `platform` slices fragment the domain.

4. **Journey gap:** formal **cluster-pass** before slice-walk (partially landed in typology v0.0.5 skills).

5. **Full architecture narrative (new)**
   - Product shape (control plane vs one job).
   - Two-repo deployment (tower + module).
   - Runtime paths (review, context, host) as sequenced diagrams.
   - Package role table per slice.
   - Binding rationale in prose, not only YAML edges.
   - Explicit “what Typology validates vs what this doc covers.”

## LLM architecture narrative (gaps vs `architecture.md`)

What an LLM needed from outside Typology to write [`architecture.md`](../docs/architecture/architecture.md), and what Typology should supply or prompt for next:

| Gap ID | Missing from Typology today | Needed so an LLM can auto-generate architecture prose |
|--------|-----------------------------|------------------------------------------------------|
| N1 | No first-class **system narrative** DocPage (or emit template) for the whole typology | Emit `docs/architecture.md` or `docs/develop/_system/overview.md` from catalog + graph |
| N2 | No catalog fields for **runtime path / lifecycle** (ordered steps across packages) | Optional `paths[]` or `subprograms`+`opRuns` filled enough that docs skill can narrate poll→prep→publish |
| N3 | Slice `objective` optional / unused on majordomo catalog | Require per-slice `objective` (business why); overview FILL must quote it |
| N4 | Binding edges have `kind` but no **rationale** string | Optional `rationale:` on `sliceBindings`; components Cross-slice FILL quotes it |
| N5 | Package purpose only in Go `doc.go`, not in catalog | Discover or journey step harvests package comments into component `summary` |
| N6 | Validate checks paths/imports/DocPage existence, not **narrative ↔ catalog drift** | Validate or docs skill: fail if architecture narrative cites packages not in owns, or omits hubs |
| N7 | Docs skill is **one leaf per turn**; no “assemble system doc” mode | Skill/phase: after freeze, generate system architecture from slices+bindings+paths in one artefact |
| N8 | No deployment/control-tower vocabulary in catalog | Optional typology-level `deployment:` notes or link field so LLM does not invent tower layout |
| N9 | CLI surface listed as packages, not command inventory | Emit or harvest cobra `Use`/`Short` into cli DocPage (or opRun `cli:` fields) |
| N10 | No gold exemplar of a “full architecture” doc in typology testdata | Ship a tiny-module `architecture.md` + docs skill rubric that scores against it |

**Working definition of done for Typology docs:** an agent, given only the catalog + graph + package comments + skills, can produce a document with the same *sections* as majordomo `architecture.md` (product shape, slices, runtime paths, package roles, couplings, deployment, typology fit) without a human pasting the control-tower plan into chat.

## Desired typology behavior (draft backlog)

| ID | Ask | Why |
|----|-----|-----|
| T1 | `typology show graph` (or discover appendix): package edges, degrees, leaves | Humans cannot invent clusters from YAML alone |
| T2 | Optional `discover --suggest-merges` | Cuts package noise without replacing judgment |
| T3 | Journey phase `cluster-pass` before `slice-walk` | Matches this pilot |
| T4 | Warn on stem-similar slice ids with different importers | Prevents bad merges |
| T5 | Platform-leaf detection as *keep small* | Avoid swallowing shared infra |
| T6 | Heuristics: temporal stages ≠ bounded contexts | Prevents fake peer slices |
| T7 | Auto-wire `cmd/*` / interaction pkgs onto `surfaces[]` | discover currently dumps `internal/` as domain owns |
| T8 | Multi-stage discovery (Inventory → Clustering → Walk) | Agent guidance |
| T9 | System-narrative emit + docs skill (see N1, N7) | Close the “thin DocPages ≠ architecture” gap |
| T10 | Slice `objective` + binding `rationale` required for docs FILL (N3, N4) | LLM has quotable why |
| T11 | Harvest package `doc.go` into component summaries (N5) | Evidence without reading entire tree |
| T12 | Optional narrative-drift checks (N6) | Validate architecture docs against catalog |
| T13 | CLI command harvest or opRun cli fields (N9) | Auto-fill cli DocPages |
| T14 | Fixture architecture.md + scoring rubric (N10) | Regression for LLM doc quality |

## Out of scope for now

- Changing typology code inside majordomo (consumer only).
- Auto-applying merges without an operator gate.
- Treating the hand-written `architecture.md` as a Typology DocPageKind until upstream adds one.

## Session log

- `2026-09-02`: Operator rejected raw discover; asked for clustering + upstream notes.
- `2026-09-02`: Cluster passes → **3** bounded contexts (`review`, `context`, `operations`).
- `2026-09-02`: Documented anti-patterns and backlog T1–T8.
- `2026-09-02`: Bumped typology to `v0.0.5`; from-scratch remap still converged to the same 3 slices.
- `2026-09-02`: Filled seven develop DocPages; journey `done`. Operator found them too thin for “how the system is put together.”
- `2026-09-03`: Wrote hand architecture narrative `docs/architecture/architecture.md` and expanded this file with LLM narrative gaps N1–N10 and backlog T9–T14.
