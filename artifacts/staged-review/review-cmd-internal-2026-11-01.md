# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...` (no Go diff versus `origin/main`; cron review)
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Stage mapping:** User-selected 7 = current Code Clarity stage 4; user-selected 8 = current Generation Gates stage 5. User-selected 4, 5, 6 = current consultant stages A, B, C.
**Hooks:** none discovered in `ai-copilots/review-hooks.yaml`
**Baseline:** Pack mount committed separately as `0796e60`.
**Pass A Fix Commit:** `9e72086`
**Pass A Status:** complete; ready to push before consultant dialogue.

## Stages
- [x] 1. Automated Tools — 4/10
- [x] 2. Type Safety — 10/10
- [x] 3. Error Handling — 6/10
- [ ] 4. Architecture (consultant)
- [ ] 5. Robustness (consultant)
- [ ] 6. Testability (consultant)
- [x] 7. Code Clarity — 8/10
- [x] 8. Generation Gates — 5/10

## Findings

### Stage 1: Automated Tools

#### High Tooling: Dependency checksum mismatch blocks quality gates
**Location:** `go.sum:240`; `Taskfile.yml:23-27`
**Severity:** High

**Current Code:**
```text
github.com/behaviorengineering/operatorconfig v0.1.1 h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
vet: downloaded h1:nVrLCDsziZyRxFB6sH6I1DHdsQ9dgoO9HPU1+ZDMH8k=
```

**Recommendation:** Verify the tagged module contents and update the dependency pin and `go.sum` together only after confirming the intended upstream release; otherwise restore a reproducible tag or replace the dependency with a verified version.

**Rationale:** The Go toolchain refuses to compile with a checksum mismatch, and lint then fails before analyzing packages. Blindly replacing the checksum would weaken dependency integrity.
**Status:** fixed in Pass A

#### Low Tooling: Lint result is unavailable after dependency failure
**Location:** `Taskfile.yml:23-27`
**Severity:** Low

**Current Code:**
```yaml
lint:
  cmds:
    - go tool golangci-lint run --timeout 5m ./cmd/... ./internal/... ./pkg/...
```

**Recommendation:** Re-run the pinned lint command after the checksum issue is resolved and retain its package diagnostics in the review record.

**Rationale:** The runner is correctly pinned, but the current result is a dependency-loading failure rather than a code analysis result.
**Status:** fixed in Pass A

### Stage 2: Type Safety

No findings. The target uses Cobra and concrete option structs, has no unchecked type assertions, and has no dereference of an unguarded pointer or map lookup in the inspected paths.

### Stage 3: Error Handling

#### Medium Error Handling: Operator output write errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`; `internal/ops/cli/root.go:54,119,513`
**Severity:** Medium

**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
```

**Recommendation:** Return or wrap output and help errors from the Cobra handlers instead of discarding them.

**Rationale:** A broken pipe or unavailable output stream is an observable command failure. Ignoring it lets the CLI report success or mask the real failure.
**Status:** fixed in Pass A

#### Medium Error Handling: OTEL config errors are logged and replaced with a fallback
**Location:** `internal/ops/cli/root.go:86-111`
**Severity:** Medium

**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
}
return observability.ResolveConfig(outputDir, settings)
```

**Recommendation:** Return the configuration error to the command path, or make the fallback an explicit, documented non-error case that does not hide malformed configuration.

**Rationale:** A malformed central configuration silently changes tracing behavior and makes configuration failures difficult to diagnose.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis tool failures are silently continued
**Location:** `internal/ops/sa/sa.go:108-112`
**Severity:** Medium

**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Collect tool failures and return an error after all configured tools finish, or fail immediately with the tool identity.

**Rationale:** The `sa` command exits successfully even when a configured analysis tool fails, so CI can accept an incomplete analysis.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failures are discarded
**Location:** `internal/ops/satools/satools.go:217-223`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
    full := "local/sa-" + tool + ":local-test"
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
```

**Recommendation:** Return the tag error through `runBuild` and mark the build failed when tagging fails.

**Rationale:** The command can report a successful image build even though the image required by later steps was not tagged.
**Status:** fixed in Pass A

#### Low Error Handling: Git cleanup failure is discarded
**Location:** `internal/ops/submodule/commands.go:293-297`
**Severity:** Low

**Current Code:**
```go
defer func() {
    m.printf("Cleaning up worktree...\n")
    _, _ = m.git([]string{"worktree", "remove", "--force", worktreePath}, m.parentRoot, false)
}()
```

**Recommendation:** Preserve the cleanup error in the returned error or report it through the manager output with the worktree path.

**Rationale:** A failed cleanup leaves a stale worktree that can affect later operations, while the command gives no actionable signal.
**Status:** fixed in Pass A

#### Low Error Handling: Test filesystem setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`; `internal/ops/satools/satools_test.go:12-43`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(cfgDir, 0o755)
_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("{}\n"), 0o644)
```

**Recommendation:** Check setup errors with `t.Fatal` or `require.NoError`.

**Rationale:** A broken fixture can cause misleading test failures or silently skip the intended setup.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Low Clarity: Inline field comment is not a complete sentence
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low

**Current Code:**
```go
parentRoot string // empty if none
```

**Recommendation:** End the comment with a period and state the condition clearly.

**Rationale:** Consistent comments improve generated documentation and satisfy the repository's `godot` gate.
**Status:** fixed in Pass A

#### Low Clarity: Static-analysis logging bypasses structured output
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Low

**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:** Inject an output/logger seam and keep human output at the CLI boundary.

**Rationale:** The package writes process logs directly to stdout, which makes callers and tests unable to control or capture the output cleanly.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium Generation Gates: Outbound subprocesses have no shared resilience seam
**Location:** `internal/ops/sa/sa.go:132`; `internal/ops/satools/satools.go:230`; `internal/ops/submodule/git.go:222`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
cmd := exec.Command(name, args...)
cmd := exec.Command("git", args...)
```

