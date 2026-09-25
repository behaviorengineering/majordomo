---
name: majordomo-local-seed
description: >-
  Operate filesystem-only Majordomo context digest seeding with --local-seed-dir:
  checkpoint resume, local inference cache, no forge token or branch push.
  Use when testing digest stages locally, resuming an interrupted seed, or
  validating teaching context without opening a context PR.
---

# Majordomo local seed workspace

**Moral:** Local seed workspaces are the durable home for unfinished teaching
context and inference skips during testing. They MUST stay on the filesystem
until a human promotes them. They MUST NOT push context or inference-cache branches.

**Related:** generator fingerprint rules live in
[`majordomo-inference-cache`](../majordomo-inference-cache/SKILL.md). Load that
skill when changing Lookup/Store keys; load this skill when operating or wiring
`--local-seed-dir`.

---

## When to load

- Starting or resuming a digest seed without a context PR
- Choosing `--local-seed-dir` vs `--resume-pr` vs a normal publish digest
- Debugging checkpoint resume, workspace locks, or local cache skips
- Documenting how to validate or discard a local teaching tree

---

## Workspace layout

```text
<local-seed-dir>/
  workspace.yaml
  context/
  analysis/
  inference-cache/
  work-story/
  local.diff
```

| Path | Role |
|------|------|
| `workspace.yaml` | repo_id, source_sha, completed_stage, lock provenance |
| `context/` | Teaching tree; `majordomo context validate --dir …/context` |
| `analysis/` | Persisted typology draft inputs for refine resume |
| `inference-cache/` | DigestStore files; no remote push configuration |
| `work-story/` | Default RLM / module traces when `--work-story-dir` omitted |

---

## Constraints

**CONSTRAINT:** Local mode (`--local-seed-dir`) MUST NOT require a forge token,
MUST NOT fetch or push `majordomo-context/*`, and MUST NOT configure DigestStore
push to `majordomo-inference-cache/*`.

- Enforcement: `Run` selects local mode before token resolution; unit tests with
  a bare remote assert no context/cache refs
- Violation: STOP, remove forge/push paths from local mode, re-verify

**CONSTRAINT:** Local mode MUST still fail closed on missing LLM provider
credentials or typology binary when the requested stage needs them. The error
MUST be a model/tool error, never a forge auth error.

- Enforcement: local path skips `resolveToken`; stage runners keep existing
  judge/typology requirements
- Violation: STOP, fix credential gating, re-verify

**CONSTRAINT:** `workspace.yaml` MUST pin `repo_id` and workdir `HEAD` at
creation. Reopen with a different repo_id MUST fail. A moved HEAD MUST fail
unless `--allow-source-move` is set.

- Enforcement: `OpenLocalSeedWorkspace` identity checks + tests
- Violation: STOP, restore fail-closed identity checks

**CONSTRAINT:** Successful stages MUST advance `completed_stage` only after
artifacts are persisted. Failed stages MUST record `last_error` without
advancing the checkpoint.

- Enforcement: `SaveCheckpoint` / `RecordFailure` + resume tests
- Violation: STOP, fix checkpoint write order

**CONSTRAINT:** Concurrent processes on the same `--local-seed-dir` MUST be
blocked by `workspace.lock` (PID + timestamp, stale lock reclaim). Corrupt
`workspace.yaml` MUST fail closed with a repair/delete hint.

- Enforcement: lock acquisition + corrupt parse tests
- Violation: STOP, restore lock/atomic write behavior

**CONSTRAINT:** `--local-seed-dir` MUST reject `--resume-pr`. `--from-stage survey`
MUST only create a new workspace. Changing `--module-scope` on resume MUST fail
closed. `--skip-story` MUST conflict with `--from-stage story`.

- Enforcement: `validateLocalSeedOptions` + CLI help
- Violation: STOP, tighten flag validation

**CONSTRAINT:** Promotion to a context PR is manual in v1. Agents MUST NOT invent
an auto-upload from the local workspace. Discard is a filesystem delete.

- Enforcement: docs + this skill; no publish helper in local mode
- Violation: STOP, remove auto-publish wiring

CORRECT:
```bash
majordomo context digest --repo-id gitboard --workdir ~/src/gitboard \
  --local-seed-dir tmp/seeds/gitboard

majordomo context digest --repo-id gitboard --workdir ~/src/gitboard \
  --local-seed-dir tmp/seeds/gitboard --from-stage story

majordomo context validate --dir tmp/seeds/gitboard/context
```

PROHIBITED:
```bash
# Local seed that still requires forge auth before opening the workspace
majordomo context digest --local-seed-dir tmp/seeds/gitboard --resume-pr 41

# Silent rebase when workdir HEAD moved
# (must fail without --allow-source-move)
```

---

## Operator recipe

1. Start: `--local-seed-dir <dir>` (optional `--from-stage survey`).
2. Interrupt: leave the directory; `completed_stage` stays at the last success.
3. Resume: same `--local-seed-dir` with `--from-stage catalog|intervention|story`.
4. Inspect: `local.diff`, `workspace.yaml`, `majordomo context validate --dir <dir>/context`.
5. Promote later: copy or re-seed into the normal digest publish path (manual).
6. Discard: `rm -rf <dir>` (no destructive CLI flag in v1).

Prune growth under `work-story/`, `inference-cache/`, and `analysis/` by deleting
old workspaces when disk use becomes an issue.

---

## Checklist

- [ ] Local mode selected before forge token resolution
      Method: `isLocalSeedMode` early return in `Run`
      Pass: no forge token env required for workspace open
      Fail: STOP, move local branch earlier in `Run`
- [ ] Cache is filesystem-only
      Method: DigestStore rooted at `<seed>/inference-cache` with no `ConfigurePush`
      Pass: logs `cache_mode=local context_publish=disabled`
      Fail: STOP, remove push config
- [ ] Checkpoint resume skips completed generators
      Method: unit test with stub survey/refine/story counts
      Pass: survey not recalled on refine resume
      Fail: STOP, fix stage gate
- [ ] Identity and lock fail-closed
      Method: unit tests for repo mismatch, HEAD move, concurrent lock
      Pass: errors as specified
      Fail: STOP, restore guards
