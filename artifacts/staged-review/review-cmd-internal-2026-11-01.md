# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8; Pass B: 4, 5, 6
**Hooks:** none discovered

## Stages
- [x] 1. Automated Tools — Score: 3/10
- [x] 2. Type Safety — Score: 10/10
- [x] 3. Error Handling — Score: 4/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity — Score: 6/10
- [x] 8. Generation Gates — Score: 3/10

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: Dependency checksum mismatch blocks vet
**Location:** `go.mod:8`, `go.sum` entry for `github.com/behaviorengineering/operatorconfig@v0.1.1`
**Severity:** Medium
**Current Code:**
```text
go tool task vet
verifying github.com/behaviorengineering/operatorconfig@v0.1.1: checksum mismatch
downloaded: h1:nVrLCDsziZyRxFB6sH6I1DHdsQ9dgoO9HPU1+ZDMH8k=
go.sum:     h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
```
**Recommendation:** Verify the intended upstream module contents and update the dependency checksum only after that verification; do not bypass Go checksum validation.
**Rationale:** The vet gate cannot compile the module while the downloaded dependency differs from the locked checksum. Bypassing the check could execute untrusted dependency contents.
**Status:** fixed in Pass A

#### Medium Tooling: Lint gate cannot analyze the module
**Location:** `Taskfile.yml:13-15`
**Severity:** Medium
**Current Code:**
```yaml
lint:
  desc: Run golangci-lint
  cmds:
    - go tool golangci-lint run --timeout 5m ./cmd/... ./internal/... ./pkg/...
```
**Recommendation:** Re-run the pinned lint gate after resolving the dependency integrity failure and record the actual diagnostics.
**Rationale:** The gate exited with `no go files to analyze`, so lint results are not trustworthy until module loading succeeds.
**Status:** fixed in Pass A

#### Low Tooling: Formatting check passed
**Location:** `./cmd/...`, `./internal/...`, `./pkg/...`
**Severity:** Low
**Current Code:**
```text
go tool gofumpt -l ./cmd ./internal ./pkg
```
**Recommendation:** Keep the current formatting.
**Rationale:** The pinned formatter reported no files.
**Status:** fixed in Pass A

### Stage 3: Error Handling

#### Medium Error Handling: CLI output and help errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`, `internal/ops/cli/root.go:54,119,513`
**Severity:** Medium
**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
```
**Recommendation:** Return writer and help errors from `RunE` handlers, and make `version` use `RunE`.
**Rationale:** Broken pipes and failed output writers currently report successful command execution and hide help-rendering failures.
**Status:** fixed in Pass A

#### Medium Error Handling: Observability config errors are logged and ignored
**Location:** `internal/ops/cli/root.go:86-110`
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
**Recommendation:** Return the config-load error to the orchestrate command instead of continuing with an empty configuration.
**Rationale:** The command can run with a different observability setup than the operator requested, while the only error path is an unchecked diagnostic.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failure is silently converted to success
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
**Recommendation:** Check the tag command result and include its output in the failed build result.
**Rationale:** A failed image tag leaves the advertised output tag unavailable even though the tool reports a passing build.
**Status:** fixed in Pass A

#### Low Error Handling: Git discovery errors are discarded
**Location:** `internal/ops/submodule/git.go:96,100,129,146,189`
**Severity:** Low
**Current Code:**
```go
indexEntry, _ := m.git(...)
gitDirRaw, _ := m.git(...)
entry, _ := m.git(...)
out, _ := m.git(...)
```
**Recommendation:** Handle optional probe errors explicitly and return the documented empty or false result without discarded assignments.
**Rationale:** The current code hides command failures and makes an unavailable repository state indistinguishable from a clean negative probe.
**Status:** fixed in Pass A

#### Low Error Handling: Test setup errors and a meaningless expression are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-43`, `internal/ops/submodule/submodule_test.go:62`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = filepath.Separator
```
**Recommendation:** Fail tests immediately when setup writes fail and remove the unused expression.
**Rationale:** Tests can continue with incomplete fixtures, and the expression provides no coverage or behavior.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Medium Code Clarity: Non-interactive SA logging bypasses structured output
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium
**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```
**Recommendation:** Route operational logging through an injected structured logger or an explicit output boundary owned by the CLI.
**Rationale:** The static-analysis service writes directly to process stdout, so callers cannot control, classify, or capture its logs consistently.
**Status:** fixed in Pass A

