# Port to typology (majordomo pilot)

Working notes from curing a raw `typology discover` catalog so humans can navigate the code. Feed these into typology when we are ready to upstream.

## Problem

`discover` produced **28 slices / 33 packages** (one slice per package). That is accurate as an import inventory, but too fine-grained for a first architecture map. Humans need fewer bounded contexts, with merge rationale, before slice-walk or docs.

The journey went through three stages of clarity:
1. **Raw discover (28 slices):** Mechanical package-to-slice 1:1 mapping. Unusable as an architecture overview.
2. **Intermediate cluster pass (20 slices):** Merged obvious companion packages (`cluster` into `staging`, `satools` into `sa`, context packages into `context`), but still treated sequential workflow steps (`staging`, `orchestrate`, `judge`, `publish`) as separate peer slices.
3. **True bounded contexts (3 slices):** Consolidated around actual domain lifecycles (`review`, `context`, `operations`).

## What we did by hand (candidate product features)

1. **Coupling summary before walk**
   - Build top-level package import graph (in/out degree, leaves, hubs).
   - Present merge candidates with one-line package purpose + who imports whom.
   - Gate human approval of clusters before rewriting the draft catalog.

2. **Merge heuristics that worked here**
   - **Sole importer:** `cluster` is only imported by `staging` → merge into `staging`.
   - **Same job family:** `contextstore` + `contextdigest` + `contextgate` (+ `agenting` grounding packs) → `context`.
   - **Same product concern, split packages:** `sa` + `satools` → `sa`.
   - **Manifest companion:** `diff` only used from CLI and described as staging-manifest diffs → `staging`.
   - **Forge side-effects:** `outbound` (HTTP client) + `status` (commit status) sit next to `publish` → candidate `publish` or `forge` slice.
   - **Do not trust name similarity alone:** `agent` (review dispatch) vs `agenting` (context-branch grounding) look related; docs say they are not the same context.

3. **Domain concepts vs anti-patterns discovered in pilot**
   - **Anti-pattern 1: Temporal pipeline stages as slices.** Treating sequential execution phases (prep → wave orchestration → LLM evaluation → report formatting → publishing) as distinct slices is a DDD violation. They share the same aggregate/lifecycle (`review`) and should live together in one bounded context.
   - **Anti-pattern 2: Capabilities as domain pillars.** Internal facilities (such as strop DSPy evaluation, LLM gateways, workspace checkout inspection) are capabilities invoked by workflows, not autonomous domain pillars.
   - **Anti-pattern 3: Horizontal technical tiers as slices.** Having dedicated `cli` or `platform` slices fragments the domain. CLI commands (`majordomo review`, `majordomo context`) belong on the `surfaces[]` of the domain slice they invoke, while the root binary sits under the host/operations boundary.

4. **Journey gap**
   - Current journey goes `situation-draft` → `slice-walk` on raw discover rows.
   - Missing phase: **cluster-pass** (propose merges from graph + package docs, operator approves, rewrite draft, then walk the fewer slices).

## Desired typology behavior (draft backlog)

| ID | Ask | Why |
|----|-----|-----|
| T1 | `typology show graph` (or discover appendix): package edges, degrees, leaves | Humans cannot invent clusters from YAML alone |
| T2 | Optional `discover --suggest-merges` using sole-importer + package-comment keywords | Cuts 28→~20 without replacing human judgment |
| T3 | Journey phase `cluster-pass` before `slice-walk` | Matches how this pilot actually worked |
| T4 | Warn when two slice ids share a stem (`agent` / `agenting`) but have different importers | Prevents bad automatic merges |
| T5 | Keep platform-leaf detection (`config`, telemetry, auth helpers) as suggested *keep small* | Avoid swallowing shared infra into the first hub |
| T6 | Heuristic guidance distinguishing temporal pipeline stages from true bounded contexts | Prevents over-fragmenting sequential workflow steps into artificial peer slices |
| T7 | Auto-wire interaction packages (`cmd/*`, `internal/cli`) into `surfaces[].components` rather than `owns[]` | `discover` currently drops all `internal/` into `owns[]` with `layer: domain`, causing validation errors when `internal/cli` is present |
| T8 | Multi-stage discovery prompt/mode (Inventory → Domain Clustering → Journey Walk) | Helps agents guide users from raw import graph to clean bounded contexts without confusion |

## Out of scope for now

- Changing typology code in this repo (majordomo only consumes the tool).
- Auto-applying merges without an operator gate.

## Session log

- `2026-09-02`: Operator rejected raw discover as unusable for code navigation; asked for a second pass on connections and to track upstream port items (this file).
- `2026-09-02`: Operator approved four merge clusters; applied to draft (**28→20** slices, **49** bindings). Validate: structure/imports clean; only missing DocPages. Slice-walk deferred.
- `2026-09-02`: Operator clarified that AI evaluation and pipeline steps are capabilities/stages, not standalone pillars. Consolidated into **3 true bounded contexts** (`review`, `context`, `operations` across all 33 packages, **6** bindings, **7** DocPages). Validate clean.
- `2026-09-02`: Documented pilot findings, architectural anti-patterns, and backlog items (T1–T8) in this file for upstream porting into typology.
