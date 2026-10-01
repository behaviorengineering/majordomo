# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the branch has no Go diff against `origin/main`
**Selected Stages:** Pass A mechanical `1, 2, 3, 7, 8`; Pass B consultant `4, 5, 6`

## Stages
- [x] 1. Automated Tools, score 9/10, done
- [x] 2. Type Safety, score 10/10, done
- [x] 3. Error Handling, score 7/10, done
- [ ] 4. Architecture, in progress
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity, score 8/10, done
- [x] 8. Generation Gates, score 6/10, done

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: golangci-lint is unavailable
**Location:** Repository toolchain, no file location.
**Severity:** Medium
**Current Code:** `golangci-lint` is not installed, so the configured lint command could not run.
**Recommendation:** Install or invoke the repository's current golangci-lint version, then run it against the review target.
**Rationale:** Without lint coverage, configured checks such as security, comment, and error-handling rules remain unverified.
**Status:** fixed in Pass A

**Tool results:** `go vet ./...` passed. `gofmt -l ./cmd ./internal` returned no files. No `Makefile` exists.

### Stage 2: Type Safety

No confirmed findings. The two variadic `any` parameters are used for
formatting APIs where the values are intentionally unconstrained. No unchecked
type assertions were found in the target packages, and reviewed public option
entry points use value options rather than nullable pointer inputs.

### Stage 3: Error Handling

#### Medium Error Handling: SA build result ignores image-tag failure
**Location:** `internal/ops/satools/satools.go:221`
**Severity:** Medium
**Current Code:**
```go
if err == nil {
	full := "local/sa-" + tool + ":local-test"
	_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```
**Recommendation:** Check the tag command error and include its output in the result so a failed tag cannot be reported as a successful build.
**Rationale:** The command can return success even though the image tag needed by callers was not created.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis tool failures are logged and discarded
**Location:** `internal/ops/sa/sa.go:108-112`
**Severity:** Medium
**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
	logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Continue running the remaining tools, collect each failure with its tool name, and return the aggregate error after the batch.
**Rationale:** The current function reports failure but returns success, so callers and CI cannot enforce static-analysis results.
**Status:** fixed in Pass A

#### Medium Error Handling: CLI output errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium
**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```
**Recommendation:** Use `RunE` where needed and return output errors, wrapping them with the command operation.
**Rationale:** Broken pipes and unavailable output writers currently produce a false-success command.
**Status:** fixed in Pass A

#### High Error Handling: Process exit bypasses deferred cleanup
**Location:** `cmd/majordomo/main.go:17-39`
**Severity:** High
**Current Code:**
```go
if err := cli.NewRoot().ExecuteContext(runCtx); err != nil {
	...
	os.Exit(1)
}
```
**Recommendation:** Return an exit code from a helper that owns the defers, then call `os.Exit` only after that helper returns.
**Rationale:** `os.Exit` skips deferred gateway shutdown and observability flush, so failed commands can lose telemetry and cleanup.
**Status:** fixed in Pass A follow-up

#### High Error Handling: Git command failures were hidden by optional checks
**Location:** `internal/ops/submodule/git.go:90-239`; `internal/ops/submodule/commands.go:105-319`
**Severity:** High
**Current Code:**
```go
if !check {
	return out, nil
}
```
**Recommendation:** Preserve command errors, classify only known detached, missing-reference, and no-op commit cases, and propagate all other failures.
**Rationale:** A failed status, commit, remote lookup, or parent-gitlink check could previously look like a clean or successful operation.
**Status:** fixed in Pass A follow-up

#### Low Test Setup: Test filesystem errors are discarded
**Location:** `internal/ops/satools/satools_test.go:12-14,30-32,42-43`; `internal/ops/sa/sa_test.go:12-14,31-32`; `internal/ops/submodule/submodule_test.go:62`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
_ = filepath.Separator
```
**Recommendation:** Fail tests on setup errors and remove the no-op separator statement and its unused import.
**Rationale:** Ignored setup failures can make tests pass without exercising their intended fixtures, and the separator statement hides dead code.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Medium Logging: Static-analysis logging bypasses structured output
**Location:** `internal/ops/sa/sa.go:33-36`
**Severity:** Medium
**Current Code:**
```go
func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```
**Recommendation:** Inject the command's logging/output seam and use the repository's structured logging equivalent, keeping operator output separate from service diagnostics.
**Rationale:** Direct process stdout logging cannot carry fields, be redirected consistently, or be controlled by callers and tests.
**Status:** fixed in Pass A

