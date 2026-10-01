# Staged Review Plan: cmd-internal

**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` (no diff versus `origin/main`)
**Pass A:** Stages 1, 2, 3, 7, 8, detect and fix mechanical findings.
**Pass B:** Stages 4, 5, 6, consultant questions only.
**Branch:** `cursor/staged-code-review-process-b29f`

## Stages

- [x] 1. Automated Tools — Score: 9/10
- [x] 2. Type Safety — Score: 10/10
- [x] 3. Error Handling — Score: 5/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity — Score: 9/10
- [x] 8. Generation Gates — Score: 4/10

## Progress

Stage 8 is complete. Pass A fixes are committed as `b15ff34`, pushed, and verified. Full tests, full vet, formatting, targeted race tests, bare CLI invocation, and version output pass. The lint gate remains blocked by a toolchain incompatibility. Pass B Stage 4 is in progress.

## Findings

### Stage 1: Automated Tools

#### Low Tooling: `golangci-lint` is unavailable

**Location:** Review environment, Stage 1 lint command.
**Severity:** Low.
**Current Code:** The latest installed `golangci-lint` binary was built with Go 1.27, but its analysis engine cannot decode this module's Go 1.27 export data (`export data version 4 is greater than maximum supported version 2`).
**Recommendation:** Use a `golangci-lint` release that supports the module's Go export-data version, then rerun the target lint command.
**Rationale:** Vet and formatting pass, but lint-specific checks remain unavailable because the current linter build is incompatible with the module toolchain.
**Status:** Open environment finding.

Stage 1 commands:

- `go vet ./cmd/... ./internal/...`: passed.
- `gofmt -l` on tracked Go files under `cmd/` and `internal/`: no output.
- `golangci-lint run ./cmd/... ./internal/...`: not run, executable missing.

### Stage 2: Type Safety

No findings. The target implementation has no bare `interface{}` or `any` declarations, no unchecked type assertions, and no identified nil dereference or constructor dependency gaps.

### Stage 3: Error Handling

#### Medium Persistence/Process: Docker tag failure is discarded

**Location:** `internal/ops/satools/satools.go:221`.
**Severity:** Medium.
**Current Code:**

```go
if err == nil {
    full := "local/sa-" + tool + ":local-test"
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```

**Recommendation:** Check the tag command error and report the build as failed when tagging fails.
**Rationale:** The command can report a successful build while the image that callers expect was never created.
**Status:** Fixed in Pass A.

#### Medium Error Handling: Root help failure is discarded

**Location:** `internal/ops/cli/root.go:50`.
**Severity:** Medium.
**Current Code:**

```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:** Return a wrapped help error when rendering help fails, otherwise return `errSubcommandRequired`.
**Rationale:** Broken output streams or help templates currently hide the real failure.
**Status:** Fixed in Pass A.

#### Medium Error Wrapping: Git discovery error dropped its cause

**Location:** `internal/ops/submodule/git.go:76`.
**Severity:** Medium.
**Current Code:** `fmt.Errorf("could not determine submodule root: not inside a git repo")`.
**Recommendation:** Preserve the Git runner cause with `%w`.
**Rationale:** Callers and tests need `errors.Is` and `errors.As` to distinguish repository state from execution failures.
**Status:** Fixed in Pass A follow-up.

#### Low CLI Output: Command output write errors are discarded

**Location:** `internal/ops/cli/root.go:113` and `internal/ops/cli/root.go:502`.
**Severity:** Low.
**Current Code:**

```go
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:** Return each writer error from the command's `RunE`.
**Rationale:** Broken pipes and closed output streams should produce a clear command failure.
**Status:** Fixed in Pass A.

#### Medium Error Boundary: Static-analysis tool failures are logged and ignored

**Location:** `internal/ops/sa/sa.go:109`.
**Severity:** Medium.
**Current Code:**

```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Continue running the remaining tools, collect failures, and return an aggregate error after the loop.
**Rationale:** CI and callers receive success even when one or more requested analysis tools fail.
**Status:** Fixed in Pass A.

#### Low Error Handling: Working-directory lookup error is discarded

**Location:** `internal/ops/sa/sa.go:150`.
**Severity:** Low.
**Current Code:**

```go
wd, _ := os.Getwd()
dir := wd
```

**Recommendation:** Return the `os.Getwd` error before building the search path.
**Rationale:** An empty path can make script discovery inspect the wrong locations and hide the actual operating-system failure.
**Status:** Fixed in Pass A.

#### Medium Configuration: OTEL config load failure silently falls back

**Location:** `internal/ops/cli/root.go:93`.
**Severity:** Medium.
**Current Code:**

```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
} else {
    // use loaded settings
}
```

**Recommendation:** Return the configuration error from `resolveOTELConfig` and make the command fail before starting its work.
**Rationale:** A requested configuration can be malformed or inaccessible, and silently using defaults makes the run differ from operator intent.
**Status:** Fixed in Pass A.

The Git calls that intentionally use `check=false` are non-fatal probes. The helper returns an empty result and no error for that mode, so those sites do not discard an error that the helper promises to expose.

#### Low Test Setup: Fixture setup errors are discarded

**Location:** `internal/ops/satools/satools_test.go:12-43`, `internal/ops/sa/sa_test.go:12-32`, and `internal/ops/submodule/submodule_test.go:62`.
**Severity:** Low.
**Current Code:** Test setup uses `_ = os.MkdirAll(...)`, `_ = os.WriteFile(...)`, and `_ = filepath.Separator`.
**Recommendation:** Fail the test when filesystem setup fails and remove the no-op separator assignment.
**Rationale:** Ignored setup errors can turn an environment problem into a misleading assertion failure or hide dead code.
**Status:** Fixed in Pass A.

### Stage 7: Code Clarity

#### Low User Interface: Em dash punctuation violates the workspace rule

**Location:** User-facing strings in `internal/ops/cli/root.go`, `internal/ops/submodule/*.go`, and `internal/ops/sa/sa.go`.
**Severity:** Low.
**Current Code:** Examples include `Majordomo — repository operations for evolving software.` and `Cancelled — local changes preserved.`
**Recommendation:** Replace em dashes with colons, commas, or semicolons while preserving the messages.
**Rationale:** The workspace rule forbids U+2014 in chat, code, strings, and documentation.
**Status:** Fixed in Pass A.

#### Low Documentation: Inline comment lacks terminal punctuation

**Location:** `internal/ops/submodule/git.go:38`.
**Severity:** Low.
**Current Code:** `parentRoot string // empty if none`.
**Recommendation:** End the inline comment with a period.
**Rationale:** Complete comments satisfy the repository's comment-formatting gate.
**Status:** Fixed in Pass A follow-up.

The subagent also flagged direct `fmt.Printf` calls in `sa` and `satools`. These are intentional interactive CLI output paths, including progress, dry-run, and result views, rather than service log paths. Recorded as a non-issue.

### Stage 8: Generation Gates

#### Medium Outbound Resilience: External process execution has no bounded policy

**Location:** `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`, and `internal/ops/sa/sa.go:132`.
**Severity:** Medium.
**Current Code:**

```go
cmd := exec.Command(name, args...)
err := cmd.Run()
```

The same direct execution pattern is used for Docker, Git, and static-analysis scripts.
**Recommendation:** Route process execution through a shared context-aware seam that requires a caller deadline, honors cancellation, retries only classified transient failures with exponential backoff and jitter, and uses a circuit breaker for shared network-backed operations.
**Rationale:** Unbounded process calls can hang or create repeated failure storms, and the current call sites cannot stop when their job budget expires.
**Status:** Fixed in Pass A.

#### Medium CLI Surface: Bare invocation lacks the agent operating guide

**Location:** `internal/ops/cli/root.go:47-52`.
**Severity:** Medium.
**Current Code:** The root `RunE` calls `cmd.Help()` and returns `errSubcommandRequired` for no arguments.
**Recommendation:** Make bare invocation print a structured guide with identity, role and boundaries, lifecycle commands, safe mutation rules, `AGENTS.md`, and `ai-copilots/` paths, then exit successfully. Keep normal command help under `help` and `--help`.
**Rationale:** Agent callers need a safe operating contract without parsing a failure path, and the command surface rule requires no-argument discovery to be successful.
**Status:** Fixed in Pass A.

## Open Questions

### Open Architecture: CLI root ownership

**Location:** `internal/ops/cli/root.go:39-888`.
**Observation:** The root command wires the full command surface and also owns OTEL configuration resolution, the agent guide template, and shared process context setup.
**Question:** Is keeping these cross-cutting concerns in the 888-line Cobra root intentional, or should support concerns move behind smaller helpers or packages while the root remains command wiring?
**Possible outcomes:**

- If intentional: record as a non-issue with the boundary rationale.
- If not intentional: record an architecture finding and wait for explicit fix instructions.

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| Docker tag errors were discarded | Medium | `internal/ops/satools/satools.go` | Checked tag execution and report failure. |
| Root help errors were discarded | Medium | `internal/ops/cli/root.go` | Bare invocation now writes the agent guide and returns writer errors. |
| CLI output errors were discarded | Low | `internal/ops/cli/root.go` | Version and poll cursor output now return write errors. |
| Static-analysis failures were ignored | Medium | `internal/ops/sa/sa.go` | Runs remaining tools and returns joined failures. |
| Script search error was discarded | Low | `internal/ops/sa/sa.go` | Returns the working-directory error. |
| OTEL config failures silently fell back | Medium | `internal/ops/cli/root.go` | Returns configuration load errors. |
| Test fixture setup errors were discarded | Low | `internal/ops/*/*_test.go` | Test setup now fails explicitly. |
| Em dash punctuation violated the workspace rule | Low | `internal/ops/*.go` | Replaced with colon punctuation. |
| Process execution lacked bounded resilience | Medium | `internal/ops/command/command.go` and callers | Added deadline-aware shared runner with classified failsafe retry and circuit breaker. |
| Bare CLI invocation lacked an agent guide | Medium | `internal/ops/cli/root.go` | Added structured guide and successful no-argument behavior. |
| Git discovery cause was dropped | Medium | `internal/ops/submodule/git.go` | Preserved the runner error with `%w` and added an `errors.Is` test. |
| Direct CLI output was flagged as logging | Medium | `internal/ops/sa`, `internal/ops/satools` | Non-issue: output is intentional interactive operator UI. |

## Pass A Delivery

- Commit: `b15ff34 fix mechanical staged review findings`.
- Follow-up commit: `f390dc1 fix follow-up staged review findings`.
- Draft PR: [#82](https://github.com/behaviorengineering/majordomo/pull/82).
