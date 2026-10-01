# Review Plan: cmd-internal

**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the branch has no diff against the default branch.
**Selected Stages:** Pass A mechanical: 1, 2, 3, 7, 8. Pass B consultant: 4, 5, 6.

## Stages

- [x] 1. Automated Tools — Score: 8/10
- [x] 2. Type Safety — Score: 10/10
- [x] 3. Error Handling — Score: 5/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity — Score: 7/10
- [x] 8. Generation Gates — Score: 4/10

## Pass A

Mechanical findings are detected first, then fixed together after stages 1, 2, 3, 7, and 8.

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: `golangci-lint` is unavailable
**Location:** Review environment, Stage 1 lint slot.
**Severity:** Medium

**Current Code:**
`golangci-lint run` could not execute because the `golangci-lint` binary is not installed. No `Makefile` is present to provide an alternate lint target.

**Recommendation:**
Install or expose the repository's configured `golangci-lint` version, then rerun the lint gate.

**Rationale:** The repository has no lint result for the target, so lint-only defects remain unverified.
**Status:** open

#### Non-finding Tooling: vet and format
`go vet ./...` passed. `gofmt -l cmd internal` reported no files.

### Stage 2: Type Safety

No findings. The target contains no bare `interface{}` values or unchecked type
assertions. The two variadic `any` parameters are used only as formatting
arguments for `fmt`-style output and do not erase a known data type.

### Stage 3: Error Handling

#### Medium Ignored output errors in CLI commands
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium

**Current Code:**
`cmd.Help`, `fmt.Fprintln`, and `fmt.Fprintf` errors are discarded with `_ =`.

**Recommendation:**
Return output errors from `RunE` functions and handle help-rendering failure explicitly.

**Rationale:** Broken pipes and failed output writers currently report success or hide the actual failure.
**Status:** fixed in Pass A

#### Medium Git command failures are suppressed
**Location:** `internal/ops/submodule/git.go:96-146`, `internal/ops/submodule/commands.go:143,244,302`
**Severity:** Medium

**Current Code:**
Probe calls discard errors, and `git` returns `nil` whenever `check` is false. That includes commit operations, so a failed commit can be treated as "nothing to commit."

**Recommendation:**
Make error handling explicit at each probe and return commit failures. Keep only deliberate deferred cleanup suppression.

**Rationale:** The manager can report a successful operation while the repository remains unchanged or inconsistent.
**Status:** fixed in Pass A

#### Medium Static-analysis failures are logged and ignored
**Location:** `internal/ops/sa/sa.go:109-112`
**Severity:** Medium

**Current Code:**
Tool runner failures are logged as warnings and `Run` returns `nil`.

**Recommendation:**
run all matching tools, collect failures, and return an aggregate error after the loop.

**Rationale:** CI can pass even when every configured static-analysis tool fails.
**Status:** fixed in Pass A

#### Low Filesystem and Docker tag errors are discarded
**Location:** `internal/ops/sa/sa.go:150`, `internal/ops/satools/satools.go:221`
**Severity:** Low

**Current Code:**
`os.Getwd` and the post-build Docker tag command use discarded errors.

**Recommendation:**
Return or surface both errors explicitly.

**Rationale:** Path discovery and image tagging can fail without changing the reported result.
**Status:** fixed in Pass A

#### Low Test setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:10-34`, `internal/ops/satools/satools_test.go:12-43`, `internal/ops/submodule/submodule_test.go:62`
**Severity:** Low

**Current Code:**
Test setup uses `_ =` for filesystem calls and a dummy `filepath.Separator` reference.

**Recommendation:**
Fail tests on setup errors and remove the dummy import/reference.

**Rationale:** Tests can continue with incomplete fixtures and hide setup regressions.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Low Non-period comments and compressed option documentation
**Location:** `internal/ops/submodule/git.go:38`, `internal/ops/submodule/menu.go:10-18`, `internal/ops/sa/sa.go:28-30`, `internal/ops/satools/satools.go:20-22`
**Severity:** Low

**Current Code:**
Several inline comments omit terminal punctuation or use compressed arrow notation in exported option documentation.

