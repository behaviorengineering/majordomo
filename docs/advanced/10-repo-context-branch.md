# Repo Context Branch

*Majordomo: repository operations for evolving software.*

The context branch is durable project understanding on the **served repo**. It is not the default branch. Product PRs still target default. Context updates are a **separate PR whose base is the context branch**. Default stays free of Majordomo files.

This slice ships **schema and validation only**. Digest, skill selection, conversation-before-merge, and review injection are later work.

## Why

PR review is a sensor: each run sees one diff and forgets. The context branch is a **teaching story** of background and important decisions that shape how the code looks today. It is not an audit log of every merge. Newcomers should be able to read it. Later digest passes MAY compact older story for readability.

## Two layers

| Layer | Role | Mutability |
|-------|------|------------|
| **Cursor** (`meta.yaml` `last_merged_sha`) | Honest tape on default first-parent. Visit every node. | Only moves forward along that tape (or a documented rewrite workflow). |
| **Story** (mission, architecture, conventions, weaknesses, chronology, generated agenting packs) | Tutoring document. Only important, evidenced decisions. | Digest may add, reshape, and compact. |

Visiting a node is not the same as writing a chronology line. A no-op advances the cursor and writes **nothing** to the story.

## Location

- **Served repo**, same home as review cache (`repo: served`).
- Canonical branch: `majordomo-context/<repo-id>` (`config.ContextBranch`).
- Optional override: `context.branch` in central config.
- **Orphan history.** Context files only. Do not branch from default.
- **Never merge context into default.**

Update PRs (later digest job):

- Head: `majordomo-context/<repo-id>-update` (one catch-up branch per repo; hyphen avoids a git ref conflict with the base branch name).
- Base: always `majordomo-context/<repo-id>`.
- At most one open context update PR per repo. Restack on that head when default moves.
- **Humans MUST NOT push file edits** to the context or update branches. They steer with `@majordomo` comments on the open PR. The agent revises the same head. Merge happens after that conversation, without a human patch.

## Tree layout

On `majordomo-context/<repo-id>`:

```text
README.md           how to read; never-merge-to-default; humans talk on the PR, do not edit
meta.yaml           schema_version, repo_id, last_merged_sha, last_digest_at
mission.md          why the project exists
architecture.md     layers, entrypoints, ownership (living story)
conventions.md      how this repo is built and reviewed
weaknesses.md       known risks; cite chronology headings when claiming history
chronology.md       important evidenced decisions, newest first; compactable
agenting/           grounding packs for review (not review SKILL.md)
  index.yaml        pack id → globs + modes
  <area>/GROUNDING.md
```

Do not put application source on this branch. When `agenting/index.yaml` is present, `majordomo context validate` checks the index and each pack's `GROUNDING.md`. Bootstrap seeds `agenting/overview`.

Validate a worktree with:

```bash
majordomo context validate --dir <worktree>
```

## Bootstrap

Empty `last_merged_sha` means **start from last**: set the cursor to current default `HEAD` and do not walk earlier history. The first story is whatever digest can evidence from HEAD as it stands (tree + recent PR if any), not a reconstruction of the whole tape.

## Digest trigger (v1)

Tower cron (same interval family as poll, e.g. every 5m). The digest job runs for a repo only when the context cursor is **behind** default `HEAD`. If caught up, exit no-op unless an open update PR has a pending `@majordomo reject` (gate regen). Story updates, agenting materialization, and compaction run during catch-up. LLM provider keys (`ANTHROPIC_API_KEY` / `OPENAI_API_KEY`) are required on the digest cron job when story generation is enabled.

## First orphan (v1)

If `majordomo-context/<repo-id>` does not exist, the digest bot creates it (orphan history, schema tree, bootstrap `meta.yaml`) and opens the first PR from `majordomo-context/<repo-id>-update`. No human seed step.

## Credentials (v1)

Same forge token as review MUST allow: read default, push to `majordomo-context/**`, and open/restack PRs with base `majordomo-context/<repo-id>` and head `majordomo-context/<repo-id>-update`.

## Generic SCM (v1)

Open/restack requires GitHub, GitLab, or Bitbucket. For `scm: generic` (clone-only): skip opening a context PR; log and exit. Review grounding stays unavailable until a supported SCM is configured.

## Chronology contract

