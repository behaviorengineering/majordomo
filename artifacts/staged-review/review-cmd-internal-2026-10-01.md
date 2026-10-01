# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the branch has no diff against `main`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Draft PR:** [Mechanical staged-review fixes](https://github.com/behaviorengineering/majordomo/pull/83)

## Stages
- [x] 1. Automated Tools (Pass A) — Score: 10/10 after fixes
- [x] 2. Type Safety (Pass A) — Score: 10/10
- [x] 3. Error Handling (Pass A) — Score: 10/10 after fixes
- [ ] 4. Architecture (Pass B) — in progress
- [ ] 5. Robustness (Pass B)
- [ ] 6. Testability (Pass B)
- [x] 7. Code Clarity (Pass A) — Score: 10/10
- [x] 8. Generation Gates (Pass A) — Score: 10/10 after fixes

## Pass A status
Mechanical findings were fixed in commit `2e02dc9` and pushed before consultant review.

## Findings

### Stage 1: Automated Tools

#### Low Tooling: `golangci-lint` was initially unavailable
**Location:** repository tool environment
**Severity:** Low
**Current Code:**
```text
golangci-lint: command not found
```
**Recommendation:** Keep the linter available in the review environment and rerun it after fixes.
**Rationale:** The first lint attempt could not run, which initially left lint coverage incomplete.
**Status:** fixed in Pass A

#### Low Error Handling: `fmt.Fprintf` error is ignored
**Location:** `internal/ops/submodule/git.go:239`
**Severity:** Low
**Current Code:**
```go
fmt.Fprintf(m.out(), format, args...)
```
**Recommendation:** Handle output write failures and propagate them through the interactive operation.
**Rationale:** A closed or failing output stream currently makes the operation appear successful.
**Status:** fixed in Pass A

### Stage 2: Type Safety

No findings. The target has no unsafe type assertions, bare `interface{}` values, or unguarded public pointer inputs identified by the type-safety pass.

### Stage 3: Error Handling

#### Medium Error Handling: CLI output and help errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium
**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```
**Recommendation:** Return output errors from `RunE` handlers and propagate help errors while preserving the command-required error.
**Rationale:** Broken output streams currently produce successful-looking command results.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis tool failures are only logged
**Location:** `internal/ops/sa/sa.go:108-111`
**Severity:** Medium
**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Continue running independent tools, collect failures, and return an aggregate error after the loop.
**Rationale:** The command can exit successfully even when configured analysis tools fail.
**Status:** fixed in Pass A

#### Medium Error Handling: Git status errors are treated as a clean tree
**Location:** `internal/ops/submodule/git.go:145-148`
**Severity:** Medium
**Current Code:**
```go
func (m *manager) isDirty(repoRoot string) bool {
    out, _ := m.git([]string{"status", "--porcelain"}, repoRoot, false)
    return strings.TrimSpace(out) != ""
}
```
**Recommendation:** Return the status error and stop before a reset or clean operation when Git cannot report the working tree.
**Rationale:** A failed status check can make destructive cleanup run against an unknown worktree state.
**Status:** fixed in Pass A

#### Low Error Handling: Test fixture setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-43`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```
**Recommendation:** Fail the test immediately when fixture creation fails.
**Rationale:** Tests can continue with incomplete fixtures and report misleading product failures.
**Status:** fixed in Pass A

#### Low Error Handling: Docker tag failure is ignored
**Location:** `internal/ops/satools/satools.go:221`
**Severity:** Low
**Current Code:**
```go
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
```
**Recommendation:** Include a failed tag operation in the build result and captured output.
**Rationale:** A failed post-build tag currently leaves the run marked successful.
**Status:** fixed in Pass A

#### Medium Error Handling: Worktree cleanup failure is discarded
**Location:** `internal/ops/submodule/commands.go:292-300`
**Severity:** Medium
**Current Code:**
```go
defer func() {
    m.printf("Cleaning up worktree...\n")
    _, _ = m.git([]string{"worktree", "remove", "--force", worktreePath}, m.parentRoot, false)
}()
```
**Recommendation:** Capture cleanup failure and return it when the main operation succeeded.
**Rationale:** A stale worktree can block later operations while the command reports success.
**Status:** fixed in Pass A

#### Medium Error Handling: OTEL configuration failure is ignored
**Location:** `internal/ops/cli/root.go:90-117`
**Severity:** Medium
**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
}
return observability.ResolveConfig(outputDir, settings)
```
**Recommendation:** Return the configuration error from the resolver and stop orchestration before initialization.
**Rationale:** Invalid observability configuration should not silently degrade tracing.
**Status:** fixed in Pass A

