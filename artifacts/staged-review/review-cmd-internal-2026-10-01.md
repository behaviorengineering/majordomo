# Review Plan: cmd-internal
**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the branch has no Go diff from `origin/main`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.

## Stages
- [x] 1. Automated Tools — Score: 8/10
- [x] 2. Type Safety — Score: 10/10
- [x] 3. Error Handling — Score: 5/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity — Score: 7/10
- [x] 8. Generation Gates — Score: 4/10

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: golangci-lint is unavailable
**Location:** Review environment
**Severity:** Medium

**Current Code:**
`golangci-lint run ./cmd/... ./internal/...` could not run because the binary is not installed.

**Recommendation:**
Install or provide the repository's configured golangci-lint before relying on lint coverage.

**Rationale:** The vet and format checks passed, but lint-specific checks such as error handling and comment rules were not executed.
**Status:** Fixed in Pass A. The latest golangci-lint was installed with the Go 1.27 toolchain and now passes.

### Stage 2: Type Safety

No findings. The target contains no unchecked type assertions, bare empty interfaces, or unguarded constructor pointer dependencies.

### Stage 3: Error Handling

#### Medium Error Handling: CLI output errors are discarded
**Location:** `internal/ops/cli/root.go:50,113,502`
**Severity:** Medium

**Current Code:**
```go
_ = cmd.Help()
_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
```

**Recommendation:**
Return output and help-writing errors from the command handlers.

**Rationale:** A broken pipe or unavailable output stream currently reports success or hides the more useful failure.
**Status:** Fixed in Pass A.

#### Medium Error Handling: Static-analysis failures are logged and suppressed
**Location:** `internal/ops/sa/sa.go:108-112`
**Severity:** Medium

**Current Code:**
```go
if err := runner(...); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:**
Continue running remaining tools, but aggregate tool failures and return the aggregate after the loop.

**Rationale:** The command can exit successfully even when one or more configured analysis tools fail.
**Status:** Fixed in Pass A.

#### Medium Error Handling: Docker tag failure is discarded
**Location:** `internal/ops/satools/satools.go:218-223`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
```

**Recommendation:**
Treat a failed tag operation as a failed build and preserve its output.

**Rationale:** The reported build result can be successful even though the expected local image tag was not created.
**Status:** Fixed in Pass A.

#### Low Error Handling: Repository discovery ignores `os.Getwd` failure
**Location:** `internal/ops/sa/sa.go:150`
**Severity:** Low

**Current Code:**
```go
wd, _ := os.Getwd()
```

**Recommendation:**
Return the working-directory error instead of searching from an empty path.

**Rationale:** A failed working-directory lookup should fail clearly and avoid misleading script-not-found errors.
**Status:** Fixed in Pass A.

#### Medium Error Handling: Submodule probes discard git errors
**Location:** `internal/ops/submodule/git.go:96,100,129,146,189`
**Severity:** Medium

**Current Code:**
```go
indexEntry, _ := m.git(...)
gitDirRaw, _ := m.git(...)
entry, _ := m.git(...)
out, _ := m.git(...)
```

**Recommendation:**
Handle each probe error explicitly, using safe defaults only where the operation is intentionally optional, and fail closed for dirty-state checks.

**Rationale:** Git failures can be mistaken for a clean tree, an untracked submodule, or a missing remote branch.
**Status:** Fixed in Pass A.

#### Low Error Handling: Worktree cleanup failure is invisible
**Location:** `internal/ops/submodule/commands.go:293-298`
**Severity:** Low

**Current Code:**
```go
defer func() {
    _, _ = m.git(..., false)
}()
```

**Recommendation:**
Report cleanup failure to the operator while preserving the original operation result.

**Rationale:** A stale worktree can block later runs and is otherwise difficult to diagnose.
**Status:** Fixed in Pass A.

### Stage 7: Code Clarity

#### Low Clarity: Comments use non-standard shorthand and omit periods
**Location:** `internal/ops/submodule/git.go:27-31,40`; `internal/ops/satools/satools.go:20`
**Severity:** Low

**Current Code:**
```go
// GitRunner overrides git execution (tests). nil → real git.
parentRoot string // empty if none
// Empty → discover from cwd.
```

**Recommendation:**
Use plain English and complete sentences in comments.

**Rationale:** Consistent comments are easier to scan and satisfy the repository's documentation lint rules.
**Status:** Fixed in Pass A.

#### Low Clarity: Operator messages contain em dashes
**Location:** `internal/ops/cli/root.go:43`; `internal/ops/submodule/git.go:76,179`; `internal/ops/submodule/commands.go`; `internal/ops/submodule/menu.go`; `internal/ops/sa/sa.go:51`
**Severity:** Low

**Current Code:**
```go
Majordomo — repository operations for evolving software.
```

**Recommendation:**
Replace em dashes with colons, commas, or parentheses.

**Rationale:** The workspace writing rule prohibits U+2014 in source strings and documentation.
**Status:** Fixed in Pass A.

### Stage 8: Generation Gates

#### Medium Generation: Outbound process execution has no resilience policy
**Location:** `internal/ops/submodule/git.go:222`; `internal/ops/sa/sa.go:132`; `internal/ops/satools/satools.go:230`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(...)
err := cmd.Run()
```

**Recommendation:**
Route shared outbound process execution through a context-aware failsafe policy with bounded retry and classified failures.

**Rationale:** Git, shell, and Docker subprocesses can fail transiently. Bare execution provides no retry, cancellation budget, or consistent failure policy.
**Status:** Fixed in Pass A with the shared `internal/ops/process` executor, deadline checks, retries, jitter, and circuit breakers.

#### Medium Generation: Dispatch does not initialize process tracing
**Location:** `internal/ops/cli/root.go:195-225`
**Severity:** Medium

**Current Code:**
```go
return dispatch.Dispatch(opts)
```

**Recommendation:**
Initialize the configured tracer before the in-process Judge dispatch, and preserve the caller context.

**Rationale:** Gateway HTTP traces do not expose client-side generation, parsing, or evaluation failures.
**Status:** Fixed in Pass A.

#### Low Generation: No root Makefile quality target is available
**Location:** Repository root
**Severity:** Low

**Current Code:**
The repository has no root `Makefile`, so the shared `make vet`, `make lint`, and `make test` entry points are unavailable.

**Recommendation:**
Add a root Makefile only if this module's operator workflow requires those shared verbs.

**Rationale:** The review used direct Go commands, but operators do not have a standard quality command surface.
**Status:** Fixed in Pass A with root `Makefile` help, build, test, vet, tidy, lint, and format verbs.

## Pass A Fixes

- Added a shared context-aware subprocess executor with classified retry and circuit-breaker policies.
- Propagated CLI output, static-analysis, Docker-tag, git probe, and cleanup errors.
- Initialized tracing for dispatch and added bounded contexts to outbound CLI operations.
- Added the root Makefile quality command surface and repaired module checksums.

## Open Questions

## Resolutions