Chronology is **high-level architectural commentary** on what landed on default. It MAY only claim:

- what the digest can thoroughly check in that node's tree/diff, or
- what the product PR review already checked for that merge.

If `Because` / `In order to` cannot be evidenced, do not invent them. Treat the node as a story no-op (cursor still advances).

When a line is warranted:

```markdown
### 2026-08-28 - Alice - PR #412

- **Did:** extract auth into middleware
- **Because:** token checks were duplicated in three handlers (shown in the diff / review)
- **In order to:** make session expiry consistent (stated in the PR or checkable in the change)
- **Evidence:** PR #412, merge `abc123def`, review artifacts if used
```

Heading date is `YYYY-MM-DD`. Actor and source follow, separated by ` - `. Newest first.

Digest MAY compact older entries after later iterations so the document stays readable. Compaction MUST NOT invent claims. Cursor history is not compacted: `last_merged_sha` remains the tape.

## Catch-up on default (later digest)

`last_merged_sha` is a **cursor on default**, first-parent only. Context MUST follow that tape. It MUST NOT jump a node.

A digest run is catch-up, not "on this merge, digest that SHA":

1. Read the cursor from the write tip (the one open context update PR if it exists, else the merged context branch). Empty cursor: bootstrap (set to default `HEAD`).
2. Walk `last_merged_sha` → current default `HEAD` with `git log --first-parent --reverse`.
3. Visit **every** commit on that walk, in order.
4. For each node: maybe update the story (only if evidenced and important). **Always** advance the cursor to that SHA.
5. The new cursor is the last node fully processed. Incomplete nodes MUST NOT be skipped.

**Concurrent merges.** Two merges (`A` cursor, then `B`, then `C` at HEAD) MUST process `B` then `C` on the cursor. Story lines are optional per node. One digest worker per repo (Actions concurrency `context-digest-<repo-id>`). One open update PR; restack. Product review reads only the **merged** context tip.

**Linearize with default first-parent**, not PR numbers or wall-clock. Squash-to-default: each squash is one node. Merge commits: each first-parent commit on default is one node.

## History rewrite (v1)

If `last_merged_sha` is not an ancestor of default `HEAD`, digest enters **rewrite** mode (not normal catch-up).

1. Record a chronology event; set `meta.yaml` `rewrite_pending` + `rewrite_new_head`.
2. If `rewrite_why` is empty, Gate blocks until `@majordomo why <reason>` on the context PR.
3. When why is present, reshape story (`architecture.md` at minimum), reset cursor to new HEAD, clear rewrite flags.
4. Open/restack the update PR as usual.

## Conversation before merge (v1)

The context PR is the merge vehicle and talk surface.

- Humans MUST NOT commit on that branch.
- Only comments starting with `@majordomo` count (`context.gateCommentPrefix` override for tests).
  - `@majordomo reject <reason>` → gate regen on next digest (story re-run).
  - `@majordomo done` → conversation complete; human may merge.
  - `@majordomo why <reason>` → supplies rewrite reason when history was rewritten.
- Gate state persists in `gate.json` on the update branch.
- Default: **human clicks merge**. `context.autoMerge: true` opts into forge merge when gate is `done`.

## Two skill classes (do not mix)

Mechanical review skills and agenting (grounding) packs are different jobs. They MUST NOT share a file, a directory tree, or OpenCode skill discovery.

| Class | Lives | Job | Must not |
|-------|--------|-----|----------|
| **Mechanical** | Go state machine (`orchestrate`, `agent` loops, later per-batch steps). Rubric labels may stay as data, not a step script for the model. | Order, IO, schema, retries, checkpoints. Same idea as generate/score loops. | Absorb project story. Rely on the LLM to remember MUST/NEVER steps. |
| **Agenting** | Served context branch: `agenting/<area>/` (not `SKILL.md`) | Who this system is: mission, shape, evidenced decisions for that area. | Become review criteria or replace the machine. |

### Mechanical as a state machine (later)

Today the **coarse** machine already exists: prep → file waves → finalize → prose → summary loop → tech loop, with checkpoints. The **fine** protocol still lives in `pr-review.agent.md` (which file to read, write `<slug>.md`, classify tags, no shell). The model is asked to police itself. That is the part to pull into Go, like `RunSummaryLoop`.

