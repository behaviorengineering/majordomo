# Staged Review Canvas

## Progress

- Pass A: complete, committed as `b15ff34`, with a follow-up fix pending commit.
- Pass B: in progress, Stage 4 architecture question is waiting for a reply.
- Stage 1: done, score 9/10.
- Stage 2: done, score 10/10.
- Stage 3: done, score 5/10.
- Stage 4: in progress.
- Stage 5: not started.
- Stage 6: not started.
- Stage 7: done, score 9/10.
- Stage 8: done, score 4/10.
- Counts: 0 critical, 6 medium fixed, 4 low fixed, 1 environment finding open.
- Draft PR: [#82](https://github.com/behaviorengineering/majordomo/pull/82).

## Now

Question at `internal/ops/cli/root.go:39-888`: Is keeping OTEL configuration, agent-guide rendering, and shared process context setup in the 888-line Cobra root intentional, or should the root remain command wiring only? Why this matters: a thinner root limits coupling and makes command registration easier to test, while an intentional facade can be a valid boundary.

## Resolutions

| Kind | Severity | Location | Outcome |
|---|---|---|---|
| Auto-fixed in Pass A | Medium | `internal/ops/satools/satools.go` | Docker tag errors now fail the build result. |
| Auto-fixed in Pass A | Medium | `internal/ops/cli/root.go` | Help, output, and OTEL configuration errors are returned. |
| Auto-fixed in Pass A | Medium | `internal/ops/sa/sa.go` | Static-analysis failures are aggregated and returned. |
| Auto-fixed in Pass A | Low | `internal/ops/*` | Test setup errors and forbidden em dash punctuation are fixed. |
| Auto-fixed in Pass A | Medium | `internal/ops/command/command.go` | External processes use deadline-aware failsafe policies. |
| Open environment finding | Low | Stage 1 lint | Linter toolchain compatibility remains unresolved. |
| Auto-fixed in Pass A follow-up | Medium | `internal/ops/submodule/git.go` | Git discovery now preserves the wrapped cause. |
| Auto-fixed in Pass A follow-up | Low | `internal/ops/submodule/git.go` | Inline comment punctuation is complete. |
| Non-issue | Medium | `internal/ops/sa`, `internal/ops/satools` | Direct output is intentional interactive CLI UI, not service logging. |