#### Low Clarity: Em dashes and incomplete inline comments
**Location:** `internal/ops/cli/root.go:43`; `internal/ops/submodule/{git.go,commands.go,menu.go}`; `internal/ops/sa/sa.go:51`
**Severity:** Low
**Current Code:**
```go
Long: `Majordomo — repository operations for evolving software.
parentRoot string // empty if none
```
**Recommendation:** Use ASCII punctuation required by the workspace rules and make inline comments complete sentences.
**Rationale:** Consistent punctuation keeps source and operator text portable and satisfies the repository language rules.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium Resilience: Outbound process execution lacks shared retry, breaker, and deadline policy
**Location:** `internal/ops/satools/satools.go:230`; `internal/ops/sa/sa.go:132`; `internal/ops/submodule/git.go:222`
**Severity:** Medium
**Current Code:**
```go
cmd := exec.Command(name, args...)
err := cmd.Run()
```
**Recommendation:** Route outbound process execution through one context-aware seam that requires a caller deadline and applies classified failsafe-go retry and circuit-breaker policies for network-backed commands.
**Rationale:** Git, Docker, and script calls can hang or create retry storms when remote dependencies fail, and the current APIs cannot honor cancellation.
**Status:** fixed in Pass A

#### Medium CLI Surface: Bare invocation returns an error instead of an agent-ready guide
**Location:** `internal/ops/cli/root.go:48-52`
**Severity:** Medium
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
	_ = cmd.Help()
	return errSubcommandRequired
},
```
**Recommendation:** Make the no-argument root path print the required operating guide and exit successfully, while keeping explicit subcommands for all actions.
**Rationale:** Automation cannot safely discover command roles, lifecycle, and mutation controls from a one-line failure path.
**Status:** fixed in Pass A

## Open Questions

#### Open Architecture: Single CLI registry file
**Location:** `internal/ops/cli/root.go:39-850`
**Observation:** The CLI package keeps the root command and constructors for roughly
15 commands in one file and imports many service packages, while the individual
handlers mostly delegate to those packages.
**Question:** Is keeping this command registry in one file intentional, or should
the command constructors split by lifecycle or domain?
**Possible outcomes:**
- If intentional: non-issue, retain the registry as the CLI composition boundary.
- If not intentional: Medium architecture finding, split command wiring without moving business logic into the CLI.

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| golangci-lint unavailable | Medium | Toolchain | Fixed in Pass A by installing a Go 1.27-compatible latest tool and running the target lint gate. |
| SA image tag errors discarded | Medium | `internal/ops/satools/satools.go` | Fixed in Pass A by returning tag failures and preserving command output. |
| Static-analysis failures masked | Medium | `internal/ops/sa/sa.go` | Fixed in Pass A by aggregating tool failures and returning them after the batch. |
| CLI output errors discarded | Medium | `internal/ops/cli/root.go` | Fixed in Pass A by checking help, version, and cursor output writes. |
| Test fixture errors discarded | Low | `internal/ops/*/*_test.go` | Fixed in Pass A by failing tests on setup errors and removing dead code. |
| Unstructured static-analysis logging | Medium | `internal/ops/sa/sa.go` | Fixed in Pass A with an injected `slog.Logger`. |
| Em dashes and incomplete comments | Low | CLI and submodule operations | Fixed in Pass A with ASCII punctuation and complete comments. |
| Unbounded process execution | Medium | `internal/ops/{satools,sa,submodule}` | Fixed in Pass A with deadline-checked `executil`, failsafe-go retry, and circuit-breaker policies. |
| Bare CLI lacks agent guide | Medium | `internal/ops/cli/root.go` | Fixed in Pass A with a successful bare-invocation operating guide and regression test. |
| Deferred cleanup bypassed by `os.Exit` | High | `cmd/majordomo/main.go` | Fixed in Pass A follow-up by returning the exit code after cleanup. |
| Git errors hidden by `check=false` | High | `internal/ops/submodule` | Fixed in Pass A follow-up by preserving errors and classifying expected no-op cases. |

## Pass A Verification

- `go vet ./cmd/... ./internal/...`: passed.
- `gofmt -l ./cmd ./internal`: passed.
- `golangci-lint run ./cmd/... ./internal/...`: passed with 0 issues.
- `go test ./internal/ops/... ./cmd/...`: passed.
- `go test ./...`: passed.
- `go vet ./...`: passed.
- Follow-up verification: `go test ./...`, `go vet ./...`, and target lint all passed.
- Added dependency: `github.com/failsafe-go/failsafe-go v0.9.8`.

## Pass A Commit

`00a8f46 fix: apply mechanical staged review findings`

Follow-up commit: `18c2eae fix: preserve cleanup and Git errors`

## Draft Pull Request
https://github.com/behaviorengineering/majordomo/pull/92