**Recommendation:** Route network-backed subprocesses through a context-aware shared execution seam with classified retry and circuit-breaker policy; keep local-only commands explicitly separate.

**Rationale:** Direct process execution has no caller deadline or consistent retry classification, so remote-backed operations can hang or create failure storms.
**Status:** fixed in Pass A

#### Medium Generation Gates: Direct dispatch can reach inference without a required deadline
**Location:** `pkg/review/dispatch/dispatch.go:100-119`; invoked by `internal/ops/cli/root.go:206-232`
**Severity:** Medium

**Current Code:**
```go
ctx := opts.Context
if ctx == nil {
    return fmt.Errorf("dispatch: context is required")
}
return judge.Dispatch(judge.DispatchOptions{Context: ctx, ...})
```

**Recommendation:** Require a caller-supplied deadline before dispatch, with the process entrypoint responsible for setting the job budget.

**Rationale:** A non-nil context without a deadline does not bound an LLM or gateway operation.
**Status:** fixed in Pass A

#### Low Generation Gates: Multi-line operator reports are assembled with formatting primitives
**Location:** `internal/ops/satools/satools.go:56-98`; `internal/ops/submodule/menu.go:8-20`
**Severity:** Low

**Current Code:**
```go
fmt.Printf("SA Tool Image Builder\nMode: ...\n\n", ...)
return header + "\n\n" + strings.Join(items, "\n")
```

**Recommendation:** Use `text/template` for multi-line operator reports and keep data separate from layout.

**Rationale:** Template-backed reports are easier to review and change without introducing spacing or field-order defects.
**Status:** fixed in Pass A

#### Low Generation Gates: Task summaries are not documented
**Location:** `Taskfile.yml:6-42`
**Severity:** Low

**Current Code:**
```yaml
tasks:
  default:
    desc: List local-dev tasks
```

**Recommendation:** Add concise `summary` text for listed operator tasks, especially the quality gates and build task.

**Rationale:** Operators can see task names but not scope, argument behavior, or platform limits from the summary view.
**Status:** fixed in Pass A

#### Medium Generation Gates: CLI and SA paths use direct stdout/stderr instead of injected structured output
**Location:** `internal/ops/cli/root.go:94-99`; `internal/ops/sa/sa.go:33-35`; `internal/ops/satools/satools.go:56-98`
**Severity:** Medium

**Current Code:**
```go
fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
fmt.Printf("[%s] [%s] %s\n", ts, level, ...)
```

**Recommendation:** Return configuration errors and inject a logger or writer at the command boundary for operator output.

**Rationale:** Direct process streams bypass the command's configured output and make structured collection and testing harder.
**Status:** fixed in Pass A

## Open Questions

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| Dependency checksum mismatch | High | `go.mod`, `go.sum` | Upgraded `operatorconfig` to verified `v0.2.0`; vet and lint now pass. |
| Lint blocked by dependency loading | Low | `Taskfile.yml` | Re-run passed with zero issues. |
| Discarded operator output errors | Medium | `internal/ops/cli/*.go` | Output and help failures now return to Cobra. |
| OTEL config fallback | Medium | `internal/ops/cli/root.go` | Config load errors now fail closed. |
| SA failures continued | Medium | `internal/ops/sa/sa.go` | Tool failures aggregate and return after the configured run. |
| Docker tag failure discarded | Medium | `internal/ops/satools/satools.go` | Tag failure marks the build failed. |
| Worktree cleanup failure discarded | Low | `internal/ops/submodule/commands.go` | Cleanup errors are reported and returned when no prior error exists. |
| Test fixture setup errors discarded | Low | `internal/ops/{sa,satools}/*_test.go` | Fixture filesystem errors now fail tests. |
| Incomplete comment | Low | `internal/ops/submodule/git.go` | Comment now states the empty-parent condition as a sentence. |
| SA direct logging | Low | `internal/ops/sa/sa.go` | Logging now uses an injected output writer. |
| Direct subprocess execution | Medium | `internal/ops/process/process.go` | Added shared context-aware failsafe retry and breaker seam for remote-backed subprocesses. |
| Dispatch without deadline | Medium | `pkg/review/dispatch/dispatch.go`, `internal/ops/cli/root.go` | Dispatch rejects missing deadlines; CLI sets an explicit job budget. |
| Formatting-based multi-line reports | Low | `internal/ops/satools/satools.go`, `internal/ops/submodule/menu.go` | Multi-line reports now use `text/template`. |
| Missing Task summaries | Low | `Taskfile.yml` | Added concise summaries for listed tasks. |
| Direct command streams | Medium | `internal/ops/cli/root.go`, `internal/ops/sa/sa.go`, `internal/ops/satools/satools.go` | Command output is returned or injected through writers. |

## Pass A Verification

- `go tool gofumpt -l ./cmd ./internal ./pkg`: passed.
- `go tool task vet`: passed.
- `go tool task lint`: passed, zero issues.
- `go tool task test`: passed.
- `go tool task build`: passed.
- Go architecture review: compliant; the new process seam is a focused shared helper and CLI RunE functions remain wiring.
- Go CLI review: compliant; Cobra owns dispatch, flags stay on subcommands, and errors return to the root.
