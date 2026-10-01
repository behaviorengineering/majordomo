# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Baseline:** `origin/main` and current HEAD are identical, so the target is the module's main command and internal packages.
**Pass A status:** Mechanical fixes applied and verified.
**Verification:** `go test ./...`, `go vet ./cmd/... ./internal/...`,
`gofmt -l cmd internal`, `git diff --check`, and
`golangci-lint run ./cmd/... ./internal/...` all passed.

## Stages
- [x] 1. Automated Tools — 10/10
- [x] 2. Type Safety — 10/10
- [x] 3. Error Handling — 7/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity — 8/10
- [x] 8. Generation Gates — 6/10

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: golangci-lint unavailable
**Location:** Review environment, lint command.
**Severity:** Medium

**Current Code:**
```text
golangci-lint run ./cmd/... ./internal/ -> command not found
```

**Recommendation:** Install or provide the repository's configured `golangci-lint` binary, then rerun the lint gate.

**Rationale:** The lint/security portion of the automated review remains unverified while the required tool is unavailable.
**Status:** fixed in Pass A

Stage 1 results:
- `go vet ./cmd/... ./internal/...`: passed.
- `gofmt -l cmd internal`: passed.
- `golangci-lint run ./cmd/... ./internal/...`: passed with 0 issues after installing the current binary with Go 1.27.

### Stage 2: Type Safety

No findings. The target contains no unchecked type assertions, bare `any` or
`interface{}` uses where a concrete type is expected, or unguarded pointer and
map dereferences identified by this stage.

### Stage 3: Error Handling

#### Medium Persistence/Process: Docker tag failures are discarded
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

**Recommendation:** Check the tag command error and report the build as failed when tagging fails.

**Rationale:** The command can report a successful image build even though the image was not tagged for later use.
**Status:** fixed in Pass A

#### Medium Error Boundary: Help output failure is discarded
**Location:** `internal/ops/cli/root.go:50`
**Severity:** Medium

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:** Return a wrapped help-output error when `cmd.Help()` fails, while preserving the missing-subcommand error when help succeeds.

**Rationale:** A broken output writer or help renderer is hidden, so the CLI can report an unrelated error and lose the actual cause.
**Status:** fixed in Pass A

#### Low Output: CLI write errors are discarded
**Location:** `internal/ops/cli/root.go:113, 502`
**Severity:** Low

**Current Code:**
```go
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:** Check and return output errors from the version and `cache poll-get` commands.

**Rationale:** Broken pipes and unwritable output destinations should produce a clear command failure.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis failures are logged but not returned
**Location:** `internal/ops/sa/sa.go:108-112`
**Severity:** Medium

**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Continue running the remaining tools, collect failed tool names, and return an error after all tools finish.

**Rationale:** The caller currently receives success even when one or more configured static-analysis tools fail.
**Status:** fixed in Pass A

#### Low Test/Error Handling: Test setup and script discovery errors were discarded
**Location:** `internal/ops/sa/sa_test.go`; `internal/ops/satools/satools_test.go`; `internal/ops/submodule/submodule_test.go`; `internal/ops/sa/sa.go:198`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
wd, _ := os.Getwd()
```

**Recommendation:** Fail tests on setup errors and return the current-working-directory error from script discovery.

**Rationale:** Ignored setup failures can invalidate test assumptions, and discovery should not continue from an unknown directory.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Low Documentation: Comments do not end with periods
**Location:** `internal/ops/satools/satools.go:20,120`; `internal/ops/sa/sa.go:29`; `internal/ops/submodule/git.go:27`
**Severity:** Low

**Current Code:**
```go
// Empty → discover from cwd.
// Vendored as .majordomo under a parent workspace.
// GitRunner overrides git execution (tests). nil → real git.
```

**Recommendation:** Use complete sentences with periods and replace arrow shorthand with plain English.

**Rationale:** Consistent comments are easier to scan and satisfy the repository's documentation lint convention.
**Status:** fixed in Pass A

#### Medium Logging: Static-analysis service uses ad-hoc output
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium

