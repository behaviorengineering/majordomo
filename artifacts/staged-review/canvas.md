# Staged Review Canvas

## Progress

| Stage | Status | Score |
|---|---|---:|
| 1. Automated Tools | done | 10/10 |
| 2. Type Safety | done | 10/10 |
| 3. Error Handling | done | 10/10 |
| 4. Architecture | in progress | — |
| 5. Robustness | pending | — |
| 6. Testability | pending | — |
| 7. Code Clarity | done | 10/10 |
| 8. Generation Gates | done | 10/10 |

Pass A: mechanical stages 1, 2, 3, 7, 8, complete in commit `2e02dc9`. Pass B: consultant stages 4, 5, 6.

Draft PR: https://github.com/behaviorengineering/majordomo/pull/83

Counts: critical 0, medium 0, low 0, open 1.

## Now

`pkg/review/reviewrun/run.go:12` imports `internal/ops/sa`. Is this public-to-internal dependency intentional?

## Resolutions

| Outcome | Severity | Location | Result |
|---|---|---|---|
| fixed in Pass A | low | lint tooling | Installed `golangci-lint` with Go 1.27. |
| fixed in Pass A | low | `internal/ops/submodule/git.go` | Propagated interactive output write failures. |
| fixed in Pass A | medium | `internal/ops/cli/root.go` | Returned CLI help and output errors. |
| fixed in Pass A | medium | `internal/ops/sa/sa.go` | Aggregated static-analysis tool failures. |
| fixed in Pass A | medium | `internal/ops/submodule/git.go` | Failed closed when Git status could not be read. |
| fixed in Pass A | low | test fixtures | Checked filesystem setup errors. |
| fixed in Pass A | low | `internal/ops/satools/satools.go` | Reported Docker tag failures. |
| fixed in Pass A | medium | process execution seams | Added context and failsafe policies. |
| fixed in Pass A | medium | `internal/ops/sa/sa.go` | Replaced custom logs with injected `slog`. |
| fixed in Pass A | low | `internal/ops/satools/satools.go` | Templated multi-line reports. |
| fixed in Pass A follow-up | medium | `internal/ops/submodule/commands.go` | Returned worktree cleanup failures. |
| fixed in Pass A follow-up | medium | `internal/ops/cli/root.go` | Returned OTEL configuration failures. |
| fixed in Pass A follow-up | medium | `internal/ops/submodule/git.go` | Preserved Git causes and narrowed detached-HEAD fallback. |
| fixed in Pass A follow-up | low | `internal/ops/sa`, `internal/ops/satools` | Added context to path-discovery errors. |
| waiting on consultant | medium | `pkg/review/reviewrun/run.go:12` | Decide whether the public review runner may own this internal operation dependency. |
