# Staged Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` (no diff from `origin/main`)
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Branch:** `cursor/staged-code-review-process-f7bc`

## Stages
- [x] 1. Automated Tools
- [x] 2. Type Safety
- [x] 3. Error Handling
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity
- [x] 8. Generation Gates

## Progress
- Pass A: fixes applied, verification complete.
- Pass B: not started.
- Scores: pending.
- Counts: critical 0, medium 1, low 0, open 1.
- Draft PR/MR: pending.
- Pass A fix commit: `d3031f7`.

## Findings

### Stage 1: Automated Tools
**Score:** 9/10

- `go vet ./...`: passed.
- `gofmt -l cmd internal`: passed with no output.
- No root `Makefile` exists, so the Go toolchain fallback was used.

#### Medium Tooling: golangci-lint is unavailable
**Location:** environment
**Severity:** Medium
**Status:** open

**Current Code:**
```text
golangci-lint: missing
```

**Recommendation:** Install or expose the repository's configured `golangci-lint` binary before relying on the lint gate.

**Rationale:** The lint portion of Stage 1 could not run, so lint findings remain unverified.

### Stage 2: Type Safety
**Score:** 10/10

No findings. The review target has no unchecked type assertions, no bare `interface{}`, and no identifiable nil dereference or missing constructor dependency guard. The `any` uses are variadic formatting arguments where the concrete types are intentionally heterogeneous.

### Stage 3: Error Handling
**Score:** 6/10

#### Medium Error Handling: CLI output errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium
**Status:** fixed in Pass A

**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:** Use `RunE` for commands that write output and return the writer error. Return a wrapped help error from the root `RunE`.

**Rationale:** Broken pipes and closed output streams currently report success or hide the actual failure.

#### Medium Error Handling: Static-analysis failures are logged and discarded
**Location:** `internal/ops/sa/sa.go:108-111`
**Severity:** Medium
**Status:** fixed in Pass A

**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Continue running the remaining tools, collect failed tool names and causes, then return an aggregate error after the loop.

**Rationale:** CI can receive a successful `sa` exit status even when one or more configured checks fail.

#### Medium Error Handling: Docker tag failures are discarded
**Location:** `internal/ops/satools/satools.go:218-222`
**Severity:** Medium
**Status:** fixed in Pass A

**Current Code:**
```go
if err == nil {
    full := "local/sa-" + tool + ":local-test"
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
```

**Recommendation:** Treat a failed tag operation as a failed build result and include its error in the reported output.

**Rationale:** The command can report `PASS` after the image was built but the tag needed by later consumers was not created.

#### Low Error Handling: Git probe errors are hidden
**Location:** `internal/ops/submodule/git.go:96,100,129,146,189`
**Severity:** Low
**Status:** fixed in Pass A

**Current Code:**
```go
indexEntry, _ := m.git([]string{"ls-files", "--stage", rel}, parent, false)
gitDirRaw, _ := m.git([]string{"rev-parse", "--git-dir"}, parent, false)
entry, _ := m.git([]string{"ls-files", "--stage", submoduleName}, m.parentRoot, false)
out, _ := m.git([]string{"status", "--porcelain"}, repoRoot, false)
out, _ := m.git([]string{"rev-parse", "--verify", "origin/" + branch}, m.submoduleRoot, false)
```

**Recommendation:** Capture and handle each error explicitly, preserving the intended optional-probe behavior while failing closed when repository state cannot be determined.

**Rationale:** A failed Git probe is indistinguishable from a valid empty result, which can skip safety checks or misclassify repository state.

#### Low Error Handling: Working-directory lookup error is hidden
**Location:** `internal/ops/sa/sa.go:150`
**Severity:** Low
**Status:** fixed in Pass A

**Current Code:**
```go
wd, _ := os.Getwd()
```

**Recommendation:** Return a wrapped error if the current directory cannot be read.

**Rationale:** Searching from an empty directory silently reduces script discovery and hides an environment failure.

#### Low Error Handling: Test setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:11-33`, `internal/ops/satools/satools_test.go:11-32`
**Severity:** Low
**Status:** fixed in Pass A

