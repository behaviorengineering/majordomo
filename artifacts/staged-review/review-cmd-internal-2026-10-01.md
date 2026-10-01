# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the branch has no diff from `origin/main`.
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.

## Progress

Pass A fixes are complete. Verification passed for the full Go tree.

| Stage | Status | Score | Critical | Medium | Low | Open |
|---|---|---:|---:|---:|---:|---:|
| 1. Automated Tools | done | 7/10 | 0 | 0 | 1 | 1 |
| 2. Type Safety | done | 10/10 | 0 | 0 | 0 | 0 |
| 3. Error Handling | done | 10/10 | 0 | 0 | 0 | 0 |
| 4. Architecture | in progress | — | 0 | 0 | 0 | 1 |
| 5. Robustness | pending | — | 0 | 0 | 0 | 0 |
| 6. Testability | pending | — | 0 | 0 | 0 | 0 |
| 7. Code Clarity | done | 10/10 | 0 | 0 | 0 | 0 |
| 8. Generation Gates | done | 10/10 | 0 | 0 | 0 | 0 |

Pass A fix commit: `c82a78e`.
Draft PR/MR: [#70](https://github.com/behaviorengineering/majordomo/pull/70).

## Stages

- [x] 1. Automated Tools, Pass A.
- [x] 2. Type Safety, Pass A.
- [x] 3. Error Handling, Pass A.
- [ ] 4. Architecture, Pass B.
- [ ] 5. Robustness, Pass B.
- [ ] 6. Testability, Pass B.
- [x] 7. Code Clarity, Pass A.
- [x] 8. Generation Gates, Pass A.

## Now

Stage 4, Architecture. `internal/ops/cli/root.go:1-900` imports many
application packages as the command composition boundary.

Question: Is this broad import surface intentional for the CLI wiring package,
or should command construction be split into smaller command groups?

Why this matters: A composition root may legitimately know many services, but
an oversized root becomes harder to navigate and test. Splitting by command
group can reduce coupling, while an unnecessary split can hide the CLI's
composition boundary.

## Findings

### Stage 1: Automated Tools

#### Low Tooling: `golangci-lint` is unavailable
**Location:** repository toolchain
**Severity:** Low
**Current Code:**
```text
golangci-lint: command not found
```
**Recommendation:** Install or provide the repository's configured `golangci-lint` binary before relying on lint results.
**Rationale:** `go vet` and `gofmt` passed, but the configured lint and security checks could not run.
**Status:** open

### Stage 2: Type Safety

No type-safety findings. The scan found no unchecked type assertions, bare
`interface{}` values, or unguarded pointer dereferences in the target.

### Stage 3: Error Handling

#### Medium Error Handling: runtime errors are discarded
**Location:** `cmd/majordomo/main.go:23-24`, `internal/ops/cli/root.go:50`,
`internal/ops/satools/satools.go:221`, `internal/ops/submodule/commands.go:295`
**Severity:** Medium
**Current Code:**
```go
_ = observability.Flush(ctx)
_ = observability.Shutdown(ctx)
_ = cmd.Help()
_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
_, _ = m.git([]string{"worktree", "remove", "--force", worktreePath}, m.parentRoot, false)
```
**Recommendation:** Handle each error explicitly. Return errors from command
handlers and propagate cleanup errors through named returns where a defer owns
the result.
**Rationale:** Shutdown, output, image-tagging, and worktree cleanup failures
currently disappear, leaving operators with a false success signal.
**Status:** fixed in Pass A

#### Medium Error Handling: static-analysis failures are logged and ignored
**Location:** `internal/ops/sa/sa.go:108-111`
**Severity:** Medium
**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Continue running independent tools, collect failures, and
return an aggregate error after the loop.
**Rationale:** A failed analysis tool currently produces a successful `Run`,
which can let a review proceed without required checks.
**Status:** fixed in Pass A

#### Medium Error Handling: command output write failures are discarded
**Location:** `internal/ops/cli/root.go:113` and `internal/ops/cli/root.go:502`
**Severity:** Medium
**Current Code:**
```go
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```
**Recommendation:** Use `RunE` and return the output error.
**Rationale:** Broken pipes and unavailable output streams are reported as
successful commands.
**Status:** fixed in Pass A

#### Low Test Hygiene: test setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`,
`internal/ops/satools/satools_test.go:12-32`, `internal/ops/submodule/submodule_test.go:62`
**Severity:** Low
**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
_ = filepath.Separator
```
**Recommendation:** Check setup errors with `t.Fatal` or remove the unused
separator expression.
**Rationale:** Tests can continue from invalid setup and can hide regressions.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Medium Logging: non-interactive paths write directly to standard output
**Location:** `internal/ops/sa/sa.go:35`,
`internal/ops/satools/satools.go:56-96`,
`internal/ops/cli/root.go:93`
**Severity:** Medium
**Current Code:**
```go
fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
fmt.Printf("SA Tool Image Builder\nMode:      %s\n", mode, ...)
fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
```
**Recommendation:** Route structured operational logs through the project
logging seam. Keep interactive tool output behind an injected writer.
**Rationale:** Direct writes bypass structured fields and make command output
harder to capture, test, and correlate.
**Status:** fixed in Pass A

#### Low Clarity: comment punctuation is incomplete
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low
**Current Code:**
```go
parentRoot string // empty if none
```
**Recommendation:** End the comment with a period and state the condition
clearly.
**Rationale:** Consistent comments improve generated documentation and lint
results.
**Status:** fixed in Pass A

#### Low Clarity: em dash characters appear in operator strings
**Location:** `internal/ops/cli/root.go:43`,
`internal/ops/submodule/{commands.go,git.go,menu.go}`,
`internal/ops/sa/sa.go:51`
**Severity:** Low
**Current Code:**
```go
Long: `Majordomo — repository operations for evolving software.`
```
**Recommendation:** Replace em dashes with ASCII punctuation.
**Rationale:** Repository chat and source rules require ASCII punctuation for
stable tooling and consistent operator output.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Critical Resilience: outbound process execution bypasses the resilience seam
**Location:** `internal/ops/sa/sa.go:132`,
`internal/ops/satools/satools.go:230`,
`internal/ops/submodule/git.go:222`
**Severity:** Critical
**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
cmd := exec.Command(name, args...)
cmd := exec.Command("git", args...)
```
**Recommendation:** Route process execution through one context-aware seam with
caller deadlines, classified retry policy, and circuit breaking for shared
remote dependencies.
**Rationale:** Direct execution can hang or create retry storms and does not
honor the outbound resilience gate.
**Status:** fixed in Pass A

#### Medium CLI Surface: bare and unknown invocations lack the required guide
**Location:** `internal/ops/cli/root.go:43-52`
**Severity:** Medium
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```
Observed behavior: bare invocation prints Cobra help and exits 1; an unknown
root command prints only an error and exits 1.
**Recommendation:** Make bare invocation print the structured agent guide and
exit 0. Make unknown commands print the root command catalog and exit non-zero.
**Rationale:** Operators and agents need a safe discovery path that does not
start work accidentally and does not hide the available commands.
**Status:** fixed in Pass A

## Open Questions

#### Open Architecture: CLI composition root coupling
**Location:** `internal/ops/cli/root.go:1-900`
**Observation:** The CLI root wires many command groups and imports many
application packages.
**Question:** Is the broad import surface intentional for this composition
root, or should command construction be split into smaller command groups?
**Possible outcomes:**
- If intentional, classify as a non-issue with the composition-root rationale.
- If not intentional, record an Architecture finding and defer the split until
  the operator requests a fix.

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| Discarded runtime errors | Medium | `cmd/majordomo/main.go`, `internal/ops/{cli,satools,submodule}` | Fixed by returning or reporting cleanup, output, image-tag, and worktree errors. |
| Ignored static-analysis failures | Medium | `internal/ops/sa/sa.go` | Fixed by aggregating tool failures and returning them after all tools run. |
| Test setup errors | Low | `internal/ops/sa/*_test.go`, `internal/ops/satools/*_test.go` | Fixed by checking setup errors and removing the unused expression. |
| Direct operational output | Medium | `internal/ops/{sa,satools,cli}` | Fixed with structured logging or injected operator output. |
| Comment and punctuation clarity | Low | `internal/ops/submodule`, `internal/ops/cli` | Fixed comment punctuation and replaced em dash strings. |
| Bare and unknown CLI discovery | Medium | `cmd/majordomo/main.go`, `internal/ops/cli/root.go` | Fixed with an agent guide, zero exit for bare invoke, and usage for command errors. |
| Outbound process resilience | Critical | `internal/ops/{sa,satools,submodule}` | Fixed with `internal/ops/execx` using deadline checks, retry backoff with jitter, and circuit breakers. |
| Missing `golangci-lint` | Low | Repository toolchain | Still open because the binary is unavailable; `go vet`, `gofmt`, and tests passed. |

