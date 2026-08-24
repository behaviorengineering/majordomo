# Plan: Control Tower, GitHub Actions, and Go

*Majordomo — repository operations for evolving software.*

**Status:** Draft  
**Date:** 2026-08-22  
**Audience:** Maintainers and contributors planning the next major evolution of Majordomo.

---

## Summary

Majordomo today is a **Jenkins + Bitbucket** pipeline delivered as a **git submodule**, with review logic in **Python/bash** and agents invoked via **GitHub Copilot CLI** inside a fat Docker image.

The target is:

1. **Control-tower repo** — one place for org config and a pinned `.majordomo/` submodule; served repos stay clean (no workflow files by default).
2. **GitHub Actions minutes** — all pipeline compute runs on GitHub-hosted runners in the control tower.
3. **Any git host** — review repos on GitHub, GitLab, Bitbucket, or self-hosted git; SCM API adapters for poll, clone, and publish.
4. **Go control plane** — port deterministic pipeline logic to a single static binary; keep agent and SA tooling in focused containers.
5. **OpenCode** — replace Copilot CLI as the agent runtime (skills/personas remain file-based).
6. **Pull poll (reconciliation)** — `majordomo-poll.yml` runs on every onboarded repo regardless of push config. Push triggers are optional speed boosts. See [Trigger modes and onboarding](#trigger-modes-and-onboarding).

**Jenkins and Groovy are decommissioned.** They are reference material for porting behaviour to Go and GitHub Actions, not a parallel supported path. Bitbucket (and other SCMs) remain valid **targets** for review via webhook adapters — only the Jenkins runtime goes away.

---

## Goals

| Goal | Description |
|------|-------------|
| **No repo pollution** | Served repos do not add `.github/workflows/`, `.majordomo/`, or Majordomo config (default pull mode). |
| **Central onboarding** | Add `majordomo-central-config/<repo>.yaml` + store SCM credential in tower secrets; no app-repo merge required. |
| **Submodule versioning** | Control tower pins `.majordomo` at a commit; bump pointer to roll out pipeline changes. |
| **SCM-agnostic core** | Same review engine for any git remote; adapters for trigger + publish only. |
| **Fast, portable binary** | Go for staging, orchestration, cache, publish — one small container on GHA. |
| **Agent swap** | OpenCode in a slim agent image; no Node/Python orchestration image. |

## Non-goals (for this phase)

- Rewriting agent skills or review rubrics in Go.
- Replacing third-party SA tools (ruff, eslint, etc.) with Go implementations.
- Maintaining Jenkins, Groovy stages, or Jenkins setup scripts in parallel with the new stack.
- Building a hosted SaaS control plane (self-hosted control tower only).

---

## Current state

```text
App repo (Bitbucket)
  └── webhook → Jenkins (per-repo OR central job)

Central Jenkins job SCM = pipeline / control-tower repo
  ├── .majordomo/                    (submodule — this repo)
  ├── majordomo-central-config/
  │     ├── _defaults.groovy
  │     └── <repo-slug>.groovy
  └── MajordomoReview.Central.CI.Jenkinsfile

Runtime
  ├── stages/*.groovy                (orchestration)
  ├── pipelines/scripts/*.py|.sh     (staging, dispatch, publish)
  ├── dockerfiles/copilot-cli        (Node + Python + Copilot CLI)
  └── agents/skills/                 (markdown — unchanged long-term)
```

See [02 — Setup](02-setup.md) and [04 — How the Review Works](04-how-the-review-works.md) for legacy behaviour to preserve in the Go port.

---

## Target architecture

**Default path (pull mode):** control-tower workflow polls SCM APIs; served repos change nothing.

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
| 1 | Jenkins / Groovy | **Decommission.** Port behaviour to Go + GHA, then delete. No parallel maintenance. |
| 2 | Review cache location | **On the served repo** (same as today). Cluster analysis cache branches live in the app repo under review. Poll cache (PR `head_sha` cursors) also lives on the served repo — not on the control-tower. |
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

Behaviour unchanged from [PROPOSAL-review-cache-bitbucket-downloads.md](PROPOSAL-review-cache-bitbucket-downloads.md):

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
  enableSkips: false
```

Go port: `majordomo cache` subcommand replaces `review-cache.py` and `push-to-cache.py` with the same git-branch storage model.

**Poll cache** (separate branch): `majordomo-poll-cache/<repo-id>` with `poll-cursor.json` — see [Poll cache](#poll-cache-where-poll-compares-head_sha).

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

Replace Groovy central config with YAML. Per-repo file: `majordomo-central-config/<repo_id>.yaml`.

```yaml
# majordomo-central-config/payments-api.yaml

scm: gitlab   # github | gitlab | bitbucket | generic

repository:
  id: payments-api
  cloneUrl: https://gitlab.com/acme/payments-api.git

scmApi:
  baseUrl: https://gitlab.com
  projectId: "12345"
  # Credentials: control-tower secret GITLAB_TOKEN__payments-api

review:
  publishMode: auto    # auto | comment | description | check | off
  enableContinuousRuns: false

trigger:
  poll: true             # always on — reconciliation (GitHub cron min 5m)
  interval: 5m
  push:
    mode: none             # none | workflow | webhook

staticAnalysis:
  - tool: ruff
    glob: "**/*.py"

pipelines:
  pr-review:
    model: ...
    routing: ...
    agentContext: ...

cache:
  repo: served           # review cache branches live on the served repo
  retentionDays: 120
  enableSkips: false

pollCache:
  repo: served           # poll cursor branch on the same served repo
  branch: majordomo-poll-cache/<repo-id>   # holds poll-cursor.json
```

Org defaults in `majordomo-central-config/_defaults.yaml` are deep-merged; per-repo keys win.

Existing Groovy config semantics in [09 — Customising the Review](advanced/09-customising-the-review.md) should map 1:1 where possible.

---

## Trigger modes and onboarding

### Pull poll = universal reconciliation

`majordomo-poll.yml` runs on a **5-minute cron** (GitHub’s minimum) for **every onboarded repo**. It asks the SCM API: *which open PRs/MRs have a `head_sha` we haven’t reviewed yet?*

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
| **Dedup** | Skip review if `head_sha` already reviewed — push and poll share the same `review_id`. |

Push modes don’t replace poll — they **front-run** it. If a webhook fires and review completes, the next poll sees `head_sha` already done and skips.

### Why pull-only is enough for most onboardings

| Benefit | Detail |
|---------|--------|
| **No served-repo changes** | Nothing to merge to default branch — avoids PR approver gatekeeping. |
| **No extra hosting** | No webhook router, no GitHub App, no inbound URL on corp network. |
| **Cross-org** | Platform admin issues PAT; no workflow in foreign repos. |
| **Replaces Jenkins GWT** | Outbound API poll instead of inbound Bitbucket webhook to Jenkins. |

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

1. **Platform admin** (customer or yours) creates a machine user or fine-grained PAT with:
   - Read access to the served repo (clone + list open PRs/MRs)
   - Write access to post PR/MR comments (and optional check/status)
2. Store token in control-tower secrets: `MAJORDOMO_CREDENTIAL__<repo_id>`.
3. Add `majordomo-central-config/<repo_id>.yaml` (clone URL, SCM API base; `trigger.push.mode: none` is fine).
4. Ensure the GHA runner can reach the SCM API (public SaaS or **self-hosted runner** on corp network for internal Bitbucket).
5. `majordomo-poll.yml` cron runs → discovers open PRs with new `head_sha` → runs review → publishes.

No webhook URL. No merge to `main` on the served repo.

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

### Poll cache (where poll compares `head_sha`)

Every poll cycle asks: *for this served repo, which PRs have a new `head_sha` since we last reviewed?* That comparison uses the **poll cache** — a small cursor file, separate from cluster review cache.

```text
PR #42  →  last reviewed head_sha: abc123
          current head_sha from API: abc123  → skip
          current head_sha from API: def456  → queue review, update cache
```

| Cache | Location (served repo) | Skips |
|-------|------------------------|-------|
| **Poll cache** | Git branch `majordomo-poll-cache/<repo-id>` (or `poll-cursor.json` on the review-cache branch — TBD at implementation) | **Whole review runs** when PR head unchanged |
| **Review cache** | Git branch `majordomo-pr-reviewer-cache/<project-id>` | **Cluster AI work** inside a run |

Both live on the **served repo** (same decision as review cache). The control-tower does not hold per-repo poll cursors — only config and the submodule pin.

**Poll cache file shape** (`poll-cursor.json` on the poll-cache branch):

```json
{
  "repo_id": "payments-api",
  "updated_at": "2026-08-23T00:12:00Z",
  "prs": {
    "42": { "head_sha": "abc123...", "reviewed_at": "...", "run_id": "12345" }
  }
}
```

**Poll algorithm:**

1. List open PRs/MRs from SCM API (read credential from tower secrets).
2. Clone or fetch `majordomo-poll-cache/<repo-id>` from the **served repo** (or start empty if branch missing).
3. Read `poll-cursor.json`; for each PR, compare `head_sha` to cached value.
4. Queue review for any PR with new or missing sha.
5. After review succeeds, commit updated `poll-cursor.json` and push to the served repo poll-cache branch.

Uses the same write-capable token as review cache publish (already required for comments + cache push).

**Not poll cache:** GitHub Actions cache is optional for in-run speed only — evicts after ~7 days and is not the comparison source of truth.

**Concurrency:** `majordomo-review` uses `concurrency: group: majordomo-${{ review_id }}` so push + poll don’t double-run; poll cache updates after success.

---

## SCM adapters (clone + publish)

Poll, clone, and publish are SCM-specific; trigger mode is independent.

| SCM | Pull (list PRs) | Clone | Publish |
|-----|-----------------|-------|---------|
| **GitHub** | `GET /repos/{owner}/{repo}/pulls` | PAT or deploy key | PR comment, description, Checks API |
| **GitLab** | MR API | `GITLAB_TOKEN` or deploy key | MR note, pipeline status |
| **Bitbucket** | PR API (Server/Cloud) | SSH key or token from config | REST comment/description (Go port of existing script) |
| **Generic** | Manual / `workflow_dispatch` only | Deploy key / PAT | Optional — artifact-only mode |

---

## Go control plane

### Single binary: `majordomo`

Built from this repo (new top-level `cmd/` and `internal/`). Distributed as `ghcr.io/behaviorengineering/majordomo`.

| Subcommand | Replaces | Notes |
|------------|----------|-------|
| `majordomo prep` | `git-diff-prep.py` | Classify, cluster, batch, write manifest |
| `majordomo dispatch` | `copilot-dispatch.sh` | Exec OpenCode with skill/env wiring |
| `majordomo orchestrate` | `copilot-review.groovy` + loop scripts | Waves, checkpoints, finalize |
| `majordomo publish` | `publish-pr-summary.py` | `--scm github\|gitlab\|bitbucket` |
| `majordomo status` | `notify-build-status.py` | Commit/check status per SCM |
| `majordomo report junit` | `review-to-junit.py` | |
| `majordomo report html` | `md-to-html.py` | |
| `majordomo cache` | `review-cache.py`, `push-to-cache.py` | Phase 5 |
| `majordomo poll` | (new) | SCM API poll; default trigger ingress |
| `majordomo sa run` | `run-sa-tool.sh` | `docker run` SA images |

### Do not port to Go

| Asset | Reason |
|-------|--------|
| `agents/skills/*`, personas | Prompt content |
| OpenCode / Copilot CLI | External agent runtime |
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
│   └── Dockerfile.agent            # OpenCode only
└── pipelines/scripts/              # legacy; deprecated per subcommand
```

### Container images

| Image | Contents |
|-------|----------|
| `majordomo` | Static Go binary only (~15–30 MB) |
| `majordomo-agent` | OpenCode + git; no Python orchestration |
| `sa-tools/*` | Existing ruff/eslint/… images (unchanged) |

---

## Control-tower repo

See [Decisions → Control-tower repository](#control-tower-repository) for the two-repo split.

This repository (`behaviorengineering/majordomo`) is **never** the workflow host — it is consumed as `.majordomo/` inside the control-tower repo.

Relationship mirrors the old Jenkins central model: tower owns config and GHA entrypoints; `.majordomo` owns code — without Jenkins in the loop.

---

## Decommission: Jenkins and Groovy

The following are **removed**, not maintained alongside the new stack. Port their behaviour into Go + GitHub Actions, then delete.

| Path | Disposition |
|------|-------------|
| `pipelines/MajordomoReview*.Jenkinsfile` | Delete after `majordomo-review.yml` + `majordomo orchestrate` |
| `stages/*.groovy` | Delete — logic lives in `internal/orchestrate/` |
| `lib/*.groovy` | Delete |
| `scripts/setup-majordomo.py` | Delete — replaced by pull-mode onboarding in this plan |
| `scripts/setup-majordomo-central.py` | Delete |
| `example.majordomo-config.groovy` | Delete — replaced by YAML example |
| `example.majordomo-central-config/*.groovy` | Delete — replaced by YAML |
| `copilot/setup-majordomo-*.prompt.md` | Rewrite for GitHub/control-tower setup (no Jenkins credential IDs) |
| `docs/02-setup.md` (Jenkins sections) | Rewrite for control-tower pull-mode onboarding |

**Keep temporarily as porting reference** until the Go equivalent has tests:

- `stages/copilot-review.groovy` → `internal/orchestrate/`
- `pipelines/scripts/*.py` → Go subcommands (then delete Python)

**Docs to update or archive:** Jenkins-specific setup guides, pipeline stages reference tied to Groovy, `04.1-pipeline-stages-reference.md` (rewrite for GHA + Go).

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
          # git clone using MAJORDOMO_CREDENTIAL__<repo_id> from secrets

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

### Phase 0 — Document and scaffold (current)

- [x] Architecture plan (this document)
- [ ] Agree on `repo_id` naming, payload schema, YAML config shape
- [x] Create control-tower repo skeleton (`xynova/majordomo-tower`)
- [x] Init Go module (`go.mod`, `cmd/majordomo`, package stubs; `go test ./...` green)
- [x] Rename review cache branch to `majordomo-pr-reviewer-cache/<project-id>`

### Phase 1 — Pull mode end-to-end (default path)

- [ ] `majordomo-poll.yml` cron workflow + `majordomo poll` Go subcommand (or script stub)
- [ ] `majordomo-review.yml` + `majordomo orchestrate` minimal path
- [ ] Port `publish` + `status` to Go (GitHub + Bitbucket first)
- [ ] One pilot served repo on **pull mode** — no files in served repo
- [ ] Document beginner onboarding in this plan (trigger section)

### Phase 2 — Go staging and orchestration

- [ ] `majordomo prep` — port `git-diff-prep.py` with tests ported from `tests/pipelines/scripts/`
- [ ] `majordomo orchestrate` — port wave/checkpoint logic from `copilot-review.groovy`
- [ ] `dep_clusters` + `doc_clusters` in Go
- [ ] **Delete** `stages/*.groovy`, `lib/*.groovy`, Jenkinsfiles, setup-majordomo scripts

### Phase 3 — OpenCode + slim images

- [ ] `Dockerfile.agent` with [OpenCode](https://github.com/anomalyco/opencode)
- [ ] `majordomo dispatch` wraps OpenCode CLI
- [ ] **Delete** `dockerfiles/copilot-cli.Dockerfile` and Copilot-specific dispatch paths

### Phase 4 — Multi-SCM + optional push triggers

- [ ] GitLab poll + `publish/gitlab`
- [ ] Bitbucket poll (primary migration path from Jenkins webhooks)
- [ ] Optional: `push-workflow` stub template + `repository_dispatch` for willing repos
- [ ] Optional: webhook router under `triggers/` (push-webhook mode)
- [ ] `generic` SCM — manual dispatch + artifact-only reviews

### Phase 5 — Cache, hardening, and legacy cleanup

- [ ] Port `review-cache.py` / `push-to-cache.py` → `majordomo cache`
- [ ] Checks API annotations from JUnit
- [ ] **Delete** remaining `pipelines/scripts/*.py` and bash orchestration
- [ ] Rewrite `docs/02-setup.md` and archive Jenkins-specific docs

---

## Migration notes (legacy Jenkins → control tower)

Historical mapping for anyone porting config or behaviour. **No Jenkins deployment is planned going forward.**

| Legacy (remove) | Replacement |
|-----------------|-------------|
| Bitbucket webhook → Jenkins central job | **Pull mode:** tower cron polls Bitbucket API (no webhook required) |
| `majordomo-central-config/*.groovy` | `majordomo-central-config/*.yaml` |
| `setup-majordomo-central.py` | Pull-mode onboarding: PAT in tower secrets + YAML config |
| Jenkins credentials store | GitHub Actions secrets (`MAJORDOMO_CREDENTIAL__<repo_id>`) |
| `MajordomoReview.Central.CI.Jenkinsfile` | `majordomo-poll.yml` + `majordomo-review.yml` + `majordomo orchestrate` |
| Per-repo Jenkins job + `setup-majordomo.py` | Not replaced — central tower only; served repos stay clean |

---

## Testing strategy

| Layer | Approach |
|-------|----------|
| Go units | Port existing pytest cases to `go test` per package |
| Staging golden files | Fixture repos + expected `manifest.json` / `batch-plan.json` |
| Publishers | HTTP mocks against GitHub/GitLab/Bitbucket APIs |
| Integration | Control-tower workflow `workflow_dispatch` with fixture payload |
| Agent | Smoke test: `majordomo dispatch` with stub OpenCode in CI |

---

## Open questions

1. **Credential model** — one org secret per SCM vs `MAJORDOMO_CREDENTIAL__<repo_id>` convention?
2. **OpenCode auth** — which provider keys in agent container vs passed per-run?

---

## Related docs

| Doc | Relevance |
|-----|-----------|
| [01 — Portable Pipeline Pattern](01-portable-pipeline-pattern.md) | Submodule + config separation (still applies) |
| [04 — How the Review Works](04-how-the-review-works.md) | Behaviour to preserve in Go orchestrator |
| [advanced/05-file-orchestration.md](advanced/05-file-orchestration.md) | Staging port spec |
| [advanced/09-customising-the-review.md](advanced/09-customising-the-review.md) | YAML config mapping |
| [PROPOSAL-review-cache-bitbucket-downloads.md](PROPOSAL-review-cache-bitbucket-downloads.md) | Cache port reference |

---

## Changelog

| Date | Change |
|------|--------|
| 2026-08-22 | Initial draft from architecture discussion |
| 2026-08-23 | Jenkins/Groovy decommissioned — no parallel maintenance path |
| 2026-08-23 | Control-tower is a separate repo; pins `majordomo` as `.majordomo/` submodule |
| 2026-08-23 | Review cache stays on served repo (`cache.repo: served`) |
| 2026-08-23 | **Poll cache** on served repo branch `majordomo-poll-cache/<repo-id>` (`poll-cursor.json`) |
| 2026-08-24 | Review cache branch renamed `copilot-pr-reviewer-cache` → `majordomo-pr-reviewer-cache` |
| 2026-08-24 | Go module scaffold: `cmd/majordomo`, `internal/*` stubs; Phase 0 started |
| 2026-08-24 | Control tower locked: `xynova/majordomo-tower`; pipeline stays `behaviorengineering/majordomo` |