Go owns the states. OpenCode is one action inside a state: "given this input and optional agenting pack, produce this artifact." Go then validates and either advances, retries, or fails.

Example file-review batch:

1. **Prepare** (Go `internal/filereview`): load reviewables from batch `manifest.json`.
2. **Judge** (strop in-process): write per-file markdown under `per-file/`.
3. **Validate** (Go): parse MD → structured reports; every reviewable has an artifact; every finding has severity + text (or explicit no-issues). Invalid → retry with `filereview_feedback.md`, up to a cap, then fail the batch.
4. **Assemble** (Go): write `findings.json`; rewrite markdown from structured reports (MD is the display formatter).

Implemented in `internal/filereview`; `orchestrate` waves call `filereview.Run`.

The LLM MUST NOT be given bash for review batches (already the intent). The machine MUST NOT offer tools the state does not allow.

What stays in the model: whether something is actually a defect, and wording. What leaves the prompt: step order, completeness, output shape, "you may not skip a file."

`pr-review-*/SKILL.md` shrinks toward rubric *data* the validator and the short Judge prompt both read. It MUST NOT remain a 500-line execution protocol.

**Workflows are deterministic.** Prep, waves, Prepare → Judge → Validate → Assemble, generate/eval/gate: Go + strop. Same inputs must take the same path. The LLM is only the Judge action (and optional workspace-tool calls). It MUST NOT choose the next mechanical state.

**Markdown is offboarded from the mechanical path.** `pr-review.agent.md` and step-by-step SKILL protocols stop being the machine. They MAY remain as generated human docs that describe the code, or they go away. They MUST NOT be what `dispatch` executes.

What stays markdown on purpose:

- Agenting packs and the context story (teaching, selected per task)
- Optional rubric tables as data (YAML/MD that the machine loads, not a script the model follows)
- Operator docs under `docs/`

Do not keep two sources of truth. If the machine changes, update code first; docs follow.

Prep still selects **which** mechanical machine (code vs docs vs summary). It copies rubric data if needed. It attaches agenting **beside** the Judge input, never folded into the machine.

### Driver: strop + DSPy; OpenCode as tools (later)

