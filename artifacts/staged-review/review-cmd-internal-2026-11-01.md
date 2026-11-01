# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...` Go files, because the branch has no product diff from `main`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8; Pass B: 4, 5, 6
**Hooks:** none found at `ai-copilots/review-hooks.yaml`

## Stages
- [x] 1. Automated Tools, score 3/10, done
- [x] 2. Type Safety, score 10/10, done
- [x] 3. Error Handling, score 4/10, done
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity, score 10/10, done
- [x] 8. Generation Gates, score 4/10, done

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: Dependency checksum blocks quality gates
**Location:** `go.sum:240`
**Severity:** Medium
**Current Code:**
```text
github.com/behaviorengineering/operatorconfig v0.1.1 h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
```
**Recommendation:** Verify the module contents and refresh the stale checksum through the Go module tooling, then rerun `go tool task vet` and `go tool task lint`.
**Rationale:** The runner stopped before analyzing the target because the downloaded module hash differed from the committed checksum. A review cannot rely on incomplete gates.
**Status:** fixed in Pass A

#### Low Tooling: Format check command uses invalid package patterns
**Location:** `Taskfile.yml:14-16`
**Severity:** Low
**Current Code:**
```yaml
format:
  cmds:
    - go tool gofumpt -w ./cmd ./internal ./pkg
```
**Recommendation:** Keep the operator task on directory paths and run the format checker with directory paths as well, or use a supported recursive package invocation.
**Rationale:** The review command `go tool gofumpt -l ./cmd/... ./internal/... ./pkg/...` failed on `lstat`; that command form is not accepted by gofumpt.
**Status:** non-issue

### Stage 2: Type Safety

No findings. The target uses variadic `any` only for printf-style formatting, has no unchecked type assertions, and has no newly identified nil dereference or non-exhaustive typed enum switch.

### Stage 3: Error Handling

