# Staged Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...`
**Trigger:** Scheduled review; no Go diff against `origin/main`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8; Pass B: 4, 5, 6
**Hooks:** none discovered
**Pass A Commit:** `b19f91d`
**Pass A Branch:** `cursor/staged-review-process-f0ec`
**Draft PR:** https://github.com/behaviorengineering/majordomo/pull/115

## Stages
- [x] 1. Automated Tools
- [x] 2. Type Safety
- [x] 3. Error Handling
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity
- [x] 8. Generation Gates

## Passes
- Pass A: detect and fix mechanical findings from stages 1, 2, 3, 7, and 8.
- Pass B: ask consultant questions for stages 4, 5, and 6 after Pass A is committed and pushed.

## Findings

### Stage 1: Automated Tools

**Score:** 10/10 after Pass A fixes

#### High Tooling: Dependency checksum verification blocks quality gates
**Location:** `go.sum:240`
**Severity:** High

**Current Code:**
```text
github.com/behaviorengineering/operatorconfig v0.1.1 h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
```

**Recommendation:** Verify the upstream module contents and reconcile the dependency
or checksum through a trusted release process before accepting a new module hash.
Do not overwrite `go.sum` blindly.

**Rationale:** `go tool task vet`, `go tool task lint`, and `go tool task test`
failed closed because the downloaded module hash differed from the recorded sum.
This is a supply-chain verification failure, not a lint finding.
**Status:** fixed in Pass A

#### Low Formatting: No formatting findings
**Location:** `./cmd/...` and `./internal/...`
**Severity:** Low

**Current Code:**
```text
go tool gofumpt -l ./cmd ./internal
```

**Recommendation:** None.

**Rationale:** The format check completed successfully.
**Status:** non-issue

### Stage 2: Type Safety

No findings. The reviewed command and internal packages do not use unchecked
type assertions, empty-interface values where a concrete type is required, or
unguarded pointer dereferences in the inspected paths.

**Score:** 10/10

### Stage 3: Error Handling

**Score:** 10/10 after Pass A fixes

#### Medium Error Handling: OTEL config errors are logged and ignored
**Location:** `internal/ops/cli/root.go:98-105`
**Severity:** Medium

**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
} else {
    // Continue with environment-only settings.
}
```

**Recommendation:** Return the configuration error to the command boundary
instead of silently changing the observability configuration.

**Rationale:** A malformed or unreadable central config can make the run
appear successful while disabling the configured telemetry.
**Status:** fixed in Pass A

#### Medium Persistence: A failed Docker tag is ignored
**Location:** `internal/ops/satools/satools.go:220-222`
**Severity:** Medium

**Current Code:**
```go
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
```

**Recommendation:** Return a failed build result and retain the tag error in
the captured output when tagging fails.

**Rationale:** The command currently reports a successful image build even
when the image required by later local steps was not tagged.
**Status:** fixed in Pass A

#### Low Error Handling: Probe and output errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`,
`internal/ops/cli/root.go:54,119,513`, `internal/ops/sa/sa.go:150`,
`internal/ops/submodule/git.go:96-147`, and test setup helpers
**Severity:** Low

**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
wd, _ := os.Getwd()
out, _ := m.git(args, cwd, false)
```

**Recommendation:** Return output and required filesystem errors, and make
best-effort probes explicitly branch on errors so the fallback behavior is
visible and fail-safe.

**Rationale:** Discarded errors hide broken output streams, working-directory
lookup failures, and repository probes. The test helpers also hide setup
failures that can make tests pass without creating their fixtures.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

**Score:** 10/10

#### Low Clarity: SA status logging bypasses an output seam
**Location:** `internal/ops/sa/sa.go:33-36`
**Severity:** Low

**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:** Route command output through an injected writer or logger
owned by `Options`, while keeping the CLI entrypoint responsible for stdout.

**Rationale:** A package-global stdout logger is difficult to test and makes
the command's output destination implicit.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

**Score:** 10/10

#### High C22: Process-exec paths lack shared outbound resilience
**Location:** `internal/ops/sa/sa.go:132-137`,
`internal/ops/satools/satools.go:230-237`, and
`internal/ops/submodule/git.go:222-237`
**Severity:** High

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
if err := cmd.Run(); err != nil {
    return fmt.Errorf("run-sa-tool.sh %s: %w", slug, err)
}
```

**Recommendation:** Route network-capable process execution through one
context-aware resilience seam with classified retry, backoff, and circuit
breaking, and fail closed when the caller supplies no deadline.

**Rationale:** Git, Docker, and static-analysis wrapper processes can fail
transiently or hang without a shared policy. The current calls have no context,
deadline enforcement, retry classification, or breaker.
**Status:** fixed in Pass A

#### Low C11: Quality gate results are incomplete because dependency verification fails
**Location:** `go.sum:240`
**Severity:** Low

**Current Code:**
```text
operatorconfig v0.1.1 has a recorded checksum that differs from the downloaded module.
```

**Recommendation:** Resolve the Stage 1 supply-chain verification failure,
then rerun vet, lint, and tests.

**Rationale:** The repository has the required Taskfile, tool pins, and
golangci-lint v2 configuration, but the gates cannot complete safely.
**Status:** fixed in Pass A

## Open Questions

## Pass A Verification
- `go tool task vet`: passed.
- `go tool task lint`: passed with 0 issues.
- `go tool task test`: passed for all packages.
- `go tool gofumpt -l ./cmd ./internal`: passed.
- `git diff --check`: passed.
- Pass A commit `b19f91d` was pushed before consultant review.

## Resolutions
- Stage 1 checksum: finding → fixed in Pass A after validating the public
  checksum database and updating the recorded module hash.
- Stage 2 type safety: non-issue.
- Stage 3 error handling: findings → fixed in Pass A.
- Stage 7 code clarity: finding → fixed in Pass A by injecting command output.
- Stage 8 generation gates: findings → fixed in Pass A with a shared bounded
  process runner and complete dependency checks.

