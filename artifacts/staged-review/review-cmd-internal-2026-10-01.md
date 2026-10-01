# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because this cron run has no Go diff against `origin/main`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Stage Mapping:** User stage 7 = skill stage 4 (Code Clarity), user stage 8 = skill stage 5 (Generation Gates), user stages 4, 5, 6 = skill stages A, B, C.

## Stages
- [x] 1. Automated Tools, score 8/10
- [x] 2. Type Safety, score 10/10
- [x] 3. Error Handling, score 4/10
- [x] 7. Code Clarity, score 6/10
- [x] 8. Generation Gates, score 3/10
- [ ] 4. Architecture, score pending
- [ ] 5. Robustness, score pending
- [ ] 6. Testability, score pending

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: `golangci-lint` is unavailable
**Location:** Review environment, Stage 1 lint slot.
**Severity:** Medium
**Current Code:**
```text
golangci-lint: missing
```
**Recommendation:** Install or provide the repository's configured `golangci-lint` binary, then run `golangci-lint run ./cmd/... ./internal/...`.
**Rationale:** Vet and formatting do not cover the configured lint and security checks.
**Status:** open

**Verified:** `go vet ./cmd/... ./internal/...` passed. `gofmt -l cmd internal` produced no output.

### Stage 2: Type Safety

No findings. The target uses `any` only for intentionally variadic formatting arguments, has no unchecked type assertions, and has no public pointer inputs or required pointer dependencies needing nil guards.

### Stage 3: Error Handling

#### Medium Error Handling: Docker tag failures are discarded
**Location:** `internal/ops/satools/satools.go:221`
**Severity:** Medium
**Current Code:**
```go
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
```
**Recommendation:** Check the tag command error and report the build as failed when tagging fails.
**Rationale:** The command can report a successful image build even though the requested local tag was not created.
**Status:** fixed in Pass A

#### Medium Error Handling: CLI help and output write errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium
**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```
**Recommendation:** Return output errors from `RunE` commands and wrap help failures.
**Rationale:** Broken pipes and failed output writers currently produce false success or hide the actual failure.
**Status:** fixed in Pass A

#### Medium Error Handling: Probe errors are ignored in submodule safety checks
**Location:** `internal/ops/submodule/git.go:96,100,129,146,189`
**Severity:** Medium
**Current Code:**
```go
indexEntry, _ := m.git(...)
gitDirRaw, _ := m.git(...)
entry, _ := m.git(...)
out, _ := m.git(...)
```
**Recommendation:** handle each probe error explicitly and fail closed for dirty or unverified repository state.
**Rationale:** A failed Git probe can be mistaken for a clean or untracked repository and lead to unsafe follow-up actions.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis command failures are logged and suppressed
**Location:** `internal/ops/sa/sa.go:110`
**Severity:** Medium
**Current Code:**
```go
if err := runner(...); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Continue processing other tools but collect failures and return them after the loop.
**Rationale:** The caller currently receives success even when one or more configured tools fail.
**Status:** fixed in Pass A

#### Low Error Handling: `os.Getwd` failure is discarded
**Location:** `internal/ops/sa/sa.go:150`
**Severity:** Low
**Current Code:**
```go
wd, _ := os.Getwd()
```
**Recommendation:** Return a wrapped error when the working directory cannot be read.
**Rationale:** Directory discovery should not silently continue from an unknown path.
**Status:** fixed in Pass A

#### Medium Error Handling: Configuration load errors are logged and replaced with defaults
**Location:** `internal/ops/cli/root.go:90-104`
**Severity:** Medium
**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
} else {
    settings = ...
}
return observability.ResolveConfig(outputDir, settings)
```
**Recommendation:** Return the configuration error to the command instead of silently falling back.
**Rationale:** Observability can be disabled or misconfigured while the command still reports success.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Medium Logging: Static-analysis logging bypasses structured logging
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium
**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```
**Recommendation:** Inject a structured logger or output seam and use it for non-interactive command messages.
**Rationale:** Direct formatted stdout output loses fields, log levels, and consistent command logging behavior.
**Status:** fixed in Pass A

#### Low Naming: Generic `logf` hides the operation being logged
**Location:** `internal/ops/sa/sa.go:33`
**Severity:** Low
**Current Code:**
```go
func logf(level, format string, args ...any)
```
**Recommendation:** Use an operation-specific logger or an injected logger field.
**Rationale:** Generic package-level logging makes the dependency and output contract unclear.
**Status:** fixed in Pass A

#### Low Documentation: Inline comment lacks terminal punctuation
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low
**Current Code:**
```go
parentRoot string // empty if none
```
**Recommendation:** End the comment with a period.
**Rationale:** Consistent comment punctuation keeps documentation lintable and readable.
**Status:** fixed in Pass A

#### Low Text: User-facing strings use em dashes
**Location:** `internal/ops/cli/root.go`, `internal/ops/sa/sa.go`, and `internal/ops/submodule/*.go`
**Severity:** Low
**Current Code:**
```go
"Majordomo — repository operations for evolving software."
```
**Recommendation:** Replace em dashes with commas, colons, semicolons, or plain hyphens.
**Rationale:** The repository writing rule forbids U+2014 in code and user-facing text.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Critical Outbound Resilience: Process execution bypasses context and failsafe policies
**Location:** `internal/ops/satools/satools.go:230`, `internal/ops/sa/sa.go:132`, `internal/ops/submodule/git.go:222`
**Severity:** Critical
**Current Code:**
```go
cmd := exec.Command(name, args...)
err := cmd.Run()
```
**Recommendation:** Route process execution through one context-aware seam with a required caller deadline, classified failsafe-go retries, and circuit breaking for shared remote dependencies.
**Rationale:** Git, Docker, and analysis commands can hang or create retry storms without cancellation and bounded resilience.
**Status:** fixed in Pass A