#### Medium Error Handling: Git causes and branch-state errors are collapsed
**Location:** `internal/ops/submodule/git.go:71-77,143-150`
**Severity:** Medium
**Current Code:**
```go
if err != nil {
    return "", fmt.Errorf("could not determine submodule root — not inside a git repo")
}
if err != nil {
    return "(detached HEAD)", nil
}
```
**Recommendation:** Wrap the original cause and treat only the explicit detached-HEAD diagnostic as expected.
**Rationale:** Permission, repository, and Git process failures must not become invented valid state.
**Status:** fixed in Pass A

#### Low Error Handling: Path-discovery errors lack operation context
**Location:** `internal/ops/sa/sa.go:54-92`, `internal/ops/satools/satools.go:34-89`
**Severity:** Low
**Current Code:**
```go
if err != nil {
    return err
}
```
**Recommendation:** Add operation context while preserving each cause with `%w`.
**Rationale:** Context makes CLI failures actionable without losing `errors.Is` and `errors.As` behavior.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

No findings. `gofmt -l ./cmd ./internal` reported no files, and the enabled `godot` check reported no comment-period violations.

### Stage 8: Generation Gates

#### Medium Resilience: Process execution is not context-bound or policy-wrapped
**Location:** `internal/ops/sa/sa.go:132`, `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Medium
**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
cmd := exec.Command(name, args...)
cmd := exec.Command("git", args...)
```
**Recommendation:** Route process execution through a shared context-aware seam with classified retry and circuit-breaker policies for network-backed Git operations.
**Rationale:** A hung or overloaded script, Docker, or remote Git operation can block the command without honoring a caller deadline or resilience policy.
**Status:** fixed in Pass A

#### Medium Observability: Static-analysis logging bypasses structured logging
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium
**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```
**Recommendation:** Inject the project’s structured logger or an equivalent output abstraction and include the repository/tool discriminator fields.
**Rationale:** Custom formatted output cannot be consistently filtered, parsed, or correlated with command-level failures.
**Status:** fixed in Pass A

#### Low Report Construction: Multi-line tool output is assembled with repeated `fmt.Printf`
**Location:** `internal/ops/satools/satools.go:56-96`
**Severity:** Low
**Current Code:**
```go
fmt.Printf("SA Tool Image Builder\nMode: ...\n", ...)
fmt.Printf("\nResults: %d/%d passed\n", passed, len(names))
```
**Recommendation:** Use a small `text/template` report for the multi-line operator summary, while keeping per-item progress output separate.
**Rationale:** A structured template keeps the report layout readable and prevents drift when fields change.
**Status:** fixed in Pass A

## Open Questions

#### Open Architecture: Public review runner depends on internal static-analysis operations
**Location:** `pkg/review/reviewrun/run.go:12,238-254`
**Observation:** The public `reviewrun` package imports `internal/ops/sa` and constructs `sa.Options` directly when no injected runner is supplied.
**Question:** Is this public-to-internal dependency intentional, or should the static-analysis service contract move to a public review package while the CLI-specific implementation stays under `internal/ops`?
**Possible outcomes:**
- If intentional: classify as a non-issue and document that `reviewrun` is a same-module product facade.
- If not intentional: record an architecture finding for a follow-up boundary change.

## Pass A verification

- `gofmt -l ./cmd ./internal ./pkg/review/reviewrun` passed.
- `go vet ./...` passed.
- `golangci-lint run ./cmd/... ./internal/... ./pkg/review/reviewrun/...` passed with 0 issues.
- `go test ./cmd/... ./internal/ops/... ./pkg/review/reviewrun/...` passed.
- Follow-up fixes were verified with `go test ./...`, `go vet ./...`, lint, and format checks.

## Resolutions
| Finding | Outcome |
|---|---|
| Missing `golangci-lint` | Fixed in Pass A by installing the current Go 1.27-compatible linter. |
| Unchecked interactive output write | Fixed in Pass A by recording and returning the first output error. |
| Discarded CLI help and output errors | Fixed in Pass A by returning wrapped write errors. |
| Static-analysis failures only logged | Fixed in Pass A by returning an aggregate error after independent tools finish. |
| Git status failure treated as clean | Fixed in Pass A by failing closed before reset operations. |
| Test fixture setup errors discarded | Fixed in Pass A by failing fixtures immediately. |
| Docker tag failure ignored | Fixed in Pass A by marking the build failed and preserving tag output. |
| Bare process execution | Fixed in Pass A with the context-bound shared process seam and failsafe policies. |
| Unstructured static-analysis logs and repeated report formatting | Fixed in Pass A with injected `slog` and templates. |