#### Low Code Clarity: User-facing strings use em-dash punctuation
**Location:** `internal/ops/cli/root.go:43`, `internal/ops/sa/sa.go:51`, `internal/ops/submodule/{commands.go,git.go,menu.go}`
**Severity:** Low
**Current Code:**
```go
Majordomo — repository operations for evolving software.
```
**Recommendation:** Use commas, colons, or semicolons in code strings and messages.
**Rationale:** The repository chat and code-writing rules prohibit U+2014, and consistent punctuation improves portability across terminals and generated output.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium Generation Gates: Outbound process execution has no context or resilience policy
**Location:** `internal/ops/sa/sa.go:130-139`, `internal/ops/satools/satools.go:225-238`, `internal/ops/submodule/git.go:220-235`
**Severity:** Medium
**Current Code:**
```go
cmd := exec.Command(name, args...)
cmd.Dir = cwd
err := cmd.Run()
```
**Recommendation:** Route external script, Docker, and Git execution through one context-aware seam that requires a caller deadline, honors cancellation, classifies retryable failures, and applies the repository's failsafe-go retry and breaker policy for network-backed commands.
**Rationale:** A hung process or transient fetch/push failure can block the CLI indefinitely or cause avoidable review and submodule outages.
**Status:** fixed in Pass A

#### Medium Generation Gates: LLM-capable dispatch lacks visible process tracing setup
**Location:** `internal/ops/cli/root.go:188-246`
**Severity:** Medium
**Current Code:**
```go
if useOpenCode {
    return dispatch.RunOpenCode(opts)
}
return dispatch.Dispatch(opts)
```
**Recommendation:** Initialize the process tracer and create client-side inference spans before dispatch paths that run LLM work, or document and test why those paths are offline-only.
**Rationale:** Gateway HTTP traces do not expose hangs or parse/evaluation failures after a response returns to the calling process.
**Status:** fixed in Pass A

#### Medium Generation Gates: Multi-line SA output is assembled with chained print calls
**Location:** `internal/ops/satools/satools.go:56-98,240-253`
**Severity:** Medium
**Current Code:**
```go
fmt.Printf("SA Tool Image Builder\nMode:      %s\nContext:   %s\nTools:     %s\nDry-run:   %v\n\n", ...)
fmt.Printf("\nResults: %d/%d passed\n", passed, len(names))
```
**Recommendation:** Render the operator report through a `text/template` with an explicit output writer, and return writer errors.
**Rationale:** Template-backed reports keep section layout stable and make output independently testable without hiding write failures.
**Status:** fixed in Pass A

#### Low Generation Gates: Task summaries are missing
**Location:** `Taskfile.yml:6-38`
**Severity:** Low
**Current Code:**
```yaml
tasks:
  lint:
    desc: Run golangci-lint
```
**Recommendation:** Add concise `summary` text for the listed local-dev tasks, including scope and argument behavior where relevant.
**Rationale:** Operators can list tasks, but cannot inspect what each task does beyond its one-line label.
**Status:** fixed in Pass A

## Open Questions

None. The mutable release tag was replaced with an immutable commit pseudo-version after comparing the tag with the public Go checksum record.

## Resolutions
- Stage 1 Low formatting check: no issue, pinned gofumpt reported no files.
- Stage 2: no type-safety findings.
- Stage 1 Medium dependency integrity: fixed by pinning `operatorconfig` to immutable commit `d6bc463b00f2eeb476f8936513e89de3dd71bd17`; vet and lint pass.
- Stage 1 Medium lint loading: fixed by restoring module loading; lint reports 0 issues.
- Stage 3 Medium CLI output: fixed by returning help and writer errors.
- Stage 3 Medium observability config: fixed by returning config-load errors.
- Stage 3 Medium Docker tag: fixed by propagating tag failures and output.
- Stage 3 Low Git probes: fixed by handling optional probe errors explicitly.
- Stage 3 Low test setup: fixed by checking fixture writes and removing the unused expression.
- Stage 7 Medium SA logging: fixed with an explicit output boundary and returned write errors.
- Stage 7 Low punctuation: fixed by replacing prohibited em-dash and arrow punctuation.
- Stage 8 Medium process execution: fixed with deadline-checked, context-aware failsafe execution.
- Stage 8 Medium dispatch tracing: fixed by initializing OTEL and wrapping dispatch in a process span.
- Stage 8 Medium reports: fixed with template-backed output and returned writer errors.
- Stage 8 Low Task summaries: fixed by documenting every listed task.

## Pass A Fix Verification
- `go tool gofumpt -l ./cmd ./internal ./pkg`: passed.
- `go tool task test`: passed for the full module.
- `go tool task vet`: passed.
- `go tool task lint`: passed with 0 issues.
- `go tool task build`: passed.
