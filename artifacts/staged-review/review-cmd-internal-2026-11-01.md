# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...` (no diff versus `origin/main`)
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Hooks:** none discovered (`ai-copilots/review-hooks.yaml` absent).
**Branch:** `cursor/staged-review-process-ddc6`

## Stages
- [x] 1. Automated Tools — Score: 6/10
- [x] 2. Type Safety — Score: 10/10
- [x] 3. Error Handling — Score: 5/10
- [ ] 4. Architecture (consultant)
- [ ] 5. Robustness (consultant)
- [ ] 6. Testability (consultant)
- [x] 7. Code Clarity — Score: 9/10
- [x] 8. Generation Gates — Score: 4/10

## Pass A Status
- Mechanical findings: 0 recorded before Stage 1.
- Fix status: all actionable mechanical findings applied.
- Pack mount commit: `0f17c12`.
- Pass A fix commit: `49bdcad`, pushed to `cursor/staged-review-process-ddc6`.
- Pass A follow-up commit: `b5ecffa`, pushed to `cursor/staged-review-process-ddc6`.
- Draft PR: https://github.com/behaviorengineering/majordomo/pull/110
- Focused tests: `go test ./internal/ops/process ./internal/ops/sa ./internal/ops/satools ./internal/ops/submodule` passed.
- Follow-up verification: focused tests and vet passed; CLI tests passed with the same temporary checksum diagnostic.
- CLI tests: passed with a temporary checksum diagnostic that did not replace the committed public checksum.
- Full `go tool task vet`, `lint`, and `test`: blocked by the `operatorconfig@v0.1.1` checksum mismatch recorded below.

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: Dependency checksum blocks vet
**Location:** `go.sum` / `github.com/behaviorengineering/operatorconfig@v0.1.1`
**Severity:** Medium

**Current Code:**
`go tool task vet` fails before analysis because the downloaded module hash differs from the committed `go.sum` hash. `go tool task lint` fails from the same integrity problem and then reports no Go files to analyze.

**Recommendation:**
Verify the published module contents and trusted checksum before changing `go.sum`; then rerun the operator gates.

**Rationale:** The quality gates cannot establish correctness while dependency integrity verification fails. Blindly replacing the checksum could accept a republished or compromised dependency.
**Status:** open (environment/dependency blocker; not safely auto-fixed)

#### Low Tooling: Quality gate status unavailable
**Location:** `Taskfile.yml:13-29`
**Severity:** Low

**Current Code:**
The project exposes the required `format`, `lint`, and `vet` tasks, but vet and lint did not complete because dependency verification stopped package loading.

**Recommendation:**
Rerun both tasks after resolving the checksum mismatch and retain their exit codes in this plan.

**Rationale:** A clean task definition is not evidence that the underlying Go packages pass analysis.
**Status:** open (blocked by dependency checksum failure)

### Stage 2: Type Safety

No findings. The target contains no unchecked type assertions, bare `interface{}` values, or unsafe map/pointer dereferences identified by the mechanical scan. The variadic `any` parameters are appropriate for formatting helpers.

### Stage 3: Error Handling

#### Medium Error Handling: CLI output errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`, `internal/ops/cli/root.go:54,119,513`
**Severity:** Medium

**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_ = cmd.Help()
```

**Recommendation:**
Use `RunE` for output-producing commands, return write failures, and return help failures before returning the root command error.

**Rationale:** Broken pipes and failed report writes currently look like successful commands, so operators and CI can miss incomplete output.
**Status:** fixed in Pass A

#### Medium Error Handling: Invalid OTEL configuration is logged and ignored
**Location:** `internal/ops/cli/root.go:96-103`
**Severity:** Medium

**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
} else {
    // Continue with zero settings.
}
```

**Recommendation:**
Return the configuration error from `resolveOTELConfig` and stop before initializing observability with an implicit fallback.

**Rationale:** A malformed or inaccessible configured file silently changes runtime behavior and hides an operator configuration failure.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failure is discarded
**Location:** `internal/ops/satools/satools.go:217-223`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```

**Recommendation:**
Check the tag command error, include its output, and report the build as failed when tagging fails.

**Rationale:** The command can claim an image build passed even though the image name used by later steps was never created.
**Status:** fixed in Pass A

#### Low Error Handling: Probe and working-directory errors are silently dropped
**Location:** `internal/ops/sa/sa.go:150`, `internal/ops/submodule/git.go:96-100,129,146,189`
**Severity:** Low

**Current Code:**
```go
wd, _ := os.Getwd()
indexEntry, _ := m.git([]string{"ls-files", "--stage", rel}, parent, false)
out, _ := m.git([]string{"status", "--porcelain"}, repoRoot, false)
```

**Recommendation:**
Handle `os.Getwd` errors and make probe failures explicit, failing closed for dirty-state checks.

**Rationale:** Ignoring these errors turns an unavailable repository state into an empty or clean-looking state.
**Status:** fixed in Pass A

#### Low Error Handling: Test fixture setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-43`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```

**Recommendation:**
Fail the test immediately when fixture creation fails.

**Rationale:** A fixture can be incomplete while the test continues and fails later with a misleading product assertion.
**Status:** fixed in Pass A

#### High Error Handling: Git probe failures are collapsed into no-op results
**Location:** `internal/ops/submodule/git.go:218-234`, `internal/ops/submodule/commands.go:276-318`
**Severity:** High

**Current Code:**
```go
if !check {
    return out, nil
}
```

**Recommendation:**
Preserve Git errors and handle expected missing references explicitly at the probe call sites.

**Rationale:** Failed commits, remote lookups, or cleanup commands can be misreported as missing branches or successful no-ops.
**Status:** fixed in Pass A follow-up

### Stage 7: Code Clarity

#### Low Clarity: Inline field comment is missing terminal punctuation
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low

**Current Code:**
```go
parentRoot string // empty if none
```

**Recommendation:**
End the comment with a period and state the condition directly.

**Rationale:** Consistent comments satisfy the configured `godot` gate and make the field contract clearer.
**Status:** fixed in Pass A

#### Medium Clarity: SA tool progress bypasses injected output
**Location:** `internal/ops/satools/satools.go:56-131,244-260`
**Severity:** Medium

**Current Code:**
```go
fmt.Printf("Building %s (%s) ...\n", tag, mode)
```

**Recommendation:**
Inject the operator output writer and return write failures from progress and result rendering.

**Rationale:** Direct process-global output is difficult to test and can bypass Cobra output redirection.
**Status:** fixed in Pass A follow-up

### Stage 8: Generation Gates

#### High CLI Surface: Bare invocation is not an agent-ready success path
**Location:** `internal/ops/cli/root.go:48-56`, `cmd/majordomo/main.go:27-34`
**Severity:** High

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
}
```