Do not grow another homemade generate/score loop. Majordomo should **drive** judgment through [strop](https://github.com/behaviorengineering/strop) (DSPy JobRunner, composition walks, refinement, Gate). OpenCode becomes a **tool** the runner may call when a job needs a repo workspace (read/grep/edit/shell), not the agent that owns the protocol.

| Layer | Owns |
|-------|------|
| **Majordomo Go** | Poll, prep, waves, checkpoints, which job/pack, selected agenting input. |
| **strop** | Generate → evaluate → refine until gate. Field/section/phase walks. Structured-output validation. Human Gate (conversation, reject-and-regen) without humans editing files. |
| **DSPy modules** | Judge signatures (findings, summary sections, digest story). Evaluators that score those fields. |
| **Workspace tool (port)** | Optional: explore or edit a checkout when staged diffs are not enough (digest, tech-deep, context PR amend). **OpenCode is one adapter.** |

File-review often needs **no** workspace tool: prep already staged the diff. JobRunner + XML/mandatory-field validation is enough. Call the port when the module must look past the batch.

The workspace is a **Go interface** (cwd-bounded: `Read`, `Grep`, `Edit`, `Shell` as the job allows). Implemented in `internal/workspace` (`Local`, `Stub`, `Guard` with `AllowNone` / `AllowTechDeep` / `AllowDigest`). DSPy/strop and Majordomo call the port only. They MUST NOT import OpenCode APIs or `agent-dispatch.sh` except inside one adapter package (`internal/workspace/opencode`, skeleton only today).

Per-job allowlist (locked):

| Job | Port |
|-----|------|
| File-review Judge | **None** (prep staged the diff) |
| Summary / tech Judge | None |
| Tech-deep | `Read`, `Grep` |
| Digest / context amend | `Read`, `Grep`, `Edit` (no `Shell` unless a later job opts in) |
| Tests | stub adapter (no process) |

Adapters can be swapped or stacked without changing Judge modules:

- OpenCode CLI (today's runtime)
- another coding agent
- a sandbox (no network, tighter FS)
- a stub for tests (no LLM, no process)

Enhance behind the same port (timeouts, allowlists, tracing). Do not leak adapter flags into strop or into `pr-review.agent.md`.

Map today's loops onto strop instead of new `for` loops:

- Summary / tech write→score → `internal/judge` + strop `JobRunner` + evaluator packs under `internal/judge/evaluation/{summary,tech}` (rubric IDs stay in Majordomo). Homemade loops in `internal/agent` remain the v1 path until cutover.
- File-review completeness → Go validate after generate (strop validators for required fields; Majordomo checks every reviewable has an artifact).
- Context conversation-before-merge → `humanreview.Gate` + reviewflow ports (comment = reject/regen message). `agentsession` for the short-lived transcript.
- Context story / compaction → **section-walk** over mission, architecture, conventions, weaknesses, chronology (lock passed sections). Not a separate phase-walk over the same files.

Strop boundary rules still apply: no product prompts inside strop; Majordomo owns signatures, rubric copy, and the workspace-tool adapter.

Judge always runs in-process via strop (`internal/judge`). OpenCode is not a protocol driver. The `majordomo-agent` image may still put OpenCode on PATH later for workspace tools only. Majordomo `go.mod` pins tagged `github.com/behaviorengineering/strop`.

## Grounding: selected agenting packs (v1 prep)

Do **not** dump the dossier into every OpenCode batch.

The story files (`mission.md`, `architecture.md`, …) are the human-readable source. Digest **materializes** small agenting packs plus an index (LLM story generation still later). Review **selects** packs by changed paths and job mode, independently of which mechanical skill the batch uses. The model MUST NOT probe `agenting/`.

```text
agenting/index.yaml
agenting/<area>/GROUNDING.md
```

`index.yaml` shape:

```yaml
packs:
  overview:
    modes: [summary, technical, digest]
  auth:
    globs: ["**/auth/**", "**/*jwt*"]
    modes: [files, summary, technical]
```

Examples:

- Summary / tech synthesis: `agenting/overview` plus area packs that match the PR's files. Mechanical skill remains `pr-review-summary` / `pr-review-technical`.
- File-review batch on `internal/auth/**`: mechanical `pr-review-code` plus agenting `agenting/auth` only.
- Unrelated docs batch: mechanical `pr-review-docs`; maybe no agenting pack, or overview only.

Selection MUST stay small. If an area pack does not match the task, it MUST NOT be attached.

**Shipped (v1):** `internal/agenting` loads `index.yaml`; prep (`AttachGrounding`) selects packs by glob + mode, copies `GROUNDING.md` into each batch `/.grounding/<id>.md`, and records `grounding_packs` on `manifest.json`. Pass `--context-dir` or `MAJORDOMO_CONTEXT_DIR` (merged context tip). Tower review workflow shallow-clones `majordomo-context/<repo-id>` when it exists. **`majordomo dispatch`** resolves paths and sets `MAJORDOMO_GROUNDING`; `agent-dispatch.sh` appends `grounding:` to the OpenCode prompt; `pr-review.agent.md` Step 1.5 reads only those files.

`pipelines.*.agentContext` in central YAML is **legacy** (still materialized today). It is not a substitute for packs. Phase 6 grounding is `agenting/`.

## Poll exclusion

Product poll ignores PRs whose base or head starts with:

- `majordomo-context/` (covers durable base and `…-update` head)
- `majordomo-pr-reviewer-cache/`
- `majordomo-poll-cache/`

Implemented in `internal/poll` (`isMajordomoInternalBranch`).

## Read model for review (v1)

Checkout the **merged** context tip only. Open context update PRs are not grounding until merged. Attach only the **selected** agenting packs for that batch, next to the mechanical skill, never folded into it. Review workflow sets `MAJORDOMO_CONTEXT_DIR` from a shallow clone of `majordomo-context/<repo-id>` when present.

## Later slices (not this work)

1. **Mechanical state machine.** Drive Judge with strop DSPy. Workspace port + per-job allowlists. Pull step protocol out of `pr-review.agent.md`. Structured findings; MD formatter after Validate.
2. **Poll filter.** Done: skip context and cache-branch PRs in `internal/poll`.

## Related

- [PLAN: Control Tower](../PLAN-control-tower-github-go.md) (Decision 5)
- `internal/contextstore` (schema)
- `internal/contextdigest` (catch-up job)
- `majordomo context validate`
- `majordomo context digest`