**Recommendation:**
Use complete sentence comments ending with periods and keep option documentation consistent.

**Rationale:** Consistent comments improve generated documentation and satisfy the repository's `godot` convention.
**Status:** fixed in Pass A

#### Medium Service logging bypasses structured output
**Location:** `internal/ops/sa/sa.go:33-36`
**Severity:** Medium

**Current Code:**
`logf` writes timestamped messages with `fmt.Printf` and calls `time.Now` at each log site.

**Recommendation:**
Use an injected logger or an explicit output dependency for this command, and keep timestamps in the logging implementation.

**Rationale:** Direct formatted output is difficult to capture, filter, and correlate in automation.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Critical Outbound process execution has no context or resilience policy
**Location:** `internal/ops/sa/sa.go:130`, `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Critical

**Current Code:**
The target invokes `exec.Command` directly for shell tools and Git. These calls do not receive a caller context, require a deadline, or use the repository's required retry and circuit-breaker policy for shared outbound dependencies.

**Recommendation:**
Introduce an injected, context-aware process runner. Apply the project's outbound resilience policy to network-backed Git and tool operations, classify retryable failures, and fail closed when no caller deadline exists.

**Rationale:** Hung or failing subprocesses can stall automation and cause repeated network failures to cascade.
**Status:** fixed in Pass A

#### Medium LLM and process command entrypoints lack a consistent observability seam
**Location:** `internal/ops/sa/sa.go:33-36`, `cmd/majordomo/main.go:17-27`
**Severity:** Medium

**Current Code:**
The SA command uses ad hoc formatted logs, while process shutdown ignores flush and shutdown errors.

**Recommendation:**
Use the shared observability seam for command logs and handle shutdown failures in a way that preserves the original command result.

**Rationale:** Automation failures and telemetry export failures are currently hard to distinguish and may be silently lost.
**Status:** fixed in Pass A

#### Low Durable and test-sensitive time is not injectable in SA logging
**Location:** `internal/ops/sa/sa.go:34`
**Severity:** Low

**Current Code:**
`logf` calls `time.Now()` directly for each message.

**Recommendation:**
Inject a clock or move timestamping into the shared logger. Do not use a leaf wall clock where tests need deterministic output.

**Rationale:** Deterministic logs and test output require control over time.
**Status:** fixed in Pass A

## Open Questions

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| Tooling: `golangci-lint` unavailable | Medium | Review environment | Still open because the binary is not installed. |
| Ignored CLI output errors | Medium | `internal/ops/cli/root.go` | Fixed by returning help and writer errors. |
| Suppressed Git failures | Medium | `internal/ops/submodule` | Fixed with explicit probe errors and context-aware Git execution. |
| Ignored static-analysis failures | Medium | `internal/ops/sa/sa.go` | Fixed by returning an `errors.Join` aggregate after all tools run. |
| Discarded filesystem and tag errors | Low | `internal/ops/sa`, `internal/ops/satools` | Fixed with explicit error handling. |
| Discarded test setup errors | Low | `internal/ops/*/*_test.go` | Fixed by failing tests on fixture setup errors. |
| Comment consistency | Low | `internal/ops/*` | Fixed comment punctuation and removed forbidden em-dash characters. |
| Ad hoc SA logging | Medium | `internal/ops/sa/sa.go` | Fixed with an injectable log function and deterministic output. |
| Unbounded outbound process execution | Critical | `internal/ops/{sa,satools,submodule}` | Fixed with the tested `internal/ops/command` runner, deadlines, retries, and circuit breakers. |
| Shutdown and process observability | Medium | `cmd/majordomo/main.go` | Fixed with bounded shutdown context and explicit telemetry error reporting. |
| Leaf wall-clock logging | Low | `internal/ops/sa/sa.go` | Fixed by removing leaf timestamp creation from the command logger. |

## Progress

- Pass A: in progress.
- Pass B: waiting for Pass A commit and push.
- Scores: Stage 1 8/10, Stage 2 10/10, Stage 3 5/10, Stage 7 7/10, Stage 8 4/10.
- Counts: critical 0 fixed, medium 1 open and 4 fixed, low 0 open and 4 fixed.
- Draft PR/MR: pending.
