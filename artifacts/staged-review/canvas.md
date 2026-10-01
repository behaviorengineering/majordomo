# Staged Review Canvas: cmd-internal

## Progress

| Stage | Pass | Status | Score |
|---|---|---|---|
| 1. Automated Tools | A | done | 8/10 |
| 2. Type Safety | A | done | 10/10 |
| 3. Error Handling | A | done | 4/10 |
| 7. Code Clarity | A | done | 6/10 |
| 8. Generation Gates | A | done | 3/10 |
| 4. Architecture | B | in progress | pending |
| 5. Robustness | B | pending | pending |
| 6. Testability | B | pending | pending |

Counts: critical 2, medium 11, low 5, open 1.

## Now

Stage 4 question: Should the process-policy seam be one injected interface or factory across static analysis, SA image builds, and submodule Git operations?

Why this matters: One seam keeps retry, breaker, and context rules consistent. Separate seams keep each operation simpler but can drift over time.

Location: `internal/ops/command/command.go:16-56`, `internal/ops/sa/sa.go:137-163`, `internal/ops/satools/satools.go:232-253`, `internal/ops/submodule/git.go:229-251`.

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| Lint tooling unavailable | Medium | Environment | Still open, no `golangci-lint` binary |
| Mechanical findings | Critical, Medium, Low | `cmd/`, `internal/ops/`, `pkg/review/reviewrun/` | Fixed in Pass A |
