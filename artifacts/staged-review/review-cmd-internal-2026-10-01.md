# Staged Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the branch has no Go diff against `origin/main`
**Selected stages:** Pass A: 1, 2, 3, 7, 8; Pass B: 4, 5, 6
**Stage mapping:** legacy 7 = current Stage 4 (Code Clarity); legacy 8 = current Stage 5 (Generation Gates); consultant 4 = A, 5 = B, 6 = C

## Stages

- [x] 1. Automated Tools: 8/10
- [x] 2. Type Safety: 10/10
- [x] 3. Error Handling: 5/10
- [x] 7. Code Clarity: 9/10
- [x] 8. Generation Gates: 4/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: `golangci-lint` is unavailable
**Location:** repository toolchain, lint command
**Severity:** Medium
**Current Code:**
```text
golangci-lint run ./cmd/... ./internal/  # command not found
```
**Recommendation:** Run the configured lint checks with an available `golangci-lint` binary or an equivalent reproducible tool invocation before the review is complete.
**Rationale:** Without lint coverage, configured static checks such as security and comment rules are unverified.
**Status:** open

#### Low Tooling: no project Makefile quality targets
**Location:** repository root
**Severity:** Low
**Current Code:**
```text
Makefile: absent
```
**Recommendation:** Provide documented `vet`, `lint`, `format`, and `test` commands through the project workflow or Makefile.
**Rationale:** Standardized quality entry points reduce review and operator drift.
**Status:** fixed in Pass A

**Tool results:** `go vet ./cmd/... ./internal/...` passed. `gofmt -l cmd internal` reported no files.

### Stage 2: Type Safety

No findings. The only `any` uses are variadic formatting arguments, and the target contains no unchecked type assertions found by the scan. Constructor and pointer-input checks did not reveal a mechanical type-safety defect.

### Stage 3: Error Handling

#### Medium Error Handling: ignored command and output errors
**Location:** `internal/ops/cli/root.go:50,113,502`; `internal/ops/satools/satools.go:221`
**Severity:** Medium
**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
```
**Recommendation:** Return or explicitly surface each failure. Treat a failed image tag as a failed build result.
**Rationale:** Operators can receive a success status after help/output or required post-build work failed.
**Status:** fixed in Pass A

#### Medium Error Handling: static-analysis failures are logged and discarded
**Location:** `internal/ops/sa/sa.go:108-112`
**Severity:** Medium
**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Continue running independent tools, but collect failures and return an error after all tools complete.
**Rationale:** The current function reports success even when one or more configured checks fail.
**Status:** fixed in Pass A

#### Medium Error Handling: package-boundary errors lose operation context
**Location:** `internal/ops/sa/sa.go:48,59,64,82`; `internal/ops/satools/satools.go:30,36,68`; `internal/ops/submodule/git.go:166`; `internal/ops/submodule/menu.go`
**Severity:** Medium
**Current Code:**
```go
return err
```
**Recommendation:** Wrap each returned cause with the operation being performed, preserving `%w`.
**Rationale:** Bare propagation makes failures harder to diagnose and does not provide a stable operation boundary.
**Status:** fixed in Pass A

#### Medium Error Handling: submodule discovery drops the underlying error
**Location:** `internal/ops/submodule/git.go:75`
**Severity:** Medium
**Current Code:**
```go
return "", fmt.Errorf("could not determine submodule root: not inside a git repo: %w", err)
```
**Recommendation:** Wrap the `git` error with `%w` while retaining the operator-facing context.
**Rationale:** The caller loses the cause and cannot inspect or classify the failure.
**Status:** fixed in Pass A

#### Low Error Handling: test setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-14,31-32`; `internal/ops/satools/satools_test.go:12-14,30-32,42-43`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```
**Recommendation:** Fail the test immediately when fixture setup fails.
**Rationale:** A broken fixture can produce misleading test failures or false passes.
**Status:** fixed in Pass A

#### Low Error Handling: dummy assignment hides an unused import
**Location:** `internal/ops/submodule/submodule_test.go:62`
**Severity:** Low
**Current Code:**
```go
_ = filepath.Separator
```
**Recommendation:** Remove the unused import and dummy assignment.
**Rationale:** Dummy work obscures test intent and bypasses normal compiler feedback.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

No new findings. Comments in the target end with sentence punctuation, names are specific enough for their packages, and the `fmt` output sites are operator-facing CLI output rather than hidden service logging. Existing error and output concerns remain tracked under Stage 3.

### Stage 8: Generation Gates

#### Medium C22/C8: outbound process execution has no shared resilience or caller deadline
**Location:** `internal/ops/sa/sa.go:132`; `internal/ops/satools/satools.go:230`; `internal/ops/submodule/git.go:222`
**Severity:** Medium
**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
cmd := exec.Command(name, args...)
cmd := exec.Command("git", args...)
```
**Recommendation:** Route outbound process execution through a context-aware shared runner that requires a caller deadline, honors cancellation, and applies classified retry and circuit-breaker policies for remote-backed operations.
**Rationale:** Unbounded or repeatedly failing Git and tool subprocesses can hang jobs and amplify remote failures.
**Status:** fixed in Pass A

