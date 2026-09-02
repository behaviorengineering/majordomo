# Typology journey

Working session for mapping this Go module. The agent Reads this file first, updates it every turn, and waits at each human gate. Commands: `go tool typology discover|emit|validate|show`. Skills: `typology-journey`, `typology-cli`, `typology-catalog`, `typology-docs`.

## Resume

- **Phase:** docs
- **Next:** fill `docs/develop/review/overview.md`
- **Waiting on:** docs page-by-page fill
- **Upstream tracker:** `architecture/typology-upstream-notes.md`

## Status

| Field | Value |
|-------|-------|
| Repo | `github.com/behaviorengineering/majordomo` (`.majordomo` in majordomo-tower) |
| Started | `2026-09-02` |
| Phase | `docs` |
| Draft catalog | `architecture/typology.draft.yaml` |
| Confirmed catalog | `architecture/typology.yaml` (mirrored from draft) |

Phase order: `land` → `situation-draft` → `slice-walk` → `situation-freeze` → `desired` → `commit` → `docs` → `done`.

## Land

- [x] **Binary:** `typology version` (or `make build` in the Typology module) runs
      Method: command exit 0
      Pass: version printed (`typology dev` via `go tool typology` @ v0.0.4)
      Fail: STOP, build or install the CLI
- [x] **Skills:** journey, cli, catalog, and docs skills are loaded when that phase needs them
      Method: agent Read those SKILL.md files
      Pass: journey/cli at land; catalog when editing draft; docs skill Read at phase `docs`
      Fail: STOP, Read `skills/README.md` in the Typology module

## Situation draft

- [x] **Draft exists:** `architecture/typology.draft.yaml` is on disk
      Method: file present
      Pass: 3 bounded contexts (`review`, `context`, `operations`), 33 packages mapped
      Fail: STOP, `typology discover REPO --out architecture/typology.draft.yaml`
- [x] **Walk table filled:** every draft slice id has a row below
      Method: slice ids in the draft vs table
      Pass: 3 rows after domain consolidation
      Fail: STOP, add missing rows as `pending`

## Slice walk

Status per row: `pending` | `keep` | `rename` | `merge` | `split` | `later`

| Slice | Owns (count) | Bindings in | Bindings out | Status | Note |
|-------|--------------|-------------|--------------|--------|------|
| review | 20 | 2 | 2 | pending | PR review automation engine (prep, static analysis, waves, judge, cache, publish) |
| context | 4 | 2 | 2 | pending | Durable repository memory (validation, digest, grounding packs, human gate) |
| operations | 9 | 2 | 2 | pending | Control plane & runtime (SCM poll, outbound HTTP, config, CLI surface, gateway, telemetry) |

One `pending` row per turn. Rename/merge/split notes MUST name the target ids.

## Situation freeze

- [ ] **Operator freeze:** operator confirmed they can name the as-is slices and main couplings
      Method: explicit freeze in chat, recorded under Notes
      Pass: freeze sentence stored
      Fail: STOP, tutor the as-is map, wait
- [ ] **Draft matches walk:** keep/rename/merge/split rows are applied in the draft YAML
      Method: draft slice ids vs table
      Pass: no `pending` rows; later rows listed as deferred
      Fail: STOP, apply the remaining walk edits to the draft

Freeze note:

```
(paste the operator freeze here)
```

## Desired

Open decisions (add a row per unresolved reshape). Close a row when the draft YAML matches.

| ID | Decision | Status |
|----|----------|--------|
| | | open |

- [ ] **Names:** slice ids are the bounded contexts the operator wants
      Method: operator confirm after desired edits
      Pass: confirm recorded
      Fail: STOP, keep phase `desired`
- [ ] **Bindings:** couplings in the draft match the intended `consumes` / `reads`
      Method: operator confirm, then catalog structure
      Pass: confirm recorded
      Fail: STOP, edit draft bindings
- [ ] **Programs:** subprograms / actuators / opRuns filled or explicitly deferred
      Method: catalog skill; deferred list in Notes
      Pass: filled or deferred named
      Fail: STOP, fill or write the deferral

## Commit

- [ ] **Catalog written:** draft copied to `architecture/typology.yaml`
      Method: files differ only if emit also updated docs; yaml matches the frozen desired draft
      Pass: confirmed catalog exists
      Fail: STOP, copy the draft
- [ ] **Emit:** `typology emit REPO` ran after the copy
      Method: command exit 0
      Pass: exit 0
      Fail: STOP, fix emit errors
- [ ] **Validate:** `typology validate REPO` exit 0
      Method: CLI issue list empty
      Pass: exit 0
      Fail: STOP, fix each issue (cli skill)
- [ ] **Docs table seeded:** every confirmed `docs.pages[]` row plus each subprogram and actuator leaf is in the Docs table as `pending`
      Method: catalog pages vs table
      Pass: one row per DocPage and program leaf
      Fail: STOP, copy paths from the catalog

## Docs

Load `typology-docs`. One `pending` or `filled` row per turn. Status: `pending` | `skip-none` | `filled` | `revised` | `done` | `later`

| Slice | Kind | Path | Status | Note |
|-------|------|------|--------|------|
| review | overview | docs/develop/review/overview.md | pending | |
| review | components | docs/develop/review/components.md | pending | |
| context | overview | docs/develop/context/overview.md | pending | |
| context | components | docs/develop/context/components.md | pending | |
| operations | overview | docs/develop/operations/overview.md | pending | |
| operations | components | docs/develop/operations/components.md | pending | |
| operations | cli | docs/develop/operations/cli.md | pending | |

- [ ] **Pages scored:** every row is `done` or `skip-none` (`later` allowed)
      Method: no `pending`, `filled`, or `revised` rows
      Pass: table complete
      Fail: STOP, stay in phase `docs`
- [ ] **Markers:** accepted fills have no `<!-- typology:generated -->`
      Method: grep DocPage paths
      Pass: marker only on still-stub `later` pages
      Fail: STOP, remove the marker on accepted pages

Phase `done` only after Pages scored passes.

## Notes

Deferred slices (`later`) and program deferrals:

```
(none yet)
```

Landed with typology v0.0.4 via `go tool typology`. Operator asked to rediscover so docs match latest typology: re-ran `go tool typology discover . --out architecture/typology.draft.yaml`, synced `architecture/typology.yaml` to the same draft, fixed `id: majordomo`. DocPages are the v0.0.4 tree (59 pages: overview+components everywhere; cli/presentation only where surfaces exist).

Operator rejected raw 28-slice map as too fine for code navigation. **Cluster-pass applied** (28→20): staging←cluster+diff; context←contextstore+contextdigest+contextgate+agenting; sa←satools; publish←outbound+status. Draft + yaml rewritten; validate clean aside from missing DocPages (43). Upstream notes in `typology-upstream-notes.md`.

Docs stubs emitted for the 3-slice model (`review`, `context`, `operations`), validate passes, and the docs table is now seeded. Ready to fill pages one at a time starting with `review/overview.md`.
