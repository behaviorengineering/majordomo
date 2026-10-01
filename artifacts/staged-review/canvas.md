# Staged Review Canvas

## Progress

| Stage | Status | Score |
|---|---|---|
| 1. Automated Tools | done | 8/10 |
| 2. Type Safety | done | 10/10 |
| 3. Error Handling | done | 5/10 |
| 4. Architecture | pending | |
| 5. Robustness | pending | |
| 6. Testability | pending | |
| 7. Code Clarity | done | 7/10 |
| 8. Generation Gates | done | 5/10 |

Pass A: mechanical, stages 1, 2, 3, 7, 8.  
Pass B: consultant, stages 4, 5, 6.  
Counts: critical 0, medium 0, low 1, open 1.
Pass A fix commit: `3027ca5`.
Draft PR: [#84](https://github.com/behaviorengineering/majordomo/pull/84).

## Now

Pass A committed and pushed, inspecting consultant stages 4, 5, and 6

## Resolutions

| Type | Severity | Location | Outcome |
|---|---|---|---|
| open | low | repository toolchain | golangci-lint unavailable; vet and format checks passed |
| auto-fixed in Pass A | medium | internal/ops/cli/root.go | CLI output and help errors now return to Cobra |
| auto-fixed in Pass A | medium | internal/ops/satools/satools.go | Docker tag failures now fail the build result |
| auto-fixed in Pass A | medium | internal/ops/sa/sa.go | Static-analysis failures now return after all tools run |
| auto-fixed in Pass A | low | internal/ops/submodule/git.go | Optional git probes now handle errors explicitly |
| auto-fixed in Pass A | low | internal/ops/*/*_test.go | Test fixture setup now fails immediately |
| auto-fixed in Pass A | low | internal/ops/sa/sa.go | SA output uses injected writer and handles write errors |
| auto-fixed in Pass A | low | internal/ops/cli/root.go and internal/ops/submodule/*.go | User-facing strings no longer contain em dashes |
| auto-fixed in Pass A | medium | internal/ops/sa/sa.go, internal/ops/satools/satools.go, internal/ops/submodule/git.go | Process execution uses bounded context and shared resilience seam |
| auto-fixed in Pass A | medium | internal/ops/cli/root.go | Bare invocation now prints the agent operating guide |