**Recommendation:**
Make the no-argument root command print a structured agent operating guide to stdout and return success; keep unknown commands non-zero.

**Rationale:** Agents invoking the binary without arguments receive only help plus a failure exit code, which prevents safe discovery and violates the CLI command-surface contract.
**Status:** fixed in Pass A

#### High Outbound Resilience: External process execution has no context, deadline, retry, or breaker
**Location:** `internal/ops/sa/sa.go:132-138`, `internal/ops/satools/satools.go:226-237`, `internal/ops/submodule/git.go:218-235`
**Severity:** High

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
err := cmd.Run()
```

**Recommendation:**
Route external execution through one context-aware seam using failsafe-go retry and circuit-breaker policies, classify retryable failures, and reject calls without a caller deadline.

**Rationale:** Hung or overloaded scripts, Docker, and remote Git operations can block indefinitely or create retry storms without a bounded resilient execution policy.
**Status:** fixed in Pass A

#### Medium Observability: Static-analysis service uses unstructured wall-clock printing
**Location:** `internal/ops/sa/sa.go:33-35`
**Severity:** Medium

**Current Code:**
```go
func logf(level, format string, args ...any) {
    ts := time.Now().UTC().Format("2006-01-02 15:04:05")
    fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:**
Inject a structured logger into `sa.Options` and emit fields such as repository, tool, and file counts through it.

**Rationale:** Raw stdout logs are hard to filter and bypass the process observability pipeline.
**Status:** fixed in Pass A

#### Low Report Construction: Multi-line SA tool summary is encoded in format strings
**Location:** `internal/ops/satools/satools.go:56-58`
**Severity:** Low

**Current Code:**
```go
fmt.Printf("SA Tool Image Builder\nMode: ...\nContext: ...\nTools: ...\n", ...)
```

**Recommendation:**
Render the operator report with a small `text/template` and an explicit report data struct.

**Rationale:** A template keeps multi-line report shape reviewable and prevents formatting drift as fields change.
**Status:** fixed in Pass A

#### Low Operator Docs: Task summaries are absent
**Location:** `Taskfile.yml:6-37`
**Severity:** Low

**Current Code:**
The listed tasks have one-line `desc` values but no `summary` guidance.

**Recommendation:**
Add concise summaries for quality tasks, including scope and any `--` argument behavior.

**Rationale:** Agents and operators cannot inspect task intent beyond short labels, and the quality skill recommends summaries for listed tasks.
**Status:** fixed in Pass A

## Open Questions
- Dependency integrity remains unresolved: the module proxy/downloaded `operatorconfig@v0.1.1` hash differs from the public checksum database and committed `go.sum`. No checksum was replaced.
- Stage 4 Architecture: `internal/ops/cli/root.go` owns the Cobra root and all command constructors in one large file. Is that centralized command wiring intentional, or should command constructors move into domain-focused files?

## Resolutions
| Finding | Severity | Location | Outcome |
|---|---|---|---|
| CLI output errors | Medium | `internal/ops/cli` | Fixed in Pass A; output writes now return errors. |
| OTEL config fallback | Medium | `internal/ops/cli/root.go` | Fixed in Pass A; configuration errors now stop orchestration. |
| Docker tag failure | Medium | `internal/ops/satools/satools.go` | Fixed in Pass A; tag errors fail the build result. |
| Repository probe errors | Low | `internal/ops/sa`, `internal/ops/submodule` | Fixed in Pass A; probes are explicit and dirty checks fail closed. |
| Test fixture setup | Low | `internal/ops/*/*_test.go` | Fixed in Pass A; setup errors fail tests immediately. |
| Comment punctuation | Low | `internal/ops/submodule/git.go` | Fixed in Pass A. |
| Bare CLI invocation | High | `internal/ops/cli/root.go` | Fixed in Pass A; bare invoke prints an agent guide and exits successfully. |
| External process resilience | High | `internal/ops/process` | Fixed in Pass A; context deadlines, retries, and circuit breakers are centralized. |
| Static-analysis logging | Medium | `internal/ops/sa/sa.go` | Fixed in Pass A; injected structured logging is used. |
| SA summary formatting | Low | `internal/ops/satools/satools.go` | Fixed in Pass A with `text/template`. |
| Task summaries | Low | `Taskfile.yml` | Fixed in Pass A. |
| Dependency checksum | Medium | `go.sum` | Still open; retained the public checksum and did not accept the mismatched download. |
| Git probe failures | High | `internal/ops/submodule` | Fixed in Pass A follow-up; expected no-ops are handled at call sites and cleanup errors propagate. |
| SA tool output | Medium | `internal/ops/satools` | Fixed in Pass A follow-up; output is injected and write errors return. |