**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:** Inject the project's structured logger or a testable output interface, and keep formatting at the CLI boundary.

**Rationale:** Direct process output bypasses structured fields and makes service behavior harder to test and operate consistently.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium C22: Process execution bypasses shared resilience
**Location:** `internal/ops/sa/sa.go:132`; `internal/ops/satools/satools.go:230`; `internal/ops/submodule/git.go:222`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
cmd := exec.Command(name, args...)
cmd := exec.Command("git", args...)
```

**Recommendation:** Route outbound process execution through one context-aware `failsafe-go` seam with exponential backoff, jitter, circuit breaking for shared network-backed commands, and retry classification.

**Rationale:** Git, Docker, and configured analysis scripts can fail transiently or hang without caller cancellation, causing avoidable job failures and resource loss.
**Status:** fixed in Pass A

#### Medium C23: CLI maps a leaf package sentinel directly
**Location:** `cmd/majordomo/main.go:11, 29`
**Severity:** Medium

**Current Code:**
```go
if errors.Is(err, staging.ErrNothingToReview) {
    os.Exit(2)
}
```

**Recommendation:** Expose a CLI-owned `IsNothingToReview` helper or wrapped sentinel and have `main` depend only on the CLI package.

**Rationale:** The process entrypoint should map errors from its immediate command/service boundary, not import a deeper review package solely for status mapping.
**Status:** fixed in Pass A

#### Medium CLI Surface: Bare and unknown invocations do not provide the required operator view
**Location:** `internal/ops/cli/root.go:39-54`
**Severity:** Medium

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
SilenceUsage:  true,
```

**Recommendation:** Add the structured agent operating guide for bare invocation, return success for that guide, and preserve usage output for unknown commands.

**Rationale:** Agents and operators need a safe command catalog without triggering work, while unknown input must not fail with an error-only response.
**Status:** fixed in Pass A

## Open Questions

None recorded yet.

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| golangci-lint unavailable | Medium | Review environment | Resolved in Pass A, current lint run passed with 0 issues. |
| Docker tag failures discarded | Medium | `internal/ops/satools/satools.go` | Fixed in Pass A and covered by return handling. |
| Help output failure discarded | Medium | `internal/ops/cli/root.go` | Fixed in Pass A by the agent guide path. |
| CLI write errors discarded | Low | `internal/ops/cli/root.go` | Fixed in Pass A with checked writes. |
| Static-analysis failures not returned | Medium | `internal/ops/sa/sa.go` | Fixed in Pass A, with failure aggregation and test coverage. |
| Discarded test and discovery errors | Low | `internal/ops/sa`, `internal/ops/satools`, `internal/ops/submodule` | Fixed in Pass A. |
| Comment punctuation | Low | `internal/ops/{sa,satools,submodule}` | Fixed in Pass A. |
| Ad-hoc static-analysis logging | Medium | `internal/ops/sa/sa.go` | Fixed in Pass A with an injectable writer and checked writes. |
| Bare process execution | Medium | `internal/ops/{command,sa,satools,submodule}` | Fixed in Pass A through the context-aware failsafe seam. |
| Leaf sentinel mapped in main | Medium | `cmd/majordomo/main.go` | Fixed in Pass A through `cli.IsNothingToReview`. |
| Bare and unknown CLI surface | Medium | `internal/ops/cli/root.go` | Fixed in Pass A with an agent guide and usage output. |

## Canvas Snapshot

### Progress
- Stage 1: done, 10/10.
- Stage 2: done, 10/10.
- Stage 3: done, 7/10, findings fixed.
- Stage 4: skipped until Pass A is committed and pushed.
- Stage 5: skipped until Pass A is committed and pushed.
- Stage 6: skipped until Pass A is committed and pushed.
- Stage 7: done, 8/10, findings fixed.
- Stage 8: done, 6/10, findings fixed.
- Pass A: mechanical fixes applied and verified.
- Counts: 0 critical, 8 medium, 3 low, 0 open.

### Now
Pass A is verified. The next step is the separate Pass A commit and push.

### Resolutions
All code findings are fixed in Pass A, and all selected mechanical gates pass.
