# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...` Go files (no product diff versus `origin/main`)
**Selected Stages:** Pass A: 1, 2, 3, 7, 8; Pass B: 4, 5, 6
**Hooks:** none found

## Stages
- [x] 1. Automated Tools
- [x] 2. Type Safety
- [x] 3. Error Handling
- [ ] 4. Architecture (Pass B)
- [ ] 5. Robustness (Pass B)
- [ ] 6. Testability (Pass B)
- [x] 7. Code Clarity
- [x] 8. Generation Gates

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: Vet blocked by a module checksum mismatch
**Location:** `go.sum` / module download for `github.com/behaviorengineering/operatorconfig@v0.1.1`
**Severity:** Medium

**Current Code:**
```text
verifying github.com/behaviorengineering/operatorconfig@v0.1.1: checksum mismatch
downloaded: h1:nVrLCDsziZyRxFB6sH6I1DHdsQ9dgoO9HPU1+ZDMH8k=
go.sum:     h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
```

**Recommendation:** Reconcile the module cache and dependency checksum through a clean, verified module download, then rerun `go tool task vet`.
**Rationale:** The vet gate cannot compile the target packages while Go's module-authentication check rejects the downloaded dependency.
**Status:** fixed in Pass A. Upgraded `github.com/behaviorengineering/operatorconfig` to verified `v0.2.0`; vet now passes.

#### Medium Tooling: Lint scope includes a missing package tree
**Location:** `Taskfile.yml:22`
**Severity:** Medium

**Current Code:**
```yaml
cmds:
  - go tool golangci-lint run --timeout 5m ./cmd/... ./internal/... ./pkg/...
```

**Recommendation:** Limit the lint command to package trees present in this module, or add the intended `pkg/` tree before including it in the gate.
**Rationale:** The current command fails with `no go files to analyze`, so the lint gate cannot report the target's actual diagnostics.
**Status:** non-issue after rerun. The package tree is present; after dependency reconciliation, the original lint failure disappeared and the existing Taskfile scope passes.

**Stage result:** `go tool task vet` failed with exit 201, `go tool task lint` failed with exit 201, and `go tool gofumpt -l ./cmd ./internal ./pkg` passed with exit 0.

#### Low Tooling: New test compared a wrapped error directly
**Location:** `internal/ops/process/process_test.go:14`
**Severity:** Low

**Current Code:**
```go
if err != ErrMissingDeadline {
```

**Recommendation:** Use `errors.Is` for sentinel comparison.
**Rationale:** Direct comparison breaks when the implementation adds context wrapping.
**Status:** fixed in Pass A.

### Stage 2: Type Safety

No findings. The two variadic `...any` parameters are formatting APIs, type assertions were not found, and the reviewed switches have explicit defaults or operate on strings.

### Stage 3: Error Handling

#### Medium Error Handling: Operator output errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`, `internal/ops/cli/root.go:54,119,513`
**Severity:** Medium

**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
```

**Recommendation:** Return wrapped write or help errors from the command instead of discarding them.
**Rationale:** A broken pipe or failed output writer can currently leave a command appearing successful.
**Status:** fixed in Pass A.

#### Medium Error Handling: Configuration load failure is logged and ignored
**Location:** `internal/ops/cli/root.go:90-110`
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

**Recommendation:** Return the configuration error to the command caller, preserving the failure boundary.
**Rationale:** Continuing with partial observability settings can hide a malformed or unreadable operator configuration.
**Status:** fixed in Pass A.

#### Medium Error Handling: Docker tag failure is discarded
**Location:** `internal/ops/satools/satools.go:219-223`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```

**Recommendation:** Treat a failed tag operation as a failed build result and include its error in the returned output.
**Rationale:** The command can report a passed image build even when the requested final tag was not created.
**Status:** fixed in Pass A.

### Stage 7: Code Clarity

#### Low Clarity: Static-analysis progress uses an ad hoc logger
**Location:** `internal/ops/sa/sa.go:33-36`
**Severity:** Low

