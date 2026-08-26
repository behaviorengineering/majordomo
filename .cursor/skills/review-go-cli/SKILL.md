---
name: review-go-cli
description: Reviews Majordomo CLI dispatch and Cobra wiring in internal/cli. Use when adding subcommands, changing flags, or after editing cmd/majordomo or internal/cli.
---

# Review Go CLI (majordomo)

**Spec sources:** `internal/cli/root.go`, `cmd/majordomo/main.go`, `docs/PLAN-control-tower-github-go.md`.

---

## 1. Scope

Default: `cmd/majordomo/main.go` and `internal/cli/`. Narrow if user names a subcommand only.

---

## 2. Entry pattern

- [ ] `main` only constructs root and executes; no business logic in `main`.
- [ ] Root command `Use` is `majordomo`.
- [ ] Subcommands registered on root via `AddCommand`.

---

## 3. Dispatch pattern

- [ ] Each subcommand uses `RunE` (or `Run` for trivial `version`) and returns `error`.
- [ ] Unknown command handled by Cobra defaults.
- [ ] No `os.Exit` inside `internal/cli` domain calls.

---

## 4. Subcommands (current surface)

Check wiring matches intent in the plan:

| Command | Domain |
|---------|--------|
| `version` | prints `Version` |
| `poll` | `internal/poll` |
| `prep` | `internal/staging` |
| `dispatch` | `internal/agent` |
| `orchestrate` | `internal/orchestrate` |
| `publish` | `internal/publish` |
| `status` | `internal/status` |
| `cache` | `internal/cache` |
| `report` | `internal/report` |

- [ ] Flags live on the owning subcommand.
- [ ] Options structs passed into domain `Run` / helpers.
- [ ] Errors printed by Cobra / root; exit non-zero on failure.
- [ ] Stubs (`not implemented yet`) are explicit, not silent success.

---

## 5. UX and docs

- [ ] Short/Long help text matches behavior.
- [ ] `-h` / `--help` works on root and subcommands.
- [ ] New commands mentioned in README or plan when user-facing.

---

## 6. Tests

- [ ] New flags or commands have tests where feasible (parse helpers or `cobra` execute with buffers).
- [ ] Exit / error behavior covered when non-obvious.

---

## 7. Report

Summary: Compliant / Gaps. List each failed check with file:line. Recommendations point to domain package if logic belongs outside `cli`.

---

## 8. Fix (if user asked)

Keep `cli` thin; move new behavior to the matching domain package. Update help text. Run `go test ./...`.