#### Medium Error Handling: CLI output and help errors are discarded
**Location:** `internal/ops/cli/root.go:54,119,513`
**Severity:** Medium
**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```
**Recommendation:** Return or wrap output and help-writer errors from `RunE` handlers.
**Rationale:** A broken pipe or failed writer currently reports success or masks the actual failure.
**Status:** fixed in Pass A

#### Medium Error Handling: Operator config initialization output errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`
**Severity:** Medium
**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Config already exists at %s (use --force to overwrite)\n", path)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", filepath.Join(filepath.Dir(path), "config.yaml.example"))
```
**Recommendation:** Check each write and return a wrapped error.
**Rationale:** `init` can claim success after stdout has failed.
**Status:** fixed in Pass A

#### Medium Error Handling: Observability config load failure is logged and ignored
**Location:** `internal/ops/cli/root.go:86-111`
**Severity:** Medium
**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
}
```
**Recommendation:** Return the configuration error to the orchestrate command instead of continuing with an implicit fallback.
**Rationale:** Continuing can run an orchestration job without the configured observability contract and hides a real configuration failure.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis tool failures are logged and execution continues
**Location:** `internal/ops/sa/sa.go:108-111`
**Severity:** Medium
**Current Code:**
```go
if err := runner(...); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Preserve the first tool error and return it after all configured tools have been attempted.
**Rationale:** The command otherwise exits successfully while one or more configured checks failed.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failure is discarded
**Location:** `internal/ops/satools/satools.go:217-222`
**Severity:** Medium
**Current Code:**
```go
if err == nil {
    full := "local/sa-" + tool + ":local-test"
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
```
**Recommendation:** Treat a failed tag as a failed build result and include its error in the reported output.
**Rationale:** A successful build without the required local tag produces a false positive for the next workflow step.
**Status:** fixed in Pass A

#### Low Error Handling: Best-effort Git probes discard errors
**Location:** `internal/ops/submodule/git.go:91-101,129,189`
**Severity:** Low
**Current Code:**
```go
indexEntry, _ := m.git(...)
gitDirRaw, _ := m.git(...)
entry, _ := m.git(...)
out, _ := m.git(...)
```
**Recommendation:** Make probe helpers return errors or explicitly classify and handle expected not-found results.
**Rationale:** Unexpected Git failures are indistinguishable from an absent parent, gitlink, or remote branch.
**Status:** fixed in Pass A

#### Low Error Handling: Test setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-32`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```
**Recommendation:** Assert setup errors with `t.Fatal`.
**Rationale:** A test can continue with incomplete fixtures and fail at a misleading assertion.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

No new clarity-only findings. The comments in scope are punctuated, exported symbols are tied to package APIs, and the operator output is intentionally human-facing.

### Stage 8: Generation Gates

#### High Generation Gates: Bare CLI invocation is not agent-ready
**Location:** `internal/ops/cli/root.go:39-57`
**Severity:** High
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```
**Recommendation:** Print a structured agent operating guide on a bare invocation and return success; keep detailed command syntax under `help`.
**Rationale:** Agents and operators receive an error instead of a stable guide, and a bare invocation cannot safely explain the command lifecycle.
**Status:** fixed in Pass A

#### High Generation Gates: External process hops lack context and resilience policy
**Location:** `internal/ops/sa/sa.go:132-137`, `internal/ops/satools/satools.go:230-237`, `internal/ops/submodule/git.go:218-234`
**Severity:** High
**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
...
cmd := exec.Command(name, args...)
...
cmd := exec.Command("git", args...)
```
**Recommendation:** Centralize external process execution behind a context-aware seam that requires a caller deadline, classifies retryable failures, and applies failsafe-go retry and circuit-breaker policies for network-backed commands.
**Rationale:** Docker, forge, fetch, pull, and push operations can hang or fail in bursts, while these calls currently have no cancellation budget, retry classification, or circuit breaker.
**Status:** fixed in Pass A

#### Medium Generation Gates: Static-analysis logging bypasses structured logging
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium
**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```
**Recommendation:** Inject the project logger or a testable output/logging seam and keep log fields structured.
**Rationale:** Direct formatted stdout loses machine-readable fields and makes failure correlation difficult.
**Status:** fixed in Pass A

#### Low Generation Gates: Task summaries are missing
**Location:** `Taskfile.yml:6-39`
**Severity:** Low
**Current Code:**
```yaml
tasks:
  default:
    desc: List local-dev tasks
```
**Recommendation:** Add concise `summary:` text for listed operator tasks where the task has usage or platform details.
**Rationale:** `go tool task --summary` currently provides no operator guidance beyond one-line descriptions.
**Status:** fixed in Pass A

## Open Questions

## Resolutions

- Stage 1 format-check invocation: non-issue. The direct supported check `go tool gofumpt -l ./cmd ./internal ./pkg` passed; the initial diagnostic used unsupported `...` patterns.
- Stage 1 checksum: fixed in Pass A. `operatorconfig` moved to v0.2.0, `failsafe-go` was pinned at v0.9.8, and `go mod tidy` restored verified sums.
- Stage 3 CLI output and help: fixed in Pass A. Writer failures now return wrapped errors.
- Stage 3 observability config: fixed in Pass A. Configuration load errors now stop orchestration.
- Stage 3 static analysis: fixed in Pass A. Tool failures are retained and returned after all configured tools run.
- Stage 3 Docker tagging: fixed in Pass A. Tag failures now fail the build result.
- Stage 3 Git probes: fixed in Pass A. Probe and commit outcomes are explicitly classified or returned.
- Stage 3 test setup: fixed in Pass A. Fixture creation errors now fail tests immediately.
- Stage 8 bare CLI: fixed in Pass A. Bare invocation now prints the structured agent guide and exits successfully.
- Stage 8 process resilience: fixed in Pass A. External commands now require deadlines and use the shared failsafe command seam.
- Stage 8 logging: fixed in Pass A. Static-analysis logs now use injected structured `slog`.
- Stage 8 Task summaries: fixed in Pass A. Listed tasks now expose concise summaries.

## Pass A Commit

- Commit: `6fb9f3b fix: apply mechanical staged-review findings`
- Verification: `go tool task vet`, `go tool task lint`, `go tool gofumpt -l ./cmd ./internal ./pkg`, and `go test ./...` all passed.
- State: ready to push before Pass B consultant dialogue.

