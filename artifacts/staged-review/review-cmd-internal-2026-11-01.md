# Staged Review Plan: cmd-internal

**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...` (no Go diff against `origin/main`)
**Selection:** Pass A mechanical stages 1, 2, 3, 7, 8; Pass B consultant stages 4, 5, 6
**Hooks:** none discovered
**Pack mount commit:** `7e825ae`
**Pass A fix commit:** `5ac3c38`

## Stages

- [x] 1. Automated tools (6/10)
- [x] 2. Type safety (9/10)
- [x] 3. Error handling (5/10)
- [ ] 4. Architecture (consultant)
- [ ] 5. Robustness (consultant)
- [ ] 6. Testability (consultant)
- [x] 7. Code clarity (9/10)
- [x] 8. Generation gates (6/10)

## Findings

### Medium Tooling: operatorconfig checksum mismatch blocks quality gates
**Location:** `go.mod:8` and `go.sum:240`
**Severity:** Medium
**Current Code:**
```text
github.com/behaviorengineering/operatorconfig v0.1.1
github.com/behaviorengineering/operatorconfig v0.1.1 h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
```
The module download resolved to `h1:nVrLCDsziZyRxFB6sH6I1DHdsQ9dgoO9HPU1+ZDMH8k=`, so `go tool task vet` stopped with a checksum security error. The dependent lint run then stopped because package loading failed.
**Recommendation:** Replace the stale, mutable dependency reference with the current verified `operatorconfig` release or an immutable version, refresh `go.sum`, and rerun the quality gates.
**Rationale:** A checksum mismatch must fail closed. Leaving it unresolved prevents reproducible builds and blocks both vet and lint.
**Status:** fixed in Pass A

### Low Tooling: format check passed
**Location:** `./cmd`, `./internal`, `./pkg`
**Severity:** Low
**Current Code:** `go tool gofumpt -l ./cmd ./internal ./pkg` returned no files.
**Recommendation:** None.
**Rationale:** The target tree is already formatted.
**Status:** non-issue

## Stage 3: Error handling

### Medium Error handling: static-analysis tool failures are logged and discarded
**Location:** `internal/ops/sa/sa.go:109-111`
**Severity:** Medium
**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Continue independent tools if desired, but collect the failures and return `errors.Join` after all tools finish.
**Rationale:** The command currently exits successfully even when a configured analysis tool fails, so callers and automation can accept an incomplete review.
**Status:** fixed in Pass A

## Stage 7: Code clarity

**Score:** 9/10

No additional findings. The target is gofmt-clean, comments in the inspected files satisfy the period check, and no TODO/FIXME markers or unclear generic symbols were found beyond the Stage 3 output/error handling items.

## Stage 8: Generation gates

**Score:** 6/10

### Medium Resilience: outbound command execution has no retry or circuit-breaker policy
**Location:** `internal/ops/sa/sa.go:130-138`, `internal/ops/satools/satools.go:227-237`, `internal/ops/submodule/git.go:219-235`
**Severity:** Medium
**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
...
err := cmd.Run()
```
Each package invokes external scripts, Docker, or Git directly without a shared retry policy that distinguishes transient failures from permanent command errors.
**Recommendation:** Route network-backed command execution through a shared, testable failsafe-go adapter with bounded exponential backoff, jitter, circuit breaking, and context cancellation. Keep injected test runners unchanged.
**Rationale:** Transient forge, Git, or registry failures currently fail the whole operation immediately and repeated callers can create retry storms.
**Status:** fixed in Pass A

### Low Operator surface: listed Taskfile tasks have no summaries
**Location:** `Taskfile.yml:8-39`
**Severity:** Low
**Current Code:** Listed tasks have `desc` values but no `summary` values.
**Recommendation:** Add concise summaries to the local-dev tasks, including package scope and argument behavior where relevant.
**Rationale:** `go tool task --summary` gives operators and agents the detailed, safe invocation contract without duplicating the Cobra catalog.
**Status:** fixed in Pass A

### Low CLI surface: bare root invocation reports an error after help
**Location:** `internal/ops/cli/root.go:52-57`
**Severity:** Low
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```
**Recommendation:** Provide the structured agent operating guide for bare invocation and return success, or document and migrate this legacy one-shot behavior.
**Rationale:** Agents receive a non-zero result for a normal discovery action and do not get the repository operator workflow in the default view.
**Status:** fixed in Pass A

### Medium Error handling: observability configuration errors fall through to defaults
**Location:** `internal/ops/cli/root.go:85-112`
**Severity:** Medium
**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
} else {
    settings = observability.Settings{...}
}
return observability.ResolveConfig(outputDir, settings)
```
**Recommendation:** Return the configuration error to the orchestrate command instead of silently continuing with default settings.
**Rationale:** A malformed or inaccessible central configuration can disable expected tracing while the command proceeds without a clear failure.
**Status:** fixed in Pass A