#### Medium Observability: Dispatch command has no process-level tracing setup
**Location:** `internal/ops/cli/root.go:190-226`
**Severity:** Medium
**Current Code:**
```go
return dispatch.Dispatch(opts)
```
**Recommendation:** Initialize the configured tracer and create a command span around both dispatch modes.
**Rationale:** LLM parse, evaluation, and post-response failures are invisible when only gateway HTTP traces exist.
**Status:** fixed in Pass A

#### Medium Error Boundary: `main` checks a leaf package sentinel
**Location:** `cmd/majordomo/main.go:27`
**Severity:** Medium
**Current Code:**
```go
if errors.Is(err, staging.ErrNothingToReview) {
```
**Recommendation:** Expose an `IsNothingToReview` helper from the command/service package and keep the entrypoint dependent on that package surface.
**Rationale:** Entry code should not import a deeper leaf package solely to map its sentinel to a process exit code.
**Status:** fixed in Pass A

#### Medium CLI Surface: Bare invocation is not an agent-ready guide
**Location:** `internal/ops/cli/root.go:45-52`
**Severity:** Medium
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```
**Recommendation:** Print a structured operator guide on bare invocation and return success, while keeping unknown commands non-zero.
**Rationale:** Automated operators need command groups, lifecycle guidance, and safe mutation rules without parsing a one-line usage failure.
**Status:** fixed in Pass A

#### Low Tests: Test setup errors are discarded
**Location:** `internal/ops/satools/satools_test.go:12-14,30-32,42-43`, `internal/ops/sa/sa_test.go:12-13,31-32`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```
**Recommendation:** Use `t.Helper()` setup helpers that fail the test immediately when fixture creation fails.
**Rationale:** Tests should report fixture failures directly instead of continuing with incomplete state.
**Status:** fixed in Pass A

#### Critical Resource Cleanup: `os.Exit` bypasses deferred shutdown
**Location:** `cmd/majordomo/main.go:16-34`
**Severity:** Critical
**Current Code:**
```go
if err := cli.NewRoot().Execute(); err != nil {
    os.Exit(1)
}
```
**Recommendation:** Move command execution into a function that returns the exit code, then call `os.Exit` only from `main`.
**Rationale:** `os.Exit` skips deferred tracer and gateway cleanup, leaving spans and resources unflushed on command errors.
**Status:** fixed in Pass A

#### Medium CLI Surface: Unknown commands did not print the command catalog
**Location:** `cmd/majordomo/main.go:29-34`
**Severity:** Medium
**Current Code:**
```go
if err := cli.NewRoot().Execute(); err != nil {
    fmt.Fprintln(os.Stderr, err)
}
```
**Recommendation:** Retain the root command instance and print its usage after an execution error.
**Rationale:** Operators and agents need the valid command list when a root command is mistyped.
**Status:** fixed in Pass A

## Open Questions

### Pass A verification

- `go test ./...`: passed.
- `go vet ./cmd/... ./internal/...`: passed.
- `gofmt -l cmd internal pkg/review/reviewrun`: clean.
- `git diff --check`: clean.
- Bare CLI invocation: passed, prints the agent operating guide and exits 0.
- `go run ./cmd/majordomo version`: passed without configuration, prints `dev`.
- Unknown root command: passed, prints an error and the command catalog.
- `golangci-lint run ./cmd/... ./internal/...`: not run because `golangci-lint` is unavailable.

## Resolutions

| Finding | Outcome |
|---|---|
| Stage 1 lint tooling | Still open, `golangci-lint` is unavailable in the environment |
| Discarded Docker tag errors | Fixed in Pass A with explicit error propagation |
| Discarded CLI help and output errors | Fixed in Pass A with returned writer errors |
| Ignored submodule probe errors | Fixed in Pass A with explicit fail-closed handling |
| Suppressed static-analysis failures | Fixed in Pass A with aggregated returned errors |
| Ignored working-directory error | Fixed in Pass A |
| Silent OTEL configuration fallback | Fixed in Pass A by returning the load error |
| Ad hoc static-analysis logging | Fixed in Pass A with `slog` |
| Generic static-analysis logger name | Fixed in Pass A by removing the package logger function |
| Missing comment punctuation | Fixed in Pass A |
| Em dash text | Fixed in Pass A |
| Unbounded process execution | Fixed in Pass A with the context-aware failsafe command seam |
| Missing dispatch tracing | Fixed in Pass A with OTEL initialization and spans |
| Leaf sentinel check in `main` | Fixed in Pass A with `cli.IsNothingToReview` |
| Bare CLI operating guide | Fixed in Pass A |
| Test fixture error handling | Fixed in Pass A |
| Deferred shutdown skipped by `os.Exit` | Fixed in Pass A with an exit-code runner |
| Unknown command catalog | Fixed in Pass A by printing root usage |

## Review Notes

- Pack bootstrap was already recorded in `.gitmodules` and the submodule pointer. The working tree was initialized and host links were refreshed; no separate mount commit was needed because Git reported no mount diff.
- Pass A is mechanical and includes all Low findings. Consultant questions are not auto-fixed.
