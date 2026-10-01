# Staged Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` (no changes versus `origin/main`)
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Baseline:** `origin/main`

## Stages
- [x] 1. Automated Tools
- [x] 2. Type Safety
- [x] 3. Error Handling
- [x] 7. Code Clarity
- [x] 8. Generation Gates
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability

## Findings

### Stage 1: Automated Tools
- `go vet ./cmd/... ./internal/...`: passed with exit code 0.
- `gofmt -l cmd internal`: passed with no output.
- No `Makefile` is present.

#### Low Tooling: golangci-lint unavailable
**Location:** Review environment.
**Severity:** Low

**Current Code:**
```text
golangci-lint: command not found
```

**Recommendation:** Install or provide the repository's configured `golangci-lint` binary before relying on the lint result.

**Rationale:** The lint gate could not run, so Stage 1 has incomplete static-analysis coverage.
**Status:** fixed in Pass A

Recheck: `golangci-lint v2.14.0 run ./cmd/... ./internal/...` passed with 0 issues after fixing the reported `errcheck` and `staticcheck` findings.

### Stage 2: Type Safety
No findings. The target contains no unsafe type assertions, bare empty interfaces, or unguarded pointer dereferences found by inspection.
**Status:** done

### Stage 3: Error Handling

#### Medium Error Handling: Ignored CLI output errors
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium

**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:** Return or propagate output errors from command handlers.

**Rationale:** A broken pipe or unavailable output stream currently reports success and hides the actual failure.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis failures are discarded
**Location:** `internal/ops/sa/sa.go:108-112`
**Severity:** Medium

**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Continue running remaining tools, but collect failures and return an aggregate error after the loop.

**Rationale:** The command currently exits successfully even when a configured tool fails, so callers and CI can accept an incomplete analysis.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failure is ignored
**Location:** `internal/ops/satools/satools.go:218-223`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
    full := "local/sa-" + tool + ":local-test"
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```

**Recommendation:** Treat a failed image tag as a failed build and include its error in the result output.

**Rationale:** The command can report a passing image build even though the requested tag was not created.
**Status:** fixed in Pass A

#### Low Test Hygiene: Test setup errors are ignored
**Location:** `internal/ops/sa/sa_test.go:12-14,31-32`; `internal/ops/satools/satools_test.go:12-14,30-32,42-43`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(cfgDir, 0o755)
_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("{}\n"), 0o644)
```

**Recommendation:** Fail tests immediately when fixture setup fails.

**Rationale:** Setup failures otherwise produce misleading assertions or later errors that obscure the cause.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Low Clarity: Inline comment is not a complete sentence
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low

**Current Code:**
```go
parentRoot    string // empty if none
```

**Recommendation:** End the comment with a period.

**Rationale:** Consistent sentence punctuation keeps generated documentation and lint output clean.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium Generation: Bare CLI invocation is not agent-ready
**Location:** `internal/ops/cli/root.go:49-52`
**Severity:** Medium

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:** Make the no-argument path print a structured operating guide and exit successfully, while keeping explicit commands for all operations.

**Rationale:** Automation invoking the binary without arguments receives an error instead of discoverable commands, lifecycle guidance, and safe mutation rules.
**Status:** fixed in Pass A

#### Medium Generation: Process execution lacks a shared bounded runner
**Location:** `internal/ops/sa/sa.go:132`; `internal/ops/satools/satools.go:230`; `internal/ops/submodule/git.go:222`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
err := cmd.Run()
```

**Recommendation:** Route process execution through one context-aware runner with caller deadlines and the repository's outbound retry policy for network-backed commands.

**Rationale:** Git, Docker, and analysis scripts can hang or fail transiently without a shared cancellation, deadline, or retry classification.
**Status:** fixed in Pass A

#### Medium Generation: Non-interactive command output bypasses structured logging
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium

**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:** Inject the repository's structured logger or a narrow logging interface into the command options.

**Rationale:** Raw printing loses structured fields and makes command output harder to collect and test consistently.
**Status:** fixed in Pass A

## Open Questions
None from Pass A.

## Resolutions
- Tooling: non-issue after installing the latest Go 1.27-compatible linter and passing the gate.
- Ignored command output: fixed in Pass A by returning writer errors.
- Static-analysis failures: fixed in Pass A by returning an aggregate error after all tools run.
- Docker tag failures: fixed in Pass A by failing the build result when tagging fails.
- Test fixture setup: fixed in Pass A by checking filesystem errors.
- Comment punctuation: fixed in Pass A.
- Bare CLI invocation: fixed in Pass A with a structured agent guide and successful exit.
- External process execution: fixed in Pass A with the shared deadline-aware failsafe runner.
- Command logging: fixed in Pass A with injected `slog.Logger`.

