# Staged Review Canvas

## Progress
- Stage 1: done, score 9/10
- Stage 2: done, score 10/10
- Stage 3: done, score 7/10
- Stage 4: in progress, waiting on consultant reply
- Stage 5: pending consultant
- Stage 6: pending consultant
- Stage 7: done, score 8/10
- Stage 8: done, score 6/10
- Pass A: mechanical fixes committed and pushed
- Pass B: consultant questions
- Scores: Stage 1 9/10, Stage 2 10/10, Stage 3 7/10, Stage 7 8/10, Stage 8 6/10
- Counts: critical 0, medium 0, low 0, open 0
- Draft PR/MR: https://github.com/behaviorengineering/majordomo/pull/92

## Now
`internal/ops/cli/root.go:39-850`: Is keeping the root command and roughly 15 command constructors in one CLI registry file intentional, or should wiring split by lifecycle or domain?

Why this matters: A single registry can be a clear composition boundary, but it can also become a high-coupling change hotspot. Splitting only wiring can improve review and test focus without moving business logic into the CLI.

## Resolutions
| Status | Severity | Location | Outcome |
|---|---|---|---|
| fixed in Pass A | medium | Toolchain | Installed a compatible lint tool and passed the target lint gate |
| fixed in Pass A | medium | `internal/ops/satools/satools.go:221` | Returned image-tag failures |
| fixed in Pass A | medium | `internal/ops/sa/sa.go:108` | Aggregated static-analysis failures |
| fixed in Pass A | medium | `internal/ops/cli/root.go:50,113,502` | Returned CLI output errors |
| fixed in Pass A | low | Tests | Checked fixture setup errors and removed dead code |
| fixed in Pass A | medium | `internal/ops/sa/sa.go:33` | Injected structured logging |
| fixed in Pass A | low | CLI and submodule text | Replaced em dashes and completed comments |
| fixed in Pass A | medium | `internal/ops/{satools,sa,submodule}` | Added deadline-checked failsafe process execution |
| fixed in Pass A | medium | `internal/ops/cli/root.go:48` | Added an agent-ready bare invocation guide |
| fixed in Pass A follow-up | high | `cmd/majordomo/main.go` | Moved exit-code selection behind deferred cleanup |
| fixed in Pass A follow-up | high | `internal/ops/submodule` | Preserved Git errors and classified expected no-op cases |
