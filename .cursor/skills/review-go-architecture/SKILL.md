---
name: review-go-architecture
description: Verifies Majordomo Go layout and dependency direction. Use after adding packages, moving code between internal/, or when the user asks for architecture review.
---

# Review Go Architecture (Majordomo)

**Module:** `github.com/behaviorengineering/majordomo`  
**Binary:** `cmd/majordomo`  
**Plan:** `docs/PLAN-control-tower-github-go.md`

This skill covers **Go code structure** only. Review rubrics for target repos live under `agents/`.

---

## 1. Scope

Ask for target path(s). Default: `internal/` and `cmd/`.

---

## 2. Expected layout

```
cmd/majordomo/main.go       # entry only
internal/cli/               # Cobra wiring, flags, thin RunE
internal/poll/              # SCM poll / reconciliation
internal/staging/           # prep, classify, stage files
internal/cluster/           # dependency clustering
internal/orchestrate/       # plan and run reviews
internal/agent/             # agent dispatch / loops
internal/report/            # JUnit / HTML
internal/publish/           # publish artefacts
internal/cache/             # cursor / cache
internal/contextstore/      # served-repo context branch schema
internal/contextdigest/     # context catch-up job (cursor, orphan seed, forge PR)
internal/workspace/         # cwd-bounded tool port (+ opencode adapter)
internal/judge/             # strop JobRunner boundary + evaluation packs
internal/filereview/        # Prepare→Judge→Validate→Assemble for file batches
internal/reviewrun/         # local/CI review job (clone, SA, orchestrate, publish)
internal/config/            # config load
internal/status/            # status
internal/diff/              # diff helpers
```

Pipeline scripts and Docker images stay under `pipelines/` and `dockerfiles/`; do not re-implement them inside Go without a design note in the plan.

No `internal/services/`, `internal/clients/`, or DI container in this repo. Do not introduce them without an explicit design change.

---

## 3. Dependency rules

| Layer | May import | Must not import |
|-------|------------|-----------------|
| `cmd/` | `internal/cli` | domain packages directly |
| `internal/cli` | domain packages | (none beyond domain) |
| Domain (`poll`, `staging`, …) | other domain/shared packages as needed | `internal/cli` |
| Shared helpers | stdlib + each other minimally | `internal/cli`, thick domain cycles |

**RED FLAGS**

- Domain packages importing `internal/cli`
- Circular imports between domain packages
- Business logic growing inside thick Cobra `RunE` bodies (move to domain package)

Verify:

```bash
go list -f '{{.ImportPath}} -> {{join .Imports "\n"}}' ./internal/...
```

---

## 4. Package responsibilities

### `internal/cli`

- Wire Cobra commands and flags.
- Construct options structs; call domain `Run` / constructors.
- Keep `RunE` thin.

### Domain packages

- Own the behavior named by the package.
- Return `error`; no `os.Exit`.
- Accept deps via options structs or params (`io.Writer`, paths, clients).

---

## 5. Error and test patterns

- Domain functions return `error`; exit only at CLI boundary.
- Wrap errors with `%w` and operation context.
- Tests co-located; use `t.TempDir()` for filesystem tests.
- No live remotes in unit tests.

---

## 6. Report

1. **Summary:** Compliant / Partial / Violations.
2. **Violations:** Rule broken, file/evidence, recommended move or import fix.
3. **Optional:** Suggest package split if a file mixes two domains.

---

## 7. Fix (if user asked)

Move code to correct package; fix imports; `go test ./... && go vet ./...`.
