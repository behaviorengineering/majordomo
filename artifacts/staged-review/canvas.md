# Staged Review Canvas

## Progress

- Pass A: complete, fixes verified and ready to commit.
- Pass B: ready to start after the Pass A commit and push.
- Stage 1: done, score 9/10.
- Stage 2: done, score 10/10.
- Stage 3: done, score 5/10.
- Stage 4: not started.
- Stage 5: not started.
- Stage 6: not started.
- Stage 7: done, score 9/10.
- Stage 8: done, score 4/10.
- Counts: 0 critical, 6 medium fixed, 4 low fixed, 1 environment finding open.
- Draft PR: not opened yet.

## Now

Pass A fixes are verified. Full tests, full vet, formatting, targeted race tests, bare CLI invocation, and version output pass. The remaining Stage 1 lint limitation is an environment issue: the available linter cannot decode the module's Go 1.27 export data.

## Resolutions

| Kind | Severity | Location | Outcome |
|---|---|---|---|
| Auto-fixed in Pass A | Medium | `internal/ops/satools/satools.go` | Docker tag errors now fail the build result. |
| Auto-fixed in Pass A | Medium | `internal/ops/cli/root.go` | Help, output, and OTEL configuration errors are returned. |
| Auto-fixed in Pass A | Medium | `internal/ops/sa/sa.go` | Static-analysis failures are aggregated and returned. |
| Auto-fixed in Pass A | Low | `internal/ops/*` | Test setup errors and forbidden em dash punctuation are fixed. |
| Auto-fixed in Pass A | Medium | `internal/ops/command/command.go` | External processes use deadline-aware failsafe policies. |
| Open environment finding | Low | Stage 1 lint | Linter toolchain compatibility remains unresolved. |