### Medium Error handling: Docker tag failure is ignored
**Location:** `internal/ops/satools/satools.go:217-223`
**Severity:** Medium
**Current Code:**
```go
if err == nil {
    full := "local/sa-" + tool + ":local-test"
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```
**Recommendation:** Check the tag command error and mark the build failed with its output when tagging fails.
**Rationale:** A successful build without the required tag produces an artifact that later steps cannot consume.
**Status:** fixed in Pass A

### Low Error handling: ignored output and setup errors
**Location:** `internal/ops/cli/operator_cli.go:57-61`, `internal/ops/cli/root.go:54,119,513`, `internal/ops/sa/sa.go:150`, `internal/ops/sa/sa_test.go:12-33`, `internal/ops/satools/satools_test.go:13-43`, `internal/ops/submodule/submodule_test.go:65`
**Severity:** Low
**Current Code:** Several `fmt.Fprintf`/`fmt.Fprintln`, `os.Getwd`, and test filesystem calls use discarded errors (`_, _ =` or `_ =`).
**Recommendation:** Return or assert writer and filesystem errors. Keep only deliberate deferred cleanup discards.
**Rationale:** Output and test setup failures currently become silent success or misleading test state.
**Status:** fixed in Pass A

### Low Error handling: cleanup error is not visible
**Location:** `internal/ops/submodule/commands.go:293-296`
**Severity:** Low
**Current Code:**
```go
defer func() {
    m.printf("Cleaning up worktree...\n")
    _, _ = m.git([]string{"worktree", "remove", "--force", worktreePath}, m.parentRoot, false)
}()
```
**Recommendation:** Keep cleanup best-effort, but report a cleanup failure to the operation output.
**Rationale:** A stale worktree can affect a later run even though the primary operation succeeded.
**Status:** fixed in Pass A

### Low Error handling: unwrapped cleanup failure
**Location:** `internal/ops/submodule/git.go:161-166`
**Severity:** Low
**Current Code:**
```go
_, err := m.git([]string{"clean", "-fd"}, repoRoot, true)
return err
```
**Recommendation:** Wrap the cleanup operation with context before returning it.
**Rationale:** Callers need to distinguish a failed reset from a failed clean when diagnosing repository state.
**Status:** fixed in Pass A

### Low CLI surface: unknown root command suppresses usage
**Location:** `internal/ops/cli/root.go:52`
**Severity:** Low
**Current Code:**
```go
SilenceUsage: true,
```
**Recommendation:** Allow Cobra to print usage for unknown root commands.
**Rationale:** A misspelled command currently returns only an error string, which is less useful to operators and agents.
**Status:** fixed in Pass A

## Stage 2: Type safety

No findings. The target contains only intentional variadic `any` parameters for formatting helpers; no unchecked type assertions, bare `interface{}`, or unguarded assertion patterns were found.

## Open questions

None recorded yet.

## Pass A verification

- `go tool task lint`: passed, 0 issues.
- `go tool task vet`: passed.
- `go tool task test`: passed.
- `go tool task build`: passed.
- `go tool gofumpt -l ./cmd ./internal ./pkg`: passed with no files listed.
- CLI smoke checks: bare root and `version` exit 0; unknown commands exit non-zero with usage.
- Pass A is committed. Pass B consultant stages 4, 5, and 6 are next.

## Resolutions

| Finding | Outcome |
|---|---|
| operatorconfig checksum mismatch | Fixed in Pass A by upgrading to verified `v0.1.3` and tidying module sums. |
| static-analysis tool failures discarded | Fixed in Pass A by returning `errors.Join` of tool failures after all tools run. |
| OTEL config errors fall through | Fixed in Pass A by returning the load error. |
| Docker tag failure ignored | Fixed in Pass A by checking the tag command and reporting its output. |
| Ignored output and setup errors | Fixed in Pass A by checking command output and test setup operations. |
| Worktree cleanup failure hidden | Fixed in Pass A by reporting cleanup failures. |
| Unwrapped cleanup failure | Fixed in Pass A with operation context. |
| Bare root invocation | Fixed in Pass A with a structured agent guide and successful exit. |
| Outbound command resilience | Fixed in Pass A with a shared failsafe-go command adapter, deadline checks, retries, jitter, and circuit breaking. |
| Taskfile summaries missing | Fixed in Pass A with concise summaries for listed tasks. |
| Unknown command usage suppressed | Fixed in Pass A by enabling Cobra usage output on root errors. |
