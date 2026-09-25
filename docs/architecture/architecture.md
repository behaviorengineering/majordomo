# Majordomo architecture (as-is)

How the Go control plane in this module is put together today. This is the human architecture narrative. The Typology catalog (`.typology/typology.yaml`) and `docs/develop/` DocPages are the machine-checked inventory of slices, packages, and bindings. Keep them aligned when either changes.

Audience: maintainers and agents who need to understand the system before changing packages, workflows, or the catalog.

## Product shape

Majordomo is a **control plane for evolving repositories**. The product boundary is not “PR review alone.” PR review is the first end-to-end job on that plane (prep → waves → publish). Durable org config, SCM adapters, cache, and job runners live here so work compounds across repos over time.

Two repos cooperate:

| Repo | Role |
|------|------|
| `behaviorengineering/majordomo` (this module, often pinned as `.majordomo/`) | Pipeline **code**: Go binary, domain packages, dockerfiles, tests |
| Control tower (for example `xynova/majordomo-tower`) | Pipeline **deployment**: submodule pin, `majordomo-central-config/`, GitHub Actions that burn runner minutes |

Default onboarding is **pull mode**: the tower polls SCM APIs; served application repos stay clean (no Majordomo workflows or config by default).

## Bounded contexts (Typology slices)

After discover and human cluster-pass, the as-is map is three bounded contexts:

```text
                    ┌──────────────────────────────┐
                    │         operations           │
                    │  poll, config, CLI, SCM auth │
                    │  aigateway, observability  │
                    └─────────────┬────────────────┘
                                  │ launches / hosts
              ┌───────────────────┼───────────────────┐
              ▼                                       ▼
   ┌─────────────────────┐                 ┌─────────────────────┐
   │       review        │◄───────────────►│      context        │
   │ prep → judge → post │  grounding /    │ branch memory,      │
   │                     │  digest feedback│ digest, gate, packs │
   └─────────────────────┘                 └─────────────────────┘
```

| Slice | Business why | Develop docs |
|-------|--------------|--------------|
| **operations** | Host boundary: CLI, poll reconciliation, shared config, outbound HTTP, git HTTPS auth, AI gateway loopback, telemetry, submodule tooling | [overview](../develop/operations/overview.md) · [components](../develop/operations/components.md) · [cli](../develop/operations/cli.md) |
| **review** | PR/MR review workflow: stage diffs, SA, orchestrate waves, judge, cache, report, publish, status | [overview](../develop/review/overview.md) · [components](../develop/review/components.md) |
| **context** | Durable served-repo context branch: validate tree, digest catch-up, human gate, grounding packs | [overview](../develop/context/overview.md) · [components](../develop/context/components.md) |

Catalog source of truth: [`.typology/typology.yaml`](../../.typology/typology.yaml).

### What is not a slice here

- Temporal steps inside review (`staging` → `orchestrate` → `judge` → `publish`) stay packages inside **review**.
- AI evaluation (strop judge, gateway, workspace port) is a **capability** owned by review (and used by context digest), not a fourth pillar.
- The cobra CLI is a **surface** on **operations**, not a `cli` bounded context.

## Runtime paths

### Review path (primary job)

Operators and tower workflows drive this through the **operations** CLI surface.

```text
poll (operations)
  → discover open PRs/MRs vs poll cursor
  → majordomo run review / orchestrate (CLI → review)
      → sa (static analysis into .sa/)
      → staging / cluster / diff (prep manifest)
      → orchestrate waves
          → filereview state machine
          → judge (in-process strop) + agent dispatch
          → workspace port when a job needs explore/edit
      → report (junit / html / all-diffs)
      → cache (served-repo review cache branches)
      → publish / status (forge side effects)
```

Typical CLI entrypoints (full list on [operations CLI](../develop/operations/cli.md)):

- `majordomo poll`
- `majordomo prep …` / `majordomo orchestrate` / `majordomo run review`
- `majordomo dispatch …` (one Judge batch)
- `majordomo publish …` / `majordomo status …`
- `majordomo cache …` / `majordomo report …` / `majordomo sa`

### Context path (durable memory)

Separate lifecycle on the served repo’s orphan context branch (`majordomo-context/<repo-id>` in the product plan).

```text
majordomo context repos | digest | validate | gate
  → contextstore (tree / meta / chronology checks)
  → agenting (select grounding packs)
  → contextdigest (catch up when cursor behind default HEAD)
  → contextgate (human reject / done conversation for Gate)
  → may read review outputs and operations config / forge helpers
```