**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```

**Recommendation:** Fail the test immediately when fixture setup fails.

**Rationale:** A broken fixture can produce misleading product-test failures or false passes.

### Stage 7: Code Clarity
**Score:** 8/10

#### Medium Clarity: Non-interactive SA logging bypasses structured logging
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium
**Status:** fixed in Pass A

**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:** Inject a structured logger into `Options` and emit level-specific fields through that logger. Keep interactive output in the interactive SA-tool command only.

**Rationale:** Plain stdout logging loses machine-readable fields and makes CI log filtering less reliable.

#### Low Clarity: Inline state comment is incomplete
**Location:** `internal/ops/submodule/git.go:39`
**Severity:** Low
**Status:** fixed in Pass A

**Current Code:**
```go
parentRoot string // empty if none
```

**Recommendation:** End the comment with a period and state the condition plainly.

**Rationale:** Consistent complete comments satisfy the repository's Go documentation gate and reduce ambiguity.

### Stage 8: Generation Gates
**Score:** 6/10

#### Medium Generation: External processes bypass the shared resilience and context seam
**Location:** `internal/ops/sa/sa.go:132`, `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Medium
**Status:** fixed in Pass A

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
...
err := cmd.Run()
```

**Recommendation:** Route process execution through one context-aware seam. Require a caller deadline, honor cancellation, and apply failsafe-go retry, backoff with jitter, and circuit breaking for shared network-backed operations. Classify local command failures as permanent where retries are unsafe.

**Rationale:** Git fetch, pull, push, and tool processes can hang or create retry storms without a bounded, consistent policy.

#### Medium Generation: Bare invocation does not provide the required agent operating guide
**Location:** `internal/ops/cli/root.go:42-51`
**Severity:** Medium
**Status:** fixed in Pass A

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:** Make the no-argument root path print a structured agent guide, including role, boundaries, lifecycle commands, safe mutation flags, `AGENTS.md`, and `ai-copilots/`, then exit successfully.

**Rationale:** Agents and operators need a safe command catalog without mistaking a generic usage error for the operating contract.

#### Medium Generation: Internal operations return untyped errors
**Location:** `internal/ops/sa/sa.go:47`, `internal/ops/submodule/git.go:46`, `internal/ops/satools/satools.go:30`
**Severity:** Medium
**Status:** fixed in Pass A

**Current Code:**
```go
if err != nil {
    return err
}
```

**Recommendation:** Define or reuse a domain error type with a stable code, operation, message, and `Unwrap`, then convert causes at package boundaries.

**Rationale:** CLI callers cannot reliably classify configuration, environment, Git, and tool failures when the boundary exposes only untyped errors.

#### Low Generation: Multi-line operator menu uses ad-hoc string assembly
**Location:** `internal/ops/submodule/menu.go:8-19`
**Severity:** Low
**Status:** fixed in Pass A

**Current Code:**
```go
header := fmt.Sprintf(
    "Submodule Manager\n-----------------\nSubmodule : %s\nBranch    : %s",
    m.submoduleName, currentBranch,
)
return header + "\n\n" + strings.Join(items, "\n")
```

**Recommendation:** Use a `text/template` with a typed view model for the multi-line operator menu.

**Rationale:** Template rendering keeps layout structure visible and avoids coupling the report shape to string concatenation.

### Pass A verification
- `go test ./...`: passed.
- `go vet ./...`: passed.
- `gofmt -l cmd internal`: passed with no output.
- `go run ./cmd/majordomo`: printed the agent guide and exited successfully.
- `go run ./cmd/majordomo version`: printed `dev` and exited successfully.
- `github.com/failsafe-go/failsafe-go`: v0.9.8.

### Gate checks with no findings
- No HTTP response bodies, SQL transactions, durable timestamp writes, durable AI dump paths, or new generator/evaluator signatures are in the target.
- The CLI entrypoint has an explicit `version` command, root command catalog, and no service listener.
- Package placement remains under `cmd/` and domain-scoped `internal/` directories.
- The Stage 7 structured-logging finding covers C14.

### Stage 4: Architecture
Pending.

### Stage 5: Robustness
Pending.

### Stage 6: Testability
Pending.

## Open Questions
None yet.

## Resolutions
| Finding | Severity | Location | Outcome |
|---|---|---|---|
| golangci-lint unavailable | Medium | environment | Still open, tool is not installed in this environment. |
| Discarded CLI output errors | Medium | `internal/ops/cli/root.go` | Fixed by returning help and writer errors. |
| Static-analysis failures discarded | Medium | `internal/ops/sa/sa.go` | Fixed by collecting failures and returning a typed aggregate error. |
| Docker tag failures discarded | Medium | `internal/ops/satools/satools.go` | Fixed by failing the build result when tagging fails. |
| Git probe errors hidden | Low | `internal/ops/submodule/git.go` | Fixed with explicit probe handling and fail-closed dirty-state behavior. |
| Working-directory lookup error hidden | Low | `internal/ops/sa/sa.go` | Fixed by returning the wrapped lookup error. |
| Test setup errors discarded | Low | `internal/ops/*/*_test.go` | Fixed by failing fixture setup immediately. |
| Non-interactive SA logging | Medium | `internal/ops/sa/sa.go` | Fixed with injected `slog.Logger` fields. |
| Incomplete inline comment | Low | `internal/ops/submodule/git.go` | Fixed. |
| Process resilience and deadlines | Medium | `internal/ops/process/runner.go` | Fixed with context deadlines, failsafe retry, jitter, and network circuit breakers. |
| Bare CLI agent guide | Medium | `internal/ops/cli/root.go` | Fixed with a successful structured guide on bare invoke. |
| Untyped operation errors | Medium | `internal/ops/errors/errors.go` | Fixed with stable operation codes and cause unwrapping at entrypoints. |
| Ad-hoc multi-line menu assembly | Low | `internal/ops/submodule/menu.go` | Fixed with `text/template`. |
