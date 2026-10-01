# Staged Review Progress

## Progress

| Stage | Status | Score |
|---|---|---|
| 1. Automated Tools | Done | 8/10 |
| 2. Type Safety | Done | 10/10 |
| 3. Error Handling | Done | 5/10 |
| 4. Architecture | In progress | |
| 5. Robustness | Not started | |
| 6. Testability | Not started | |
| 7. Code Clarity | Done | 7/10 |
| 8. Generation Gates | Done | 4/10 |

Pass A: mechanical review complete, commit `343ea8f`. Pass B: consultant review.

Counts: critical 0, medium 0, low 0, open 1.

## Now

Question: `internal/ops/process/process.go:18-31`. Is one shared subprocess boundary intentional for submodule, static-analysis, and SA-tool operations, or should each operation own a narrower process client?

## Resolutions

| Kind | Severity | Location | Outcome |
|---|---|---|---|
| Auto-fixed | Medium | Review environment | Installed golangci-lint with Go 1.27; lint passes. |
| Auto-fixed | Medium | `internal/ops/cli/root.go` | CLI output errors are returned, and dispatch initializes tracing. |
| Auto-fixed | Medium | `internal/ops/sa/sa.go` | Tool failures are aggregated and returned. |
| Auto-fixed | Medium | `internal/ops/satools/satools.go` | Docker tag failures are surfaced. |
| Auto-fixed | Medium | `internal/ops/submodule/git.go` | Git probe failures fail closed. |
| Auto-fixed | Low | `internal/ops/submodule/commands.go` | Worktree cleanup failures are reported. |
| Auto-fixed | Low | `internal/ops/cli/root.go`, `internal/ops/submodule`, `internal/ops/sa` | Comments and operator strings use compliant punctuation. |
| Auto-fixed | Medium | `internal/ops/process` | Outbound commands use deadlines, retries, jitter, and circuit breakers. |
| Auto-fixed | Low | `Makefile` | Added standard quality and build verbs. |
| Auto-fixed | High | `cmd/majordomo/main.go` | Deferred gateway and telemetry cleanup now runs before exit codes are returned to the OS. |

## Current Consultant Question

Why this matters: A shared boundary keeps retry and deadline behavior consistent, but it can also couple unrelated command types and make circuit-breaker behavior harder to reason about. The answer determines whether this package is a deliberate platform seam or an architecture finding.