Context updates are meant to teach the next review, not to replace the PR review workflow.

### Host path (operations)

```text
cmd/majordomo
  → observability + aigateway process lifecycle
  → internal/cli (cobra)
       → poll, config load, submodule, …
       → delegates into review and context packages
```

Shared leaves inside operations: `config` (central YAML), `outbound` (retrying HTTP), `githttps` (git auth headers), `observability` (OTel), `aigateway` (Bifrost loopback for OpenAI-shaped clients).

## Package map by slice

### operations

| Package | Role |
|---------|------|
| `cmd/majordomo` | Process entry; shutdown hooks for gateway and telemetry |
| `internal/cli` | Cobra wiring; thin `RunE` into domain packages |
| `internal/poll` | SCM poll + cursor comparison |
| `internal/config` | Load/merge `majordomo-central-config` |
| `internal/outbound` | Shared HTTP client for forges |
| `internal/githttps` | `http.extraHeader` args for git HTTPS |
| `internal/aigateway` | In-process Bifrost / loopback chat completions |
| `internal/observability` | Tracing and failure dumps |
| `internal/submodule` | Interactive `.majordomo` submodule manager |

### review

| Package | Role |
|---------|------|
| `internal/staging`, `cluster`, `diff` | Classify, cluster, batch, combined diffs from manifests |
| `internal/sa`, `satools` | Run / build static analysis tooling |
| `internal/orchestrate`, `filereview`, `reviewrun` | Waves, per-file machine, local/CI review job |
| `internal/judge` (+ evaluation packs, modules) | strop JobRunner boundary |
| `internal/agent`, `workspace`, `workspace/opencode` | Dispatch helpers and workspace port |
| `internal/report`, `cache` | Artifacts and served-repo cache branches |
| `internal/publish`, `status` | PR/MR summaries and commit status |

### context

| Package | Role |
|---------|------|
| `internal/contextstore` | Validate context-branch tree |
| `internal/contextdigest` | Catch-up job when cursor is behind |
| `internal/contextgate` | Gate reject / regenerate options from PR conversation |
| `internal/agenting` | Load and select grounding packs |

## Couplings (why the slices read each other)

From [`.typology/typology.yaml`](../../.typology/typology.yaml) `sliceBindings` (all `reads` today):

| From → To | Why it exists |
|-----------|----------------|
| operations → review | CLI and poll launch review jobs and publish paths |
| review → operations | Review needs config, SCM auth helpers, telemetry, gateway |
| operations → context | Host triggers digest / validate / gate |
| context → operations | Digest and gate use shared config and forge helpers |
| review → context | Review consumes grounding packs and context history |
| context → review | Digest / gate examine review-shaped outputs when updating the branch |

Validate these edges with:

```bash
go tool typology validate .
```

That checks catalog structure, owned paths, and import consistency against the catalog. It does **not** yet check that this prose narrative still matches the catalog.

## Deployment sketch (tower)

```text
SCM (GitHub / GitLab / Bitbucket / …)
        │ poll / clone / publish
        ▼
control tower (GHA)
  ├── majordomo-central-config/
  ├── .majordomo/   ← this module @ pin
  └── workflows: poll.yml, review.yml, …
        │
        ▼
   majordomo binary (+ optional agent / SA images)
        │
        ▼
   served repo checkout (product code; optional cache / context branches)
```

Longer product plan: [`docs/PLAN-control-tower-github-go.md`](../PLAN-control-tower-github-go.md).

## How Typology fits this doc

| Artefact | Job |
|----------|-----|
| `.typology/typology.yaml` | Machine map: slices, owns, surfaces, bindings |
| `docs/develop/<slice>/…` | Per-slice inventory leaves (overview, components, cli) |
| **This file** | Assembled system narrative: paths, why, deployment |

Today an agent fills DocPages one leaf at a time from catalog tables. Producing **this** narrative still requires human or agent judgment outside Typology’s emit templates.

## Change checklist

When you change package layout or domain boundaries:

1. Rediscover into `tmp/typology/typology.yaml`, apply the cluster-pass, then update `.typology/typology.yaml`.
2. `go tool typology validate .`
3. Update this narrative if runtime paths or slice meaning changed.
4. Update the affected `docs/develop/<slice>/` pages (and remove `<!-- typology:generated -->` only after intentional human/agent fill).
5. Record any new Typology product gaps in the upstream issue tracker.
