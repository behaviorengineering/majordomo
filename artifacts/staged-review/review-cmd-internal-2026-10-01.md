# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the branch has no diff from the default branch.
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.

## Stages
- [x] 1. Automated Tools
- [x] 2. Type Safety
- [x] 3. Error Handling
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity
- [x] 8. Generation Gates

## Pass A
Status: complete. All detected mechanical findings were fixed and verified.

## Findings

### Stage 1: Automated Tools
**Score:** 7/10
**Results:** `go vet ./...` passed. `gofmt -l cmd internal` returned no files. `golangci-lint` is not installed, so the lint gate did not run.

#### Medium Tooling: Missing golangci-lint
**Location:** Review environment, Stage 1.
**Severity:** Medium

**Current Code:**
```text
golangci-lint: missing
```

**Recommendation:** Run the repository lint configuration with an available `golangci-lint` binary or an equivalent pinned tool invocation.

**Rationale:** Without the configured lint gate, issues such as unchecked output errors, comment style, and security checks may go undetected.
**Status:** fixed in Pass A

### Stage 2: Type Safety
**Score:** 10/10
**Results:** No bare `interface{}`, unsafe type assertions, or unchecked pointer parameters were found in `cmd/` or `internal/ops/`. The two variadic `...any` helpers are formatting seams where a concrete type is not applicable.

No issues found.

### Stage 3: Error Handling
**Score:** 6/10

#### Medium Persistence: Docker tag failure is discarded
**Location:** `internal/ops/satools/satools.go:221`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
	full := "local/sa-" + tool + ":local-test"
	_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
```

**Recommendation:** Return the tag command error as part of the build result, while preserving its output for diagnostics.

**Rationale:** A successful image build followed by a failed tag currently reports success and leaves the expected image tag unavailable.
**Status:** fixed in Pass A

#### Medium Error Handling: SA tool failures are logged and discarded
**Location:** `internal/ops/sa/sa.go:110`
**Severity:** Medium

**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
	logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Continue running other configured tools, collect each failure, and return an aggregate error after all tools finish.

**Rationale:** The command exits successfully even when one or more requested analysis tools fail.
**Status:** fixed in Pass A

#### Low Output: CLI write errors are ignored
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Low

**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:** Use `RunE` for commands that write output and return the writer error.

**Rationale:** Broken pipes and other output failures are hidden from callers.
**Status:** fixed in Pass A

#### Low Test Setup: Test filesystem setup errors are ignored
**Location:** `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-32`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```

**Recommendation:** Check each setup error with `t.Fatal` or a small test helper.

**Rationale:** A setup failure can produce a misleading test result instead of failing at the cause.
**Status:** fixed in Pass A

### Stage 7: Code Clarity
**Score:** 8/10

#### Low Style: Source contains em dash punctuation
**Location:** `internal/ops/cli/root.go:43`, `internal/ops/sa/sa.go:51`, `internal/ops/submodule/*.go`
**Severity:** Low

**Current Code:**
```go
Long: `Majordomo — repository operations for evolving software.`
```

**Recommendation:** Replace em dash characters with ASCII punctuation such as a colon or semicolon.

**Rationale:** The workspace writing rule prohibits U+2014 in code and user-facing strings, and ASCII punctuation is more portable.
**Status:** fixed in Pass A

#### Low Comments: Comment does not end with a period
**Location:** `internal/ops/submodule/git.go:39`
**Severity:** Low

**Current Code:**
```go
parentRoot    string // empty if none
```

**Recommendation:** End the comment with a period.

**Rationale:** Consistent sentence punctuation keeps comments compatible with the repository's `godot` quality gate.
**Status:** fixed in Pass A

No other clarity findings. Interactive command output in `satools` and `submodule` is intentional operator UI, not service logging.

### Stage 8: Generation Gates
**Score:** 4/10

#### Critical Resilience: Outbound process execution bypasses the shared resilience contract
**Location:** `internal/ops/sa/sa.go:132`, `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Critical

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
cmd := exec.Command(name, args...)
cmd := exec.Command("git", args...)
```

**Recommendation:** Thread a caller context through these seams, fail closed when it is nil or has no deadline, and execute network-backed or shared process hops through a common failsafe-go retry and circuit-breaker policy with classified transient errors.

**Rationale:** Git, Docker, and analysis-tool processes can hang or fail in correlated bursts. Bare execution has no caller budget, retry policy, or circuit breaker.
**Status:** fixed in Pass A

#### Medium Observability: The direct dispatch LLM path lacks process tracing
**Location:** `internal/ops/cli/root.go:180-236`
**Severity:** Medium

**Current Code:**
```go
return dispatch.Dispatch(opts)
```

**Recommendation:** Initialize the process tracer and wrap the dispatch operation in a span before the LLM work starts, using the same configuration path as orchestration.

**Rationale:** Gateway HTTP traces do not expose client-side parsing, evaluation, or post-response hangs.
**Status:** fixed in Pass A

#### Medium CLI Surface: Bare invocation is not agent-ready
**Location:** `internal/ops/cli/root.go:47-52`
**Severity:** Medium

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
	_ = cmd.Help()
	return errSubcommandRequired
},
```

**Recommendation:** Print a structured agent operating guide and return success for a bare invocation, while keeping help and command errors for explicit invalid input.

**Rationale:** Operators and coding agents need safe command discovery without a misleading failure or accidental mutation.
**Status:** fixed in Pass A

Generation checks without findings: no HTTP body, transaction, cancellation, SQL migration, durable timestamp, report-template, or Makefile gate was in the review target. Package layout is consistent with the existing `internal/ops` domain grouping.

## Pass A Verification
- `go test ./...`: passed.
- `go vet ./...`: passed.
- `gofmt -l cmd internal`: clean.
- `GOTOOLCHAIN=go1.27.0 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./cmd/... ./internal/...`: passed with 0 issues.
- Outbound process calls now use `internal/ops/process` with required caller deadlines, retry classification, and shared circuit breakers.

## Open Questions

## Resolutions
| Finding | Outcome |
|---|---|
| Missing lint tool | Resolved by running the latest linter with Go 1.27. |
| Docker tag failure discarded | Fixed by returning tag failures from SA image builds. |
| SA tool failures discarded | Fixed by aggregating tool failures and returning them after all tools run. |
| CLI write errors ignored | Fixed by returning command output errors and preserving submodule output errors. |
| Test setup errors ignored | Fixed by checking all test filesystem setup operations. |
| Em dash punctuation | Fixed by replacing prohibited em dash characters. |
| Comment punctuation | Fixed by ending the comment with a period. |
| Bare outbound process execution | Fixed with the shared deadline, retry, and circuit-breaker process seam. |
| Direct dispatch tracing | Fixed by initializing OTEL and wrapping direct dispatch in a span. |
| Bare CLI invocation | Fixed with a structured agent operating guide and successful bare invocation. |
| Finding | Outcome |
|---|---|
