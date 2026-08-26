# Plan: Control Tower, GitHub Actions, and Go

*Majordomo — repository operations for evolving software.*

**Status:** In progress (Phases 1–3 and most of Phase 5 done; Bitbucket poll and a few Phase 4 items remain)  
**Date:** 2026-08-22 (progress updated 2026-08-27)  
**Audience:** Maintainers and contributors planning the next major evolution of Majordomo.

---

## Summary

Majordomo is a **control plane for evolving repositories**: durable org config, SCM adapters, cache, and a Go job runner. **PR review is one workflow** on that plane (prep → agent waves → publish), not the product boundary.

**Current stack (this repo):** Go CLI (`majordomo`) owns poll, prep, orchestrate, dispatch, publish, status, cache, and report. Agents run via OpenCode (`agent-dispatch.sh`). Remaining bash is dispatch and image build.

**Target architecture:**

1. **Control-tower repo** — one place for org config and a pinned `.majordomo/` submodule; served repos stay clean (no workflow files by default).
2. **GitHub Actions minutes** — all pipeline compute runs on GitHub-hosted runners in the control tower.
3. **Any git host** — operate on GitHub, GitLab, Bitbucket, or self-hosted git; SCM API adapters for poll, clone, and publish.
4. **Go control plane** — deterministic job logic in a single static binary; agent and SA tooling in focused containers.
5. **OpenCode** — agent runtime for jobs that need LLM skills (skills/personas remain file-based).
6. **Pull poll (reconciliation)** — `majordomo-poll.yml` runs on every onboarded repo regardless of push config. Push triggers are optional speed boosts. See [Trigger modes and onboarding](#trigger-modes-and-onboarding).

Bitbucket (and other SCMs) remain valid **targets** for repository operations.

---

## Goals

| Goal | Description |
|------|-------------|
| **Evolve with the repo** | Durable config, cursors, and cache so jobs compound over time — not one-shot scripts. |
| **No repo pollution** | Served repos do not add `.github/workflows/`, `.majordomo/`, or Majordomo config (default pull mode). |
| **Central onboarding** | Add `majordomo-central-config/<repo>.yaml` + store SCM credential in tower secrets; no app-repo merge required. |
| **Submodule versioning** | Control tower pins `.majordomo` at a commit; bump pointer to roll out engine changes. |
| **SCM-agnostic core** | Same control plane for any git remote; adapters for trigger + publish only. |
| **Fast, portable binary** | Go for poll, staging, orchestration, cache, publish — one small container on GHA. |
| **Agent when needed** | OpenCode in a slim agent image for skill-backed jobs (review today; more later). |

## Non-goals (for this phase)

- Rewriting agent skills or review rubrics in Go.
- Replacing third-party SA tools (ruff, eslint, etc.) with Go implementations.
- Building a hosted SaaS control plane (self-hosted control tower only).
- Treating PR review as the only supported job forever (the plane must stay open to new workflows).

---

## Current state (this repository)

```text
majordomo (this repo)
  ├── agents/                        (skills + personas)
  ├── pipelines/scripts/             (agent-dispatch + image build)
  ├── dockerfiles/                   (public/corp images)
  ├── .github/workflows/             (image CI on GHA)
  ├── cmd/majordomo + internal/      (Go control plane)
  └── docs/                          (setup + architecture plan)
```

See [02 — Setup](02-setup.md) for local builds, and the rest of this plan for the control-tower target.

---

## Target architecture

**Default path (pull mode):** control-tower workflow polls SCM APIs for work; served repos change nothing. PR review is the first end-to-end job wired on that path.

```text
┌─────────────────────────────────────────────────────────────────────────┐
│  Any SCM (GitHub / GitLab / Bitbucket / self-hosted)                     │
│  Served repos: NO Majordomo files (default)                              │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │  outbound API (poll open PRs/MRs)
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  xynova/majordomo-tower (GitHub)      ← burns GitHub Actions minutes     │
│  ├── .majordomo/                      (submodule → behaviorengineering/majordomo)
│  ├── majordomo-central-config/
│  │     ├── _defaults.yaml
│  │     └── <repo-id>.yaml
│  └── .github/workflows/
│        ├── majordomo-poll.yml          (cron — default trigger)
│        └── majordomo-review.yml        (runs review; also accepts dispatch)
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
  ghcr.io/majordomo        ghcr.io/majordomo-agent   ghcr.io/sa-tools/*
  (Go binary)              (OpenCode only)           (ruff, eslint, …)
        │                       │
        └───────────┬───────────┘
                    ▼
           clone served repo → prep → review waves → publish → artifacts
```

**Optional paths (advanced):** push triggers via thin per-repo workflow, webhook router, or GitHub App — see [Trigger modes](#trigger-modes-and-onboarding). Not required for initial rollout.

### Design principles

1. **GitHub is the engine room, not the SCM lock-in.** Runners and secrets live in the control tower; target repos can be anywhere.
2. **Normalize early.** Every SCM adapter emits the same dispatch payload; downstream code is SCM-blind.
3. **Thin edges, fat core in Go.** Webhook routers and publishers are small; staging/orchestration/cache are one binary.
4. **Agents stay files.** Skills, personas, and templates remain markdown under `agents/`; OpenCode loads them from disk.

---

## Decisions

| # | Topic | Decision |
|---|-------|----------|
| 1 | Review cache location | **On the served repo.** Cluster analysis cache branches live in the app repo under review (`majordomo-pr-reviewer-cache/<project-id>`). |
| 2 | Poll cursor location (v1) | **GitHub Actions cache** on the control-tower job (directory `.poll-cache/`, files `.poll-cache/<repo-id>/poll-cursor.json`). Not on the served repo yet. Durable enough for pilot; see Phase 5 for optional served-repo git branch hardening. |
| 3 | Control-tower location | **Separate repository** [`xynova/majordomo-tower`](https://github.com/xynova/majordomo-tower). Pipeline code stays at [`behaviorengineering/majordomo`](https://github.com/behaviorengineering/majordomo). Tower pins that repo as `.majordomo/` submodule; holds org config, GHA workflows, and optional trigger deploy assets. |
| 4 | Default trigger | **Pull poll always runs** (every 5m, GitHub cron floor) as the reconciliation layer for all onboarded repos. Push modes (workflow/webhook) are optional accelerators on top — not a replacement for poll. |

### Control-tower repository

Two-repo split:

| Repo | Role |
|------|------|
| [`behaviorengineering/majordomo`](https://github.com/behaviorengineering/majordomo) | Pipeline **code** — Go binary source, agents/skills, dockerfiles, tests |
| [`xynova/majordomo-tower`](https://github.com/xynova/majordomo-tower) | Pipeline **deployment** — submodule pin, central config, GitHub Actions (compute / poll) |

The control-tower repo is the only GitHub repo that runs workflows and burns Actions minutes. It does **not** fork or duplicate Majordomo logic — it checks out `.majordomo/` at a pinned commit and invokes the binary/scripts from there.

```text
xynova/majordomo-tower/
├── .gitmodules
├── .majordomo/                     # submodule → behaviorengineering/majordomo @ <sha>
├── majordomo-central-config/
│   ├── _defaults.yaml
│   └── <repo-id>.yaml              # one per onboarded served repo
├── .github/workflows/
│   ├── majordomo-poll.yml          # cron — default entry (pull mode)
│   └── majordomo-review.yml        # review job (called from poll or dispatch)
└── triggers/                       # optional: webhook router (push mode only)
```

**Submodule setup** (in control-tower):

```bash
git submodule add https://github.com/behaviorengineering/majordomo.git .majordomo
# pin to a release tag or commit; bump pointer to roll out pipeline changes org-wide
```

**Versioning:** control-tower PRs that only bump `.majordomo` are routine rollout PRs. Config changes (`majordomo-central-config/`) are independent of pipeline code changes.

**Not in scope:** a `pipelines` or `tower` branch inside `majordomo` that holds config — config and runtime entrypoints live in the tower repo only.

### Review cache (served repo)

Behaviour for cluster cache on the served repo:

Git-tracked analysis files on branch `majordomo-pr-reviewer-cache/<project-id>` (or similar), with retention and fingerprint checks. See `majordomo cache` (`precheck` / `lookup` / `store` / `restore` / `push`).

- Cache branch pattern: `majordomo-pr-reviewer-cache/<project-id>`.
- Skill-scoped layout: `<skill-name>/analysis-<cluster-sha>.json`
- Cluster SHA keys derived from clustering output — skip re-analysis on cache hit.
- Control tower clones the **served repo** for review; cache read/write uses the same clone remote and a write-capable token for that repo.
- `cache.repo: central` (cache in control-tower repo) is **not planned** for the new stack unless a future need arises.

```yaml
# majordomo-central-config/payments-api.yaml (cache section)
cache:
  repo: served              # only supported mode in v1
  dir: .majordomo-review-cache/payments-api   # optional; defaults per legacy rules
  retentionDays: 120
  # Analysis-cache skips are on by default; set disableSkips: true to force re-analysis.
  # disableSkips: true
```

Go port: `majordomo cache` implements the git-branch storage model (`precheck` / `lookup` / `store` / `restore` / `push`).

**Poll cursor (v1):** separate from review cache. Stored under tower job `.poll-cache/` via Actions cache — see [Poll cache](#poll-cache-where-poll-compares-head_sha).

---

## Normalized dispatch payload

All SCM adapters must produce this shape (JSON) before triggering the control-tower workflow:

```json
{
  "review_id": "gitlab:acme/payments-api:42:abc123def",
  "scm": "gitlab",
  "repo_id": "payments-api",
  "change_id": 42,
  "head_ref": "feature/foo",
  "base_ref": "main",
  "head_sha": "abc123def456...",
  "event": "opened",
  "clone_url": "https://gitlab.com/acme/payments-api.git",
  "api": {
    "base_url": "https://gitlab.com",
    "project_id": "12345"
  }
}
```

| Field | Purpose |
|-------|---------|
| `review_id` | Stable dedup key for `concurrency:` groups |
| `scm` | Selects publisher + credential namespace |
| `repo_id` | Key into `majordomo-central-config/<repo_id>.yaml` |
| `change_id` | PR/MR number |
| `head_sha` | Checkout ref |
| `clone_url` | May differ from webhook payload; config can override |

`repository_dispatch` event type: `majordomo-review`.

---

## Central config schema (draft)

Per-repo config lives in YAML: `majordomo-central-config/<repo_id>.yaml`. Org defaults in `_defaults.yaml` are deep-merged; per-repo keys win.

### Loaded by Go today (`internal/config`)

Includes SCM, trigger, review, cache, **pipelines**, and **staticAnalysis**.

```yaml
# majordomo-central-config/payments-api.yaml

scm: gitlab   # github | gitlab | bitbucket | generic

repository:
  id: payments-api
  owner: acme                 # required for org token lookup (GH_TOKEN_* / GITLAB_TOKEN_*)
  name: payments-api
  cloneUrl: https://gitlab.com/acme/payments-api.git

scmApi:
  baseUrl: https://gitlab.com
  projectId: "12345"
  # Credentials: MAJORDOMO_CREDENTIAL_payments-api (override) or GITLAB_TOKEN_ACME (org/group).

review:
  publishMode: auto    # auto | comment | description | check | off
  # false: one review per PR/MR (cursor remembers the number). true: re-queue when head_sha changes.
  enableContinuousRuns: false

trigger:
  poll: true             # always on — reconciliation (GitHub cron min 5m)
  interval: 5m
  push:
    mode: none             # none | workflow | webhook

cache:
  repo: served           # review cache branches live on the served repo
  retentionDays: 120
  # disableSkips: true   # opt out of analysis-cache skips (skips are on by default)

pollCache:
  repo: served           # reserved; v1 cursor is Actions .poll-cache (see Decisions #2)
  # branch defaults to majordomo-poll-cache/<repo-id> when/if served-repo cursor lands

pipelines:
  pr-review:
    model: anthropic/claude-sonnet-4-5   # sets COPILOT_MODEL if unset
    scoreModel: auto                     # sets COPILOT_SCORE_MODEL if unset
    routing:                             # materialized to routing.json for prep
      pr-review-docs:
        - "**/*.md"
      pr-review-code:
        - "**"
    agentContext:                        # materialized to agent-context.json
      global:
        customRules:
          - "No hardcoded credentials."

staticAnalysis:                          # majordomo sa → .sa/<tool>.txt before prep
  - tool: ruff
    image: ghcr.io/org/sa-ruff:latest    # or dockerfile: + MAJORDOMO_SA_IMAGE_PREFIX
    command: check --output-format=concise
    glob: "**/*.py"
```

Legacy top-level `publishMode:` is still accepted as a fallback for `review.publishMode`.

`prep` / `orchestrate` accept `--config-dir` + `--repo-id` to load this YAML (explicit `--routing` / `--agent-context` still win). `majordomo sa` runs `staticAnalysis` via `run-sa-tool.sh`.

---

## Trigger modes and onboarding

### Pull poll = universal reconciliation

`majordomo-poll.yml` runs on a **5-minute cron** (GitHub’s minimum) for **every onboarded repo**. It asks the SCM API which open PRs/MRs need review, then applies the poll cursor and `review.enableContinuousRuns` (see [Poll algorithm](#poll-algorithm)).

```text
                    ┌─────────────────────────────────┐
                    │  majordomo-poll.yml (always)     │
                    │  cron */5 — reconciliation       │
                    └───────────────┬─────────────────┘
                                    │
          ┌─────────────────────────┼─────────────────────────┐
          ▼                         ▼                         ▼
   pull-only repo            push + poll repo          missed webhook
   (beginner default)        (fast path + safety net)   caught next poll
```

**Why poll is always on:**

| Role | Detail |
|------|--------|
| **Beginner default** | No served-repo files, no hosting, corp-friendly — poll is the only trigger. |
| **Reconciliation** | Catches missed, duplicate, or out-of-order webhook/workflow events. |
| **Source of truth** | SCM API state wins over “did we receive an event?” |
| **Dedup** | Skip when the poll cursor says this PR is done (same head, or already reviewed once if continuous runs are off). Push and poll share the same `review_id` concurrency group. |

Push modes don’t replace poll — they **front-run** it. If a webhook fires and review completes, the next poll sees `head_sha` already done and skips.

### Why pull-only is enough for most onboardings

| Benefit | Detail |
|---------|--------|
| **No served-repo changes** | Nothing to merge to default branch — avoids PR approver gatekeeping. |
| **No extra hosting** | No webhook router, no GitHub App, no inbound URL on corp network. |
| **Cross-org** | Platform admin issues PAT; no workflow in foreign repos. |
| **Why poll** | Outbound API poll instead of relying on inbound webhooks alone. |

Tradeoff for pull-only: reviews start within one poll interval (~5 min + scheduler jitter). Add push when a repo needs faster feedback.

### Trigger config (per repo)

```yaml
trigger:
  poll: true              # default true — always reconcile (do not disable in v1)
  interval: 5m            # GitHub cron floor; poll job uses */5
  push:
    mode: none            # none | workflow | webhook
```

| `push.mode` | Served repo | Extra hosting | Effect |
|-------------|-------------|---------------|--------|
| **`none`** (beginner default) | None | None | Poll only — review within ~5 min |
| **`workflow`** | Thin stub on default branch | None | Instant on `pull_request`; poll reconciles misses |
| **`webhook`** | None | External router | Instant on SCM webhook; poll reconciles misses |

Legacy `trigger.mode: pull|hybrid|push-workflow` maps to this shape in the Go config loader.

Org default in `_defaults.yaml`:

```yaml
trigger:
  poll: true
  interval: 5m
  push:
    mode: none
```

### Beginner onboarding (pull mode)

All steps happen in the **control-tower** repo and platform admin consoles — not in the served repo.

1. **Platform admin** (customer or yours) creates a machine identity with:
   - Read access to the served repo (clone + list open PRs/MRs)
   - Write access to post PR/MR comments (and optional check/status)
   - GitHub: fine-grained PAT for **one** resource owner (org), or a GitHub App install
   - GitLab: group access token (or bot PAT) for the group that owns the project
2. Store token in control-tower secrets (prefer one per org/group):
   - GitHub: `GH_TOKEN_<OWNER>` (Actions forbids secret names starting with `GITHUB_`)
   - GitLab: `GITLAB_TOKEN_<OWNER>` (nested `group/sub` → `GROUP_SUB`)
   - Optional per-repo override: `MAJORDOMO_CREDENTIAL_<REPO_ID>`
3. Add `majordomo-central-config/<repo_id>.yaml` (clone URL, SCM API base, `repository.owner`; `trigger.push.mode: none` is fine).
4. Ensure the GHA runner can reach the SCM API (public SaaS or **self-hosted runner** on corp network for internal Bitbucket). Map org secrets into the poll/review job env (Actions cannot resolve secret names dynamically).
5. `majordomo-poll.yml` cron runs → discovers work → queues `majordomo-review` → publishes.
6. For LLM review output: inject provider API key(s) into the review job and run with the agent image (or a runner that has OpenCode + keys). Prep alone is not a successful pilot.

No webhook URL. No merge to `main` on the served repo.

### Pilot definition of done (pull mode)

A pilot is complete only when all of the following are true:

1. **Submodule pin** — tower `.majordomo` points at a majordomo SHA that includes org credentials + `review.*` loader.
2. **Org secrets** — tower has `GH_TOKEN_<OWNER>` and/or `GITLAB_TOKEN_<OWNER>` (plus optional `MAJORDOMO_CREDENTIAL_<REPO_ID>`), mapped into poll/review job `env:`.
3. **Repo YAML** — `majordomo-central-config/<repo_id>.yaml` with `scm`, `repository.owner` / `name` / `cloneUrl`, and `scmApi` as needed.
4. **Continuous policy** — `review.enableContinuousRuns` set intentionally (`false` = one review per PR; `true` = re-queue on new `head_sha`).
5. **Agent runtime** — review job can run OpenCode (`majordomo-agent` image or equivalent) with at least one of `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` / `OPENCODE_PROVIDER_API_KEY`.
6. **End-to-end proof** — poll discovers an open change, review produces `summary.md`, publish posts to the PR/MR (or documents artifact-only if publish mode is `off`).
7. **No served-repo Majordomo files** — pull mode only.

### Push mode (optional accelerator)

**`push.workflow`** — served repo adds a stub on default branch that calls `repository_dispatch` on the control-tower. Poll still runs every 5m for reconciliation.

**`push.webhook`** — SCM admin points webhook at external router → `repository_dispatch`. Poll still runs.

### Cross-org served repos

Pull mode: customer platform team issues PAT → your tower secrets. No files in their repo.

Push-workflow: customer must merge stub to default branch **and** store `MAJORDOMO_DISPATCH_TOKEN` org secret (PAT scoped to trigger control-tower only).

### Internal flow (pull)

```text
majordomo-poll.yml (cron */5 — always)
  │
  ├─ for each onboarded repo in majordomo-central-config/
  ├─ SCM API: list open PRs/MRs; compare head_sha to last_reviewed[review_id]
  ├─ skip if already reviewed at this sha (push path may have handled it)
  └─ invoke majordomo-review / majordomo orchestrate

majordomo-review.yml (on repository_dispatch — when push.mode is workflow|webhook)
  │
  └─ same review path; poll dedupes on next cycle if duplicate
```

### Poll cache (where poll compares cursors)

Every poll cycle asks which open PRs/MRs need a review run. That uses the **poll cursor**, separate from the cluster review cache.

```text
review.enableContinuousRuns: true
  PR #42 last head abc123, API still abc123  → skip
  PR #42 last head abc123, API now def456    → queue review, update cursor after success

review.enableContinuousRuns: false (default)
  PR #42 never in cursor                     → queue once
  PR #42 already in cursor (any head)        → skip until cursor cleared
```

| Cache | Location (v1) | Skips |
|-------|---------------|-------|
| **Poll cursor** | Tower Actions cache → `.poll-cache/<repo-id>/poll-cursor.json` | Whole review job (see continuous policy above) |
| **Review cache** | Served-repo git branch `majordomo-pr-reviewer-cache/<project-id>` | Cluster AI work inside a run |

**v1 store of truth for poll:** GitHub Actions `actions/cache` on poll and review jobs (restore before poll / after review cursor update). Eviction (~7 days unused) may re-queue; acceptable for pilot. Optional Phase 5: move cursor to served-repo branch `majordomo-poll-cache/<repo-id>` for multi-runner durability.

**Poll cursor file shape** (`.poll-cache/<repo-id>/poll-cursor.json`):

```json
{
  "repo_id": "payments-api",
  "updated": "2026-08-27T00:12:00Z",
  "heads": {
    "42": "abc123..."
  }
}
```

<a id="poll-algorithm"></a>

**Poll algorithm:**

1. List open PRs/MRs from SCM API (credential: `MAJORDOMO_CREDENTIAL_<REPO_ID>` or `GH_TOKEN_<OWNER>` / `GITLAB_TOKEN_<OWNER>`).
2. Restore `.poll-cache/` from Actions cache (or start empty).
3. Read `.poll-cache/<repo-id>/poll-cursor.json`.
4. For each open PR:
   - If `review.enableContinuousRuns` is **false** (default): queue only when the PR number is absent from `heads`.
   - If **true**: queue when the PR is absent **or** `heads[pr] !=` current `head_sha`.
5. Emit pending reviews JSON; tower queues `majordomo-review` per item.
6. After a successful review, write the PR → `head_sha` into the cursor and save Actions cache.

**Concurrency:** `majordomo-review` uses `concurrency: group: majordomo-${{ review_id }}` so push + poll don’t double-run.

---

## SCM adapters (clone + publish)

Poll, clone, and publish are SCM-specific; trigger mode is independent.

| SCM | Pull (list PRs) | Clone | Publish |
|-----|-----------------|-------|---------|
| **GitHub** | `GET /repos/{owner}/{repo}/pulls` | `GH_TOKEN_<OWNER>` (or per-repo override) | PR comment, description, Checks API |
| **GitLab** | MR API | `GITLAB_TOKEN_<OWNER>` (or per-repo override) | MR note, pipeline status |
| **Bitbucket** | PR API (Server/Cloud) | SSH key or token from config | REST comment/description (Go port of existing script) |
| **Generic** | Manual / `workflow_dispatch` only | Deploy key / PAT | Optional — artifact-only mode |

---

## Go control plane

### Single binary: `majordomo`

Built from this repo (new top-level `cmd/` and `internal/`). Distributed as `ghcr.io/behaviorengineering/majordomo`.

| Subcommand | Role |
|------------|------|
| `majordomo prep` | Classify, cluster, batch, write manifest |
| `majordomo dispatch` | Exec OpenCode via `agent-dispatch.sh` |
| `majordomo orchestrate` | Waves, checkpoints, finalize, prose, summary/tech loops, tech-deep |
| `majordomo publish` | Post summary (`--scm github\|gitlab\|bitbucket`) |
| `majordomo status` | Commit/check status per SCM |
| `majordomo report junit` | Findings → JUnit XML |
| `majordomo report html` | Markdown → HTML |
| `majordomo report all-diffs` | Concatenate staging diffs for synthesis |
| `majordomo cache` | Review-cache + poll-cursor helpers |
| `majordomo poll` | SCM API poll; default trigger ingress |
| `majordomo sa` | Run `staticAnalysis` tools into `.sa/` from central config |
| `majordomo build-sa-tools` | Local SA image smoke builds |
| `majordomo submodule` | Interactive vendored-submodule manager |

### Do not port to Go

| Asset | Reason |
|-------|--------|
| `agents/skills/*`, personas | Prompt content |
| OpenCode | External agent runtime ([opencode.ai](https://opencode.ai)) |
| SA tool images | Third-party linters |

### Proposed module layout

```text
majordomo/                          (this repo, new paths)
├── cmd/majordomo/
├── internal/
│   ├── config/                     # YAML central + per-repo merge
│   ├── poll/                       # SCM API poll (default trigger)
│   ├── staging/                    # prep
│   ├── cluster/                    # dep + doc clusters
│   ├── cache/
│   ├── diff/
│   ├── orchestrate/
│   ├── agent/                      # OpenCode exec, env, mounts
│   ├── publish/                    # github, gitlab, bitbucket
│   ├── status/
│   └── report/                     # junit, html
├── agents/                         # unchanged
├── docker/
│   ├── Dockerfile.majordomo        # distroless + static binary
│   ├── Dockerfile.agent            # OpenCode only
│   ├── Dockerfile.gh               # GitHub CLI (job container)
│   └── Dockerfile.glab             # GitLab CLI (job container)
└── pipelines/scripts/              # legacy; deprecated per subcommand
```

### Container images

| Image | Contents |
|-------|----------|
| `majordomo` | Static Go binary only (~15–30 MB) |
| `majordomo-agent` | OpenCode + git; no Python orchestration |
| `majordomo-gh` | `gh` + git/jq; forge job container (majordomo binary built in-job) |
| `majordomo-glab` | `glab` + git/jq; forge job container |
| `sa-tools/*` | Existing ruff/eslint/… images (unchanged) |

---

## Control-tower repo

See [Decisions → Control-tower repository](#control-tower-repository) for the two-repo split.

This repository (`behaviorengineering/majordomo`) is **never** the workflow host — it is consumed as `.majordomo/` inside the control-tower repo. The tower owns config and GHA entrypoints; this repo owns review code.

---

## GitHub Actions workflows (sketch)

### Pull mode (default) — `majordomo-poll.yml`

```yaml
# control-tower/.github/workflows/majordomo-poll.yml
on:
  schedule:
    - cron: '*/5 * * * *'   # interval from config may tune per env
  workflow_dispatch:        # manual poll for debugging

jobs:
  poll:
    runs-on: ubuntu-latest   # or self-hosted for internal SCM
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive

      - name: Discover and queue reviews
        run: |
          docker run --rm \
            -v "${{ github.workspace }}:/workspace" \
            -e MAJORDOMO_CONFIG_DIR=/workspace/majordomo-central-config \
            ghcr.io/behaviorengineering/majordomo \
            poll --default-interval 5m
          # exits 0; enqueues or directly runs reviews for changed PRs/MRs
```

### Review job — `majordomo-review.yml`

Triggered by poll, `repository_dispatch` (push modes), or `workflow_call`.

```yaml
# control-tower/.github/workflows/majordomo-review.yml
on:
  repository_dispatch:
    types: [majordomo-review]
  workflow_call:
    inputs:
      repo_id: { required: true, type: string }
      review_id: { required: true, type: string }
      # ... head_sha, change_id, scm

concurrency:
  group: majordomo-${{ github.event.client_payload.review_id || inputs.review_id }}
  cancel-in-progress: true

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive

      - name: Clone served repository
        run: |
          # git clone using MAJORDOMO_CREDENTIAL_<repo_id> or GH_TOKEN_<OWNER> / GITLAB_TOKEN_<OWNER>

      - name: Run review
        run: |
          docker run --rm \
            -v "${{ github.workspace }}:/workspace" \
            ghcr.io/behaviorengineering/majordomo \
            orchestrate --config /workspace/majordomo-central-config/<repo_id>.yaml

      - uses: actions/upload-artifact@v4
        with:
          name: review-${{ github.event.client_payload.review_id }}
          path: review-output/

      - name: Publish
        run: |
          docker run ... majordomo publish --scm ... --mode auto
```

---

## Phased rollout

### Phase 0 — Document and scaffold (done)

- [x] Architecture plan (this document)
- [x] Agree on `repo_id` naming, payload schema, YAML config shape (loader fields listed under Central config; pipelines/SA still flag-driven)
- [x] Create control-tower repo skeleton (`xynova/majordomo-tower`)
- [x] Init Go module (`go.mod`, `cmd/majordomo`, package stubs; `go test ./...` green)
- [x] Rename review cache branch to `majordomo-pr-reviewer-cache/<project-id>`
- [x] Pin `.majordomo` submodule in tower → `behaviorengineering/majordomo`

### Phase 1 — Go control plane

Port deterministic pipeline logic to Go. Tower poll/workflows stay stubs until the binary can prep, orchestrate, and publish.

**1a — Staging and reports (start here)**

- [x] `majordomo prep` — classify, cluster, batch + Go tests
- [x] `majordomo report junit` / `html` / `all-diffs`
- [x] Dependency + doc clustering in Go (`internal/cluster`)

**1b — Orchestration and agent bridge**

- [x] `majordomo orchestrate` — waves, checkpoints, finalize, prose, loops, tech-deep
- [x] `majordomo dispatch` — OpenCode via `agent-dispatch.sh`
- [x] Summary / tech score loops in Go

**1c — Publish and cache**

- [x] `majordomo publish` — GitHub, GitLab (`glab`), Bitbucket HTTP
- [x] `majordomo status` — commit/check status
- [x] `majordomo cache` — review + poll cursor + cluster precheck/lookup/store/restore

**1d — Tooling**

- [x] `majordomo build-sa-tools` / `majordomo submodule`
- [x] Pipeline Python removed; bash retained for dispatch + image build

### Phase 2 — Pull mode end-to-end (tower wiring)

Requires enough Go from Phase 1 to run prep → orchestrate → publish.

- [x] Wire `majordomo-poll.yml` to build/run Go binary from `.majordomo`
- [x] Wire `majordomo-review.yml` to `majordomo orchestrate` + `publish`
- [x] Document beginner onboarding (credentials + YAML)
- [ ] Wire agent image + LLM provider secrets into `majordomo-review` (required for real review output)
- [ ] One pilot served repo on **pull mode** — meet [Pilot definition of done](#pilot-definition-of-done-pull-mode)

### Phase 3 — OpenCode + slim images

- [x] `Dockerfile.agent` with [OpenCode](https://github.com/anomalyco/opencode)
- [x] `majordomo dispatch` wraps OpenCode CLI
- [x] Agent image CI (`majordomo-agent.yml`)

### Phase 4 — Multi-SCM + optional push triggers

- [x] GitLab poll (+ GitHub poll pagination)
- [x] `publish/gitlab` via `glab` (GitHub publish via `gh`; Bitbucket remains HTTP)
- [ ] Bitbucket poll
- [ ] Optional: `push-workflow` stub template + `repository_dispatch` for willing repos
- [ ] Optional: webhook router under `triggers/` (push-webhook mode)
- [ ] `generic` SCM — clone + artifact-only reviews

### Phase 5 — Cache hardening and polish

- [x] Cluster cache precheck / lookup / store / restore
- [ ] Optional: move poll cursor from Actions cache to served-repo branch `majordomo-poll-cache/<repo-id>`
- [ ] Checks API annotations from JUnit
- [x] Docs refreshed for Go + OpenCode
- [x] Load `pipelines` / `staticAnalysis` from central YAML into Go (`majordomo sa` + prep materialize)

---

## Testing strategy

| Layer | Approach |
|-------|----------|
| Go units | `go test` per package |
| Staging fixture | Temp git repos + expected `manifest.json` / `batch-plan.json` shape |
| Publishers | HTTP mocks / injectable runners against GitHub/GitLab/Bitbucket APIs |
| Integration | Control-tower workflow `workflow_dispatch` with fixture payload |
| Agent | Smoke test: `majordomo dispatch` with stub OpenCode in CI |

---

## Open questions

_(none blocking pilot)_

Deferred (tracked under Phase 4 / 5 checkboxes): Bitbucket poll, push triggers, served-repo poll cursor, Checks annotations.

## Resolved decisions

| Topic | Decision |
|-------|----------|
| **Poll cursor (v1)** | Actions cache + `.poll-cache/<repo-id>/poll-cursor.json` on the tower. Served-repo git branch is optional Phase 5 hardening, not required for pilot. |
| **Credential model** | One forge token **per org/group** in tower secrets: `GH_TOKEN_<OWNER>` (GitHub; never `GITHUB_TOKEN_*` — Actions forbids that prefix) or `GITLAB_TOKEN_<OWNER>` (GitLab). Optional per-repo override `MAJORDOMO_CREDENTIAL_<REPO_ID>`. Lookup order: per-repo → org. **No** unqualified `GH_TOKEN` / `GITLAB_TOKEN` / `GITHUB_TOKEN` for served-repo access (tower Actions `GITHUB_TOKEN` remains for operating on the tower itself). Map secrets into job env explicitly. |
| **Continuous runs** | `review.enableContinuousRuns` default **false** (one review per PR number). Set **true** to re-queue when `head_sha` changes. |
| **Cache skips** | Analysis-cache skips **on by default**; opt out with `cache.disableSkips: true`. |
| **OpenCode auth** | Provider API keys are **per-run job secrets/env** (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `OPENCODE_PROVIDER_API_KEY` for custom OpenAI-compatible gateways). Never bake keys into the agent image. Optional non-secret provider config (`baseURL`, provider id) via `opencode.json` / `OPENCODE_CONFIG_CONTENT` with `{env:...}`. SCM tokens remain separate from LLM auth. `agent-dispatch.sh` preflights provider keys; it no longer requires `GITHUB_TOKEN` for Copilot CLI. |
| **Forge publish** | GitHub/GitLab publish uses **`gh` / `glab` on PATH** inside separate job containers (`majordomo-gh`, `majordomo-glab`). Majordomo Go binary is built in-job and not baked into forge images. Bitbucket publish stays HTTP until a Bitbucket CLI path exists. Poll remains Go HTTP. |

---

## Related docs

| Doc | Relevance |
|-----|-----------|
| [01 — Portable Pipeline Pattern](01-portable-pipeline-pattern.md) | Submodule + config separation |
| [02 — Setup](02-setup.md) | Local CLI and image builds |
| [04 — How the Review Works](04-how-the-review-works.md) | Review behaviour |
| [advanced/05-file-orchestration.md](advanced/05-file-orchestration.md) | Staging and waves |
| [advanced/09-customising-the-review.md](advanced/09-customising-the-review.md) | YAML config mapping |

---

## Changelog

| Date | Change |
|------|--------|
| 2026-08-22 | Initial draft from architecture discussion |
| 2026-08-23 | Control-tower is a separate repo; pins `majordomo` as `.majordomo/` submodule |
| 2026-08-23 | Review cache stays on served repo (`cache.repo: served`) |
| 2026-08-23 | **Poll cache** on served repo branch `majordomo-poll-cache/<repo-id>` (`poll-cursor.json`) |
| 2026-08-24 | Review cache branch renamed `copilot-pr-reviewer-cache` → `majordomo-pr-reviewer-cache` |
| 2026-08-24 | Go module scaffold: `cmd/majordomo`, `internal/*` stubs; Phase 0 started |
| 2026-08-24 | Control tower locked: `xynova/majordomo-tower`; pipeline stays `behaviorengineering/majordomo` |
| 2026-08-24 | **Priority shift:** Go control plane (Phase 1) before tower poll wiring (Phase 2) |
| 2026-08-26 | OpenCode auth resolved: runtime provider API keys |
| 2026-08-26 | Forge CLI images `majordomo-gh` / `majordomo-glab`; publish via `gh`/`glab` on PATH |
| 2026-08-26 | Pipeline Python removed; historical proposal/revision docs deleted |
| 2026-08-26 | Credential model: per-org `GH_TOKEN_*` / `GITLAB_TOKEN_*` + optional `MAJORDOMO_CREDENTIAL_*`; no unqualified served-repo tokens |
| 2026-08-27 | Wire nested `review.publishMode` / `review.enableContinuousRuns` into config + poll |
| 2026-08-27 | Cache skips default on; opt out with `cache.disableSkips: true` (replaces `enableSkips`) |
| 2026-08-27 | Plan sync: poll cursor = Actions `.poll-cache` (v1); schema vs loader split; pilot DoD + agent requirement; continuous-runs in poll algorithm |
| 2026-08-27 | Load `pipelines` + `staticAnalysis` in Go; materialize for prep; `majordomo sa` + tower review step |
