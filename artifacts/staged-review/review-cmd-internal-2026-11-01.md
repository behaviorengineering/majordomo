# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...` because the cron run has no Go diff against `origin/main`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Hooks:** none discovered
**Baseline:** `origin/main`
**Pack mount:** committed separately as `bce1d46` (`chore: mount shared review pack`)
**Draft PR:** [#112](https://github.com/behaviorengineering/majordomo/pull/112)

## Stages
- [x] 1. Automated Tools
- [x] 2. Type Safety
- [x] 3. Error Handling
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity
- [x] 8. Generation Gates

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: Module checksum mismatch blocks vet and lint
**Location:** `go.sum:240`
**Severity:** Medium
**Current Code:**
```text
github.com/behaviorengineering/operatorconfig v0.1.1 h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
```
**Recommendation:** Verify the intended `github.com/behaviorengineering/operatorconfig v0.1.1` artifact and update the dependency checksum or dependency version through the module tooling. Re-run the required quality gates after the integrity issue is resolved.
**Rationale:** `go tool task vet` stopped on the checksum mismatch, and `go tool task lint` could not complete analysis. The review cannot rely on those gates while dependency integrity is unresolved.
**Status:** fixed in Pass A

#### Low Tooling: Format check is clean
**Location:** `./cmd/...`, `./internal/...`, `./pkg/...`
**Severity:** Low
**Current Code:**
```text
go tool gofumpt -l ./cmd ./internal ./pkg
```
**Recommendation:** No change required.
**Rationale:** The format check completed successfully with no files reported.
**Status:** non-issue

### Stage 2: Type Safety

No findings. The reviewed `cmd/` and `internal/` Go files do not use unchecked type assertions, bare `any` or `interface{}` values where a concrete type is apparent, or unguarded pointer/map dereferences identified by this stage.

**Score:** 10/10

### Stage 3: Error Handling

#### Medium Error Handling: CLI output errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`, `internal/ops/cli/root.go:119,513`
**Severity:** Medium
**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
```
**Recommendation:** Return output-write errors from `RunE` handlers. Convert the version handler to `RunE` and return the `fmt.Fprintln` error.
**Rationale:** Broken pipes and unavailable output streams currently report command success, hiding an operator-visible failure.
**Status:** fixed in Pass A

#### Low Error Handling: Root help errors are discarded
**Location:** `internal/ops/cli/root.go:54`
**Severity:** Low
**Current Code:**
```go
_ = cmd.Help()
return errSubcommandRequired
```
**Recommendation:** Return the help error when rendering fails, otherwise return the subcommand error.
**Rationale:** The root command can hide a failed help render and return a less useful error.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis failures are logged and then reported as success
**Location:** `internal/ops/sa/sa.go:109-112`
**Severity:** Medium
**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Continue running independent tools, but collect tool failures and return an aggregated error after the loop.
**Rationale:** A failed analyzer currently produces a zero exit status, so CI or operators can mistake an incomplete scan for a passing scan.
**Status:** fixed in Pass A

#### Medium Error Handling: `os.Getwd` failure is ignored
**Location:** `internal/ops/sa/sa.go:150`
**Severity:** Medium
**Current Code:**
```go
wd, _ := os.Getwd()
```
**Recommendation:** Check and return the error before probing parent directories.
**Rationale:** A missing working directory leaves script discovery operating on an invalid or empty path.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failure is ignored
**Location:** `internal/ops/satools/satools.go:221`
**Severity:** Medium
**Current Code:**
```go
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
```
**Recommendation:** Check the tag command error and mark the build failed or return the error.
**Rationale:** The command claims a local image was built even when the required retag step failed.
**Status:** fixed in Pass A

#### Medium Error Handling: Observability config load failure is logged and ignored
**Location:** `internal/ops/cli/root.go:98-100`
**Severity:** Medium
**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
}
```
**Recommendation:** Return the configuration error from `resolveOTELConfig` and let the command fail clearly.
**Rationale:** Continuing with partial observability settings hides malformed or inaccessible operator configuration.
**Status:** fixed in Pass A

#### Low Error Handling: Test fixture setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-43`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```
**Recommendation:** Use `if err != nil { t.Fatal(err) }` or a small test helper that fails the test on setup errors.
**Rationale:** Fixture failures can produce misleading test failures or false positives.
**Status:** fixed in Pass A

#### Low Error Handling: No-op test assignment
**Location:** `internal/ops/submodule/submodule_test.go:62`
**Severity:** Low
**Current Code:**
```go
_ = filepath.Separator
```
**Recommendation:** Remove the no-op assignment and the unused import.
**Rationale:** It obscures dead code and bypasses the unused-import check instead of testing behavior.
**Status:** fixed in Pass A

**Score:** 3/10

### Stage 7: Code Clarity

#### Medium Clarity: Static-analysis logging bypasses structured logging
**Location:** `internal/ops/sa/sa.go:33-36`
**Severity:** Medium
**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```
**Recommendation:** Inject the project logger or a structured logging seam into `sa.Options`; keep operator output at the CLI boundary.
**Rationale:** A hand-built timestamped `fmt.Printf` line loses structured fields and makes the analyzer service harder to test and integrate.
**Status:** fixed in Pass A

#### Low Clarity: Field comment is not a complete sentence
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low
**Current Code:**
```go
parentRoot    string // empty if none
```
**Recommendation:** End the comment with a period and state the empty-value meaning clearly.
**Rationale:** Complete comments satisfy the repository's `godot` convention and reduce ambiguity for maintainers.
**Status:** fixed in Pass A

**Score:** 8/10

### Stage 8: Generation Gates

#### Critical Generation Gates: Outbound subprocesses bypass resilience and caller deadlines
**Location:** `internal/ops/sa/sa.go:132`, `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Critical
**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
err := cmd.Run()
```
**Recommendation:** Introduce a shared context-aware subprocess seam using failsafe-go retry and circuit-breaker policies, classify transient versus permanent failures, require a caller deadline, and route all three callers through it.
**Rationale:** Script, Docker, and remote Git operations can hang or fail under load without cancellation, bounded work, retry classification, or circuit breaking.
**Status:** fixed in Pass A

#### Medium Generation Gates: Bare CLI invocation is not agent-ready
**Location:** `internal/ops/cli/root.go:45-57`
**Severity:** Medium
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```
**Recommendation:** Make the no-argument path print a structured agent operating guide and exit successfully. Keep config bootstrap out of that path.
**Rationale:** Agents and operators get a non-zero usage error after configuration bootstrap instead of a safe, discoverable overview.
**Status:** fixed in Pass A

#### Medium Generation Gates: Multi-line operator reports are assembled with print chains
**Location:** `internal/ops/satools/satools.go:56-96,247-253`, `internal/ops/submodule/menu.go:8-19`
**Severity:** Medium
**Current Code:**
```go
fmt.Printf("SA Tool Image Builder\nMode:      %s\nContext:   %s\nTools:     %s\nDry-run:   %v\n\n", ...)
```
**Recommendation:** Render multi-line operator reports and menus with `text/template`; keep one-line status messages as direct writes.
**Rationale:** Templates make line structure explicit and testable, reducing formatting drift in human and agent-facing output.
**Status:** fixed in Pass A

#### Low Generation Gates: Task summaries are missing
**Location:** `Taskfile.yml:8-42`
**Severity:** Low
**Current Code:**
```yaml
format:
  desc: Format with gofumpt
  cmds:
    - go tool gofumpt -w ./cmd ./internal ./pkg
```
**Recommendation:** Add concise `summary:` text to the listed operator tasks where extra usage or scope information helps.
**Rationale:** The task list is usable, but `go tool task --summary` provides no operator guidance beyond one-line descriptions.
**Status:** fixed in Pass A

**Score:** 4/10

## Pass A Fix Commit

`b968265` (`fix: apply mechanical staged review improvements`)

All Pass A findings, including Low findings, are marked fixed in the post-fix tree. The pack mount remains isolated in commit `bce1d46`.

## Pass A Validation

| Gate | Result |
|---|---|
| `go tool task format` equivalent, `go tool gofumpt -l ./cmd ./internal ./pkg` | Passed |
| `go tool task vet` | Passed |
| `go tool task lint` | Passed, 0 issues |
| `go tool task test` | Passed |
| `go tool task build` | Passed |
| CLI bare invoke, version, unknown-command smoke checks | Passed |
| `git diff --check` | Passed |

## Open Questions

#### Open Architecture: Centralized Cobra root wiring
**Location:** `internal/ops/cli/root.go:1-850`
**Observation:** The CLI root package imports 13 domain packages and defines every root and nested command constructor in one file.
**Question:** Is this centralized Cobra facade intentional, or should command constructors be split by lifecycle or domain while keeping the root as wiring?
**Possible outcomes:**
- If intentional: record a non-issue with the facade rationale.
- If it has become a maintenance bottleneck: record an Architecture finding and wait for instructions before changing it.

## Resolutions

| Finding | Outcome |
|---|---|
| Module checksum mismatch blocks vet and lint | Fixed in Pass A by upgrading operatorconfig to v0.2.0 and rerunning vet/lint |
| Format check is clean | Non-issue |
| CLI output errors are discarded | Fixed in Pass A with returned output-write errors |
| Root help errors are discarded | Fixed in Pass A with returned help errors |
| Static-analysis failures return success | Fixed in Pass A by aggregating tool failures |
| `os.Getwd` failure is ignored | Fixed in Pass A with explicit error handling |
| Docker tag failure is ignored | Fixed in Pass A with failed retag propagation |
| Observability config error is ignored | Fixed in Pass A with fail-closed config loading |
| Test fixture setup errors are discarded | Fixed in Pass A with test setup assertions |
| No-op test assignment | Fixed in Pass A by removing dead code |
| Static-analysis logging bypass | Fixed in Pass A with injected slog logging |
| Incomplete field comment | Fixed in Pass A with a complete comment |
| Outbound subprocesses bypass resilience and deadlines | Fixed in Pass A with internal/ops/execx |
| Bare CLI invocation is not agent-ready | Fixed in Pass A with an agent guide and safe bootstrap |
| Multi-line reports use print chains | Fixed in Pass A with text/template rendering |
| Task summaries are missing | Fixed in Pass A with Taskfile summaries |

