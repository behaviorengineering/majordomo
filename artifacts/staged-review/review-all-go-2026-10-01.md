# Review Plan: all-go
**Date:** 2026-10-01
**Target:** `cmd/**/*.go` and `internal/**/*.go` (no diff against `origin/main`)
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Pass A Commit:** `8b77aa5`
**Draft PR:** https://github.com/behaviorengineering/majordomo/pull/77

## Stages
- [x] 1. Automated Tools — Score: 9/10
- [x] 2. Type Safety — Score: 10/10
- [x] 3. Error Handling — Score: 7/10
- [ ] 4. Architecture — in progress
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity — Score: 9/10
- [x] 8. Generation Gates — Score: 6/10

## Pass A
Mechanical stages run in the requested order: 1, 2, 3, 7, 8.

## Findings

### Stage 1: Automated Tools

#### Low Tooling: golangci-lint unavailable
**Location:** Review environment
**Severity:** Low

**Current Code:**
```text
golangci-lint: missing
```

**Recommendation:** Install or provide `golangci-lint` for future full lint coverage.

**Rationale:** `go vet` and `gofmt` passed, but configured lint checks could not run in this environment.
**Status:** fixed in Pass A (environment finding recorded; no product change required)

### Stage 2: Type Safety

No findings. The target contains no unchecked type assertions, bare empty
interfaces, or unguarded pointer dereferences identified by this review.

### Stage 3: Error Handling

#### Medium Persistence/Process: Docker tag errors are discarded
**Location:** `internal/ops/satools/satools.go:219-223`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
    full := "local/sa-" + tool + ":local-test"
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```

**Recommendation:** Check the tag command error and report the build as failed
when tagging fails.

**Rationale:** A failed post-build tag currently reports a successful image
build, leaving callers with a false success result.
**Status:** fixed in Pass A

#### Low Output: CLI writer errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Low

**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:** Return or propagate output errors from command handlers.

**Rationale:** Broken pipes and unavailable output streams are hidden.
**Status:** fixed in Pass A

#### Low Test Setup: File setup errors are discarded
**Location:** `internal/ops/sa/sa_test.go:12-32`,
`internal/ops/satools/satools_test.go:12-43`
**Severity:** Low

**Current Code:**
```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
```

**Recommendation:** Fail the test immediately when fixture setup fails.

**Rationale:** Tests can continue with incomplete fixtures and fail for an
unrelated reason.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

#### Low Wording: Em dash characters appear in user-facing strings
**Location:** `internal/ops/cli/root.go`, `internal/ops/sa/sa.go`,
`internal/ops/submodule/*.go`
**Severity:** Low

**Current Code:**
```go
Majordomo — repository operations for evolving software.
```

**Recommendation:** Use ASCII punctuation in code strings.

**Rationale:** The repository writing rule forbids em dashes and ASCII output
is easier to process in logs and scripts.
**Status:** fixed in Pass A

### Stage 8: Generation Gates

#### Medium Resilience: External process calls bypass the shared resilience policy
**Location:** `internal/ops/sa/sa.go:132`,
`internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(name, args...)
err := cmd.Run()
```

**Recommendation:** Route process execution through a context-aware shared
failsafe policy with classified retries and a circuit breaker for shared
network-backed dependencies.

**Rationale:** Git, Docker, and static-analysis commands can fail transiently
or hang without consistent cancellation and retry behavior.
**Status:** fixed in Pass A

#### Medium CLI Surface: Bare invocation only prints help and returns an error
**Location:** `internal/ops/cli/root.go:47-51`
**Severity:** Medium

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
},
```

**Recommendation:** Print a structured agent operating guide for bare
invocation and exit successfully; keep command help under `help`.

**Rationale:** Agents and operators need a safe command catalog without a
spurious failure when no mutation was requested.
**Status:** fixed in Pass A

## Open Questions

#### Open Architecture: Submodule manager mixes UI, Git, and mutation workflows
**Location:** `internal/ops/submodule/git.go:37`
**Observation:** `manager` has 22 methods spanning prompt and output handling,
Git process execution, repository discovery, branch selection, worktree
cleanup, and parent pointer commits.
**Question:** Is keeping these concerns on one interactive facade intentional,
or should the UI/menu, Git adapter, and mutation workflows split?
**Why this matters:** Splitting the seams would let Git behavior and mutation
failures be tested without prompt state. Keeping the facade is simpler if this
package is intentionally a small, single-use CLI boundary.
**Status:** waiting on consultant

## Resolutions
| Finding | Outcome |
|---|---|
| Missing `golangci-lint` | Non-product environment limitation recorded |
| Docker tag error discarded | Fixed by propagating tag failure as a failed build |
| CLI writer errors discarded | Fixed by returning output errors |
| Test fixture errors discarded | Fixed by failing tests on setup errors |
| Em dash in user-facing strings | Fixed with ASCII punctuation |
| External process calls bypass resilience | Fixed with context, deadline checks, failsafe retry, jitter, and circuit breaker |
| Bare CLI invocation returns an error | Fixed with a structured agent guide and successful exit |

## Pass A Verification
- `go test ./...` passed.
- `go vet ./...` passed.
- `gofmt -l cmd internal` produced no output.
- `git diff --check` passed.
- `golangci-lint` remains unavailable and is recorded as a Low tooling finding.
