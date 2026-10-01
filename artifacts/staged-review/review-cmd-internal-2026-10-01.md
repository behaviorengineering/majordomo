# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` main packages, because `main...HEAD` has no diff
**Selected Stages:** Pass A: 1, 2, 3, 7, 8; Pass B: 4, 5, 6

## Stages
- [x] 1. Automated Tools, score 8/10
- [x] 2. Type Safety, score 10/10
- [x] 3. Error Handling, score 7/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity, score 9/10
- [x] 8. Generation Gates, score 6/10

## Findings

### Stage 1: Automated Tools

#### Low Tooling: golangci-lint unavailable
**Location:** Repository toolchain
**Severity:** Low

**Current Code:**
```text
golangci-lint: not installed
```

**Recommendation:** Install or provide the repository's configured `golangci-lint` binary before relying on the full lint gate.

**Rationale:** `go vet` and `gofmt -l cmd internal` passed, but the configured lint rules were not independently checked.
**Status:** open

**Tool results:** `go vet ./cmd/... ./internal/...` passed. `gofmt -l cmd internal` returned no files.

### Stage 2: Type Safety

No findings. The target has no unchecked type assertions, bare `interface{}`, unsafe pointer dereferences, or unguarded public pointer inputs.

### Stage 3: Error Handling

#### Medium Error Handling: Docker tag failure is discarded
**Location:** `internal/ops/satools/satools.go:221`
**Severity:** Medium

**Current Code:**
```go
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
```

**Recommendation:** Treat a failed tag operation as a failed tool build and include its output in the result.

**Rationale:** The command reports a successful build even when the final image tag was not created.
**Status:** fixed in Pass A

#### Low Error Handling: Working-directory lookup is discarded
**Location:** `internal/ops/sa/sa.go:150`
**Severity:** Low

**Current Code:**
```go
wd, _ := os.Getwd()
```

**Recommendation:** Handle the lookup error explicitly or omit that candidate path.

**Rationale:** A failed working-directory lookup is currently hidden, which makes script discovery less predictable.
**Status:** fixed in Pass A

#### Low Error Handling: CLI output errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Low

**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:** Return output errors from `RunE` handlers and handle help-rendering failure.

**Rationale:** Broken pipes and failed writers currently report success or replace the more useful error.
**Status:** fixed in Pass A

#### Low Error Handling: Test setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-32`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```

**Recommendation:** Check each setup error with `t.Fatal` or `t.Helper` test utilities.

**Rationale:** A broken fixture can produce misleading test results.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis failures are logged and suppressed
**Location:** `internal/ops/sa/sa.go:108-110`
**Severity:** Medium

**Current Code:**
```go
if err := runner(...); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Run all configured tools, aggregate their failures, and return the aggregate error after the loop.

**Rationale:** The command can exit successfully even when one or more configured analysis tools fail.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Low Clarity: Inline field comment lacks terminal punctuation
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low

**Current Code:**
```go
parentRoot string // empty if none
```

**Recommendation:** End the comment with a period.

**Rationale:** Consistent comment punctuation keeps generated documentation and lint checks predictable.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium CLI Surface: Bare invoke lacks the agent guide and succeeds only through an error path
**Location:** `internal/ops/cli/root.go:48-52`
**Severity:** Medium

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:** Make bare invocation print the structured agent operating guide and exit successfully, while keeping explicit help and version commands available.

**Rationale:** Operators and coding agents receive only a generic usage surface and a failure exit code when they invoke the binary without arguments.
**Status:** fixed in Pass A

#### Low CLI Surface: Unknown root commands omit usage
**Location:** `cmd/majordomo/main.go:22-25`
**Severity:** Low

**Current Code:**
```text
unknown command "not-a-command" for "majordomo"
```

**Recommendation:** Allow Cobra to print the root usage catalog for unknown-command errors.

**Rationale:** A typo currently gives no immediate command list or correction path.
**Status:** fixed in Pass A

#### Medium Resilience: Outbound process execution lacks caller context and failsafe policies
**Location:** `internal/ops/process/process.go:48-83`, used by `internal/ops/sa`, `internal/ops/satools`, and `internal/ops/submodule`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(name, args...)
err := cmd.Run()
```

**Recommendation:** Route process execution through a shared context-aware seam with deadline checks, classified retry policy, and circuit breaking for network-backed commands. Keep destructive local git mutations non-retryable.

**Rationale:** A hung fetch, push, Docker build, or static-analysis process can outlive its caller, and repeated network failures can cascade without a bounded resilience policy.
**Status:** fixed in Pass A

#### Low Generation Gates: Raw internal operation errors have no stable operation context
**Location:** `internal/ops/sa`, `internal/ops/satools`, and `internal/ops/submodule`
**Severity:** Low

**Current Code:**
```go
return err
```

**Recommendation:** Wrap errors at package boundaries with stable operation names while preserving the cause chain.

**Rationale:** CLI failures currently lose the operation context needed for reliable classification and troubleshooting.
**Status:** fixed in Pass A

## Open Questions

## Resolutions

| Finding | Classification | Severity | Location | Outcome |
|---|---|---|---|---|
| golangci-lint unavailable | still open | Low | Toolchain | No binary is installed; `go vet` and `gofmt` passed. |
| Docker tag failure | auto-fixed in Pass A | Medium | `internal/ops/satools/satools.go` | Tag failures now fail the build result and retain command output. |
| Working-directory lookup | auto-fixed in Pass A | Low | `internal/ops/sa/sa.go` | `os.Getwd` errors now return with operation context. |
| CLI output errors | auto-fixed in Pass A | Low | `internal/ops/cli/root.go` | Version and cursor output errors now return. |
| Test setup errors | auto-fixed in Pass A | Low | SA and SA tools tests | Fixture setup failures now fail tests explicitly. |
| Static-analysis failure suppression | auto-fixed in Pass A | Medium | `internal/ops/sa/sa.go` | Tool failures are aggregated and returned after all tools run. |
| Comment punctuation | auto-fixed in Pass A | Low | `internal/ops/submodule/git.go` | Inline comment now ends with a period. |
| Bare CLI agent guide | auto-fixed in Pass A | Medium | `internal/ops/cli/root.go` | Bare invoke prints the agent guide and exits successfully. |
| Unknown command usage | auto-fixed in Pass A | Low | `cmd/majordomo/main.go` | Errors now include the root usage catalog. |
| Process resilience | auto-fixed in Pass A | Medium | `internal/ops/process/process.go` | Shared deadline checks, retry classification, backoff, jitter, and circuit breaking now protect external processes. |
| Operation context | auto-fixed in Pass A | Low | Internal ops boundaries | New and touched boundaries wrap causes with stable operation names. |