**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:** Use the repository's structured logging seam, or inject the output/logger dependency into `Options` so the operation is explicit and testable.
**Rationale:** The custom global-style formatter bypasses consistent fields and makes output ownership unclear.
**Status:** fixed in Pass A. Replaced the formatter with an injectable `slog.Logger`.

### Stage 8: Generation Gates

#### High CLI Surface: Bare invocation is not agent-ready
**Location:** `internal/ops/cli/root.go:39-57`
**Severity:** High

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:** Make the no-argument root path print the structured agent operating guide and exit successfully, while keeping the full flag catalog under `help`.
**Rationale:** Agents and operators receive only a failure-oriented help path and cannot discover safe lifecycle commands, documentation, or mutation rules from a bare binary invocation.
**Status:** fixed in Pass A. Bare invocation now prints a multiline guide and exits successfully; unknown commands print the catalog.

#### High Resilience: SA tool execution bypasses the outbound execution seam
**Location:** `internal/ops/sa/sa.go:132-138`
**Severity:** High

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
...
if err := cmd.Run(); err != nil {
    return fmt.Errorf("run-sa-tool.sh %s: %w", slug, err)
}
```

**Recommendation:** Route the script process through a context-aware shared execution seam with required deadlines, retry classification, and the repository's outbound resilience policy.
**Rationale:** Static-analysis jobs can invoke Docker or remote-backed tooling, but this path has no cancellation, deadline validation, retry policy, or circuit breaker.
**Status:** fixed in Pass A. Added `internal/ops/process` and threaded the CLI job context.

#### High Resilience: SA image build execution bypasses the outbound execution seam
**Location:** `internal/ops/satools/satools.go:230-237`
**Severity:** High

**Current Code:**
```go
cmd := exec.Command(name, args...)
cmd.Env = env
cmd.Dir = dir
...
err := cmd.Run()
```

**Recommendation:** Use the shared context-aware execution seam and classify Docker/build failures instead of issuing bare process calls.
**Rationale:** The command runs build and Docker processes without a caller deadline or resilience controls, so hung or overloaded dependencies can stall the CLI.
**Status:** fixed in Pass A. Docker and build commands now use the shared process runner.

#### High Resilience: Submodule git execution bypasses the outbound execution seam
**Location:** `internal/ops/submodule/git.go:222-232`
**Severity:** High

**Current Code:**
```go
cmd := exec.Command("git", args...)
cmd.Dir = cwd
...
err := cmd.Run()
```

**Recommendation:** Thread a caller context through the interactive manager and route remote-capable git operations through the shared execution seam with deadline and retry classification.
**Rationale:** Fetch, pull, push, and worktree operations can block on remote SCM state, but the current runner cannot be canceled or bounded.
**Status:** fixed in Pass A. Submodule operations now receive the CLI job context and use the shared process runner.

#### Medium Observability: Dispatch command does not initialize process tracing
**Location:** `internal/ops/cli/root.go:202-230`
**Severity:** Medium

**Current Code:**
```go
return dispatch.Dispatch(opts)
```

**Recommendation:** Initialize observability and a client-side span around in-process Judge generation/evaluation before dispatching.
**Rationale:** The dispatch command can run LLM work but currently has no entrypoint tracer setup or client span seam, leaving parse, evaluation, and post-response failures invisible outside the gateway.
**Status:** fixed in Pass A. Dispatch now initializes observability and wraps the operation in a chain span.

## Open Questions

## Resolutions

- Pack mount: committed separately before product fixes.
- Stage 1 checksum: fixed by upgrading to verified `operatorconfig v0.2.0`.
- Stage 1 lint scope: non-issue after dependency reconciliation; all package scopes pass.
- Stage 2 type safety: non-issue.
- Stage 3 output/config/tag errors: fixed in Pass A.
- Stage 7 logging: fixed in Pass A with `slog`.
- Stage 8 CLI, process resilience, and dispatch tracing: fixed in Pass A.

## Pass A Verification

- `go tool task format`: passed.
- `go tool task lint`: passed with 0 issues.
- `go tool task vet`: passed.
- `go tool task test`: passed.
- `go tool task build`: passed.
- CLI checks: bare invoke prints 23 guide lines and exits 0; `version` exits 0; unknown commands print the catalog and exit non-zero.

