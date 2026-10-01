# Review Plan: default-go
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because this run has no diff against `main`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Pass A Fix Commit:** `3027ca5` (`fix mechanical staged review findings`)

## Stages
- [x] 1. Automated Tools (8/10)
- [x] 2. Type Safety (10/10)
- [x] 3. Error Handling (5/10)
- [x] 7. Code Clarity (7/10)
- [x] 8. Generation Gates (5/10)
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability

## Findings

### Stage 1: Automated Tools

#### Low Tooling: golangci-lint is unavailable
**Location:** repository toolchain
**Severity:** Low

**Current Code:**
`make vet` and `make lint` are not defined. The fallback `go vet ./...` passed, `gofmt -l ./cmd ./internal` returned no files, and `golangci-lint run ./...` could not run because `golangci-lint` is not installed.

**Recommendation:**
Install or provision `golangci-lint`, then run the repository lint configuration.

**Rationale:** The lint gate remains unverified even though vet and formatting passed.
**Status:** open

### Stage 2: Type Safety

No findings. The target has no unsafe `any` or `interface{}` use, unchecked type assertions, or unguarded pointer dereferences. The variadic `any` parameters are required for formatting APIs.

### Stage 3: Error Handling

#### Medium Error Handling: CLI output errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium

**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:**
Use `RunE` for output commands and return help and writer errors to Cobra.

**Rationale:** Broken pipes and failed output writers currently report success.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failures are discarded
**Location:** `internal/ops/satools/satools.go:221`
**Severity:** Medium

**Current Code:**
```go
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
```

**Recommendation:**
Propagate the tag error and include its output in the build result.

**Rationale:** The command can claim a successful image build even when the requested tag was not created.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis failures are logged and hidden
**Location:** `internal/ops/sa/sa.go:110`
**Severity:** Medium

**Current Code:**
```go
if err := runner(...); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:**
Run all matching tools, then return an error that reports the failed tools.

**Rationale:** A successful process exit currently hides failed quality checks.
**Status:** fixed in Pass A

#### Low Error Handling: Optional git probes suppress command errors
**Location:** `internal/ops/submodule/git.go:96,100,129,146,189`
**Severity:** Low

**Current Code:**
```go
indexEntry, _ := m.git(...)
```

**Recommendation:**
Preserve command errors in the git helper and handle expected probe failures explicitly at each optional probe.

**Rationale:** The current helper also hides unexpected failures, making repository state decisions unreliable.
**Status:** fixed in Pass A

#### Low Error Handling: Test setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12,13,31,32` and `internal/ops/satools/satools_test.go:12,13,30,31,42,43`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```

**Recommendation:**
Fail the test immediately when setup cannot create its fixture.

**Rationale:** A test can fail later with a misleading assertion instead of reporting fixture setup failure.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Low Clarity: Static-analysis logging bypasses structured logging
**Location:** `internal/ops/sa/sa.go:33-35,90-110`
**Severity:** Low

**Current Code:**
```go
func logf(level, format string, args ...any) {
    fmt.Printf("[%s] [%s] %s\n", ...)
}
```

**Recommendation:**
Use an injected structured logger or an explicit operator output writer, and keep tool failure reporting separate from logging.

**Rationale:** Timestamped `fmt.Printf` output cannot be filtered or correlated reliably by callers and mixes service behavior with presentation.
**Status:** fixed in Pass A

#### Low Clarity: User-facing messages use em dash punctuation
**Location:** `internal/ops/cli/root.go:43`, `internal/ops/submodule/*.go`, `internal/ops/sa/sa.go:51`
**Severity:** Low

**Current Code:**
```go
"Majordomo — repository operations for evolving software."
```

**Recommendation:**
Replace em dashes with commas, colons, or parentheses.

**Rationale:** The repository language rules prohibit em dashes in code and user-facing strings.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium Generation Gates: Outbound process execution bypasses resilience policy
**Location:** `internal/ops/sa/sa.go:132`, `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(...)
err := cmd.Run()
```

**Recommendation:**
Route process execution through one context-aware seam with caller deadlines, classified retry policy, and circuit breaking for network-backed git and tool dependencies. Local commands should still honor cancellation.

**Rationale:** A hung or failing external process can block the CLI indefinitely and repeated remote operations can create failure storms.
**Status:** fixed in Pass A

#### Medium Generation Gates: Bare invocation does not provide the agent operating guide
**Location:** `internal/ops/cli/root.go:49-52`
**Severity:** Medium

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:**
Print the structured agent operating guide on bare invocation and exit successfully, while retaining normal help for explicit `help`.

**Rationale:** Automation receives only a generic usage error and cannot discover safe inspect, dry-run, and mutating command paths from the default view.
**Status:** fixed in Pass A

### Stage 4: Architecture

### Stage 5: Robustness

### Stage 6: Testability

## Open Questions

- Stage 1 tooling: `golangci-lint` is unavailable, so the lint gate remains unverified.

## Resolutions

- Tooling finding: still open because `golangci-lint` is not installed; vet and formatting passed.
- CLI output errors: fixed in Pass A by returning help and writer errors.
- Docker tag errors: fixed in Pass A by failing the build result and preserving tag output.
- Static-analysis failures: fixed in Pass A by aggregating failed tools and returning an error.
- Git probe errors: fixed in Pass A by preserving process errors and handling expected probes explicitly.
- Test setup errors: fixed in Pass A by failing fixture setup immediately.
- Static-analysis output: fixed in Pass A by injecting the output writer and returning write errors.
- Em dash punctuation: fixed in Pass A across the reviewed CLI and submodule messages.
- Outbound execution: fixed in Pass A with the bounded `execx` seam, context propagation, retry classification, and a shared circuit breaker.
- Bare invocation: fixed in Pass A with the templated agent operating guide and regression test.

