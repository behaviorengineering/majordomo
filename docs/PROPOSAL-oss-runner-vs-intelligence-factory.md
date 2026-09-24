# Open runner vs closed intelligence factory

*Majordomo — product boundary note. Packaging is staged; this documents the live contract.*

## Why this note exists

Majordomo’s lasting value is not “another PR bot.” It is **reviews that remember the repository**, with evidence and a teaching story that evolves as default moves. Competitors can copy a forge poller. Copying the factory that builds that memory is what we want to make hard.

This note draws a clear line: what may live in open source, what stays proprietary, and how the two talk.

## Two tools, one handshake

```text
  [ Closed factory ]  builds / updates context packs on orphan branch
           |
           |  finished packs + provenance (signed when we ship that)
           v
  [ Open runner ]     polls PRs, stages diffs, attaches packs, reviews, publishes
```

- **Open runner:** uses intelligence once it exists.
- **Closed factory:** creates and evolves that intelligence.
- **Primary consumer of context:** Majordomo’s own reviews. Curious humans may read the teaching story; IDE agents are not the design center.

## Open (review runner)

Keep this useful even when the factory is absent or unpaid.

| In plain English | Rough home today |
|------------------|------------------|
| Find PRs that need work | `pkg/review/poll` |
| Stage diffs, batch files, route skills | `pkg/review/staging`, `cluster`, `diff` |
| Run review waves and checkpoints | `pkg/review/orchestrate`, `reviewrun`, `filereview` |
| Ask the Judge “is this a defect?” | `pkg/review/dispatch`, `pkg/judge`, workspace hooks |
| Post comments and status | `pkg/forge/publish`, `status`, `pkg/review/report` |
| Remember “already reviewed this SHA” | `pkg/platform/cache` (review / poll cursors) |
| Tower wiring (CLI, config, images, workflows) | `cmd/`, `pkg/platform/config`, `internal/ops/cli`, Docker, Actions |
| **Read and select** existing grounding packs | `pkg/context/agenting` |
| **Validate** context tree shape | `pkg/context/store` (schema / check) |
| Parse human gate comments (`@majordomo …`) | `pkg/context/gate` |
| Mechanical review rubrics | `agents/` (how to review, not who this product is) |

With only this half, teams can still review. They may use hand-written packs, a thin example generator, or no grounding. Reviews work; they do not get Majordomo’s compounding memory for free.

## Closed (intelligence factory)

This is the paid differentiator: evidence-first understanding that compounds.

| In plain English | Rough home today |
|------------------|------------------|
| Catch up when the context cursor is behind default | private `majordomo-context` (`digest` / `repos`) |
| Survey code; roles, grouping, meaning with evidence first | typology path inside digest |
| Write mission, architecture, conventions, weaknesses, chronology | story generation / compaction in digest |
| **Create** agenting packs from the story | digest materialize (not mere select) |
| Prompts, model routing, eval fixtures, contradiction handling | digest internals |
| Digest inference cache under `digest/` on the inference-cache branch | factory binary (imports transitional `pkg/platform/cache` DigestStore) |

Treat license keys and binary obfuscation as friction if a factory ever runs on customer hardware. They are not the moat. The moat is continuously better inference, provenance, incremental catch-up, and a private evaluation corpus.

## Gray line (do not let it drift)

| Job | Open or closed? | Why |
|-----|-----------------|-----|
| Clone context tip and attach packs for a PR | Open | Runner consuming a product |
| Match packs to changed paths / job mode | Open (simple rules) | Needed for any pack source |
| Decide what packs say / regenerate from code | Closed | That is the factory |
| Pack schema + signed manifest format | Open | Others can write packs; we can verify |
| Branch layout (`majordomo-context/…`) | Open convention | Storage contract, not the brain |
| Digest inference fingerprints / stage traces | Closed | How the factory gets copied if leaked |

**Rule of thumb:** if a stranger can reimplement it from forge APIs and “attach these markdown files,” it can be open. If the value is “infer what this repo means and keep that true over time,” it stays closed.

## How they should talk

The factory publishes **finished packs** (preferably signed) onto the orphan context branch, with provenance: source commit, generator version, claim age. The runner downloads the **merged** tip only, selects packs, and reviews.

Do not expose rejected candidates, internal scores, or digest stage traces through the runner API. That leak is how the factory gets copied without cloning the source.

### Compatibility contract (staged)

1. **Judge runtime:** empty `RuntimeOptions.Tasks` registers **review** generators only. Factory callers pass `Tasks: DigestTasks()` and should supply `Generators` (see `modules.DigestGenerators()` until prompts move).
2. **Provider routing:** `config.JobForTask` keeps review + context-digest task names; factories may `RegisterJobForTask` for extra names.
3. **CLI:** public `majordomo` exposes `context validate` / gate read and review/poll cache; it does **not** expose `cache digest-*`. Digest CLI lives on `majordomo-context`.
4. **Cache:** `DigestCachePrefix` (`digest/`) remains the open branch path convention. `DigestStore` stays importable as a transitional API for the private binary until that module owns the store.

## Packaging (in progress)

1. **Open runner:** majordomo product kit under `pkg/<domain>/…` (`context`, `platform`, `forge`, `review`, `judge`); ops only under `internal/ops/`.
2. **Closed factory:** private GitLab module [`majordomo-context`](https://gitlab.com/behaviorengineering/majordomo-context) ships a licensed, optionally garbled binary (`digest` / `repos` / `gate`). Pattern: kairos Ed25519 Gate + GoReleaser `-tags release`. Imports majordomo `pkg/` domains; keeps `internal/digest`, `internal/license`, `internal/ops/cli`.
3. **Not required:** SaaS digest API. Hosted factory remains a later option.
4. **Avoid:** open-core build tags as the only IP boundary.
5. **Next slice:** move bundled digest prompts and `DigestStore` into the private module; leave only schema, selection, and registration seams public.

## One-line pitch

Open Majordomo is the steward that runs reviews on evolving repos. The paid intelligence is the factory that teaches that steward how *this* repo works, with receipts.

## Related

- [Repo context branch](advanced/10-repo-context-branch.md) — orphan branch, cursor, agenting vs mechanical
- [Control tower plan](PLAN-control-tower-github-go.md) — architecture and phases