#### Medium C7/C4: bare CLI invocation provides only help-as-error
**Location:** `internal/ops/cli/root.go:48-51`
**Severity:** Medium
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```
**Recommendation:** Print the structured agent operating guide on bare invocation and exit successfully without starting work.
**Rationale:** Automation cannot discover safe inspect, plan, and mutation commands from a successful default view.
**Status:** fixed in Pass A

#### Medium C15: the dispatch command does not initialize client-side inference tracing
**Location:** `internal/ops/cli/root.go:193-225`
**Severity:** Medium
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    opts := dispatch.DispatchOptions{
        Context:    cmd.Context(),
        PRNumber:   args[0],
        StagingDir: args[1],
        OutputDir:  args[2],
        Mode:       mode,
        ScriptsDir: scriptsDir,
    }
    return dispatch.Dispatch(opts)
}
```
**Recommendation:** Initialize the project tracer and start a client-side span for the in-process dispatch path before invoking the Judge.
**Rationale:** Gateway HTTP traces do not show local generation, parsing, or evaluation failures after the response.
**Status:** fixed in Pass A

#### Low C13: operator menu shape is assembled imperatively
**Location:** `internal/ops/submodule/menu.go:8-20`
**Severity:** Low
**Current Code:**
```go
header := fmt.Sprintf("Submodule Manager\n-----------------\nSubmodule : %s\nBranch    : %s", ...)
return header + "\n\n" + strings.Join(items, "\n")
```
**Recommendation:** Render the multi-line menu through a small `text/template`.
**Rationale:** Template structure keeps the operator-facing layout visible and prevents formatting drift.
**Status:** fixed in Pass A

No C1, C2, C3, C16, C17, C18, C24, or C25 findings were found in the target.

## Pass A Fixes and Verification

- Added `internal/ops/process` with context and deadline checks, failsafe retry and circuit-breaker policies, classified remote failures, and focused tests.
- Routed SA tools, Docker tag commands, and submodule Git commands through the process runner.
- Returned ignored output, fixture, tag, and static-analysis failures.
- Added a structured bare CLI agent guide, version output error handling, dispatch tracing, and a template-rendered submodule menu.
- Added a documented Makefile quality surface.
- `make format`, `make vet`, `go test ./...`, and `make build` passed.
- `make lint` remains blocked because `golangci-lint` is not installed. The latest available v1.64.8 binary also cannot analyze this Go 1.27 module because its analysis engine only supports an older export-data format.

## Open Questions

## Resolutions

| Finding | Outcome |
|---|---|
| Stage 1: missing `golangci-lint` | Still open, environment tool is unavailable |
| Stage 1: missing Makefile quality targets | Fixed in Pass A, Makefile added |
| Stage 3: ignored command and output errors | Fixed in Pass A |
| Stage 3: static-analysis failures discarded | Fixed in Pass A, failures are joined and returned |
| Stage 3: bare package-boundary errors | Fixed in Pass A, command operations now wrap causes |
| Stage 3: dropped Git discovery cause | Fixed in Pass A |
| Stage 3: discarded fixture errors | Fixed in Pass A |
| Stage 3: dummy test assignment | Fixed in Pass A |
| Stage 8: outbound process resilience | Fixed in Pass A, shared process runner added |
| Stage 8: bare CLI guide | Fixed in Pass A |
| Stage 8: dispatch tracing | Fixed in Pass A |
| Stage 8: menu layout | Fixed in Pass A, template added |
