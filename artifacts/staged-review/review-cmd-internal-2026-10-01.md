# Staged Review Plan: cmd-internal

**Date:** 2026-10-01
**Target:** `./cmd/...` and `./internal/...` because the scheduled run has no diff against `origin/main`.
**Selected stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Branch:** `cursor/staged-code-review-process-9ce0`

## Stages

- [x] 1. Automated tools, score 9/10
- [x] 2. Type safety, score 10/10
- [x] 3. Error handling, score 7/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code clarity, score 10/10
- [x] 8. Generation gates, score 5/10

## Pass status

- Pass A: in progress, fixing mechanical findings.
- Pass B: waiting for the Pass A commit and draft pull request.

## Findings

### Stage 1: Automated tools

#### Low tooling: golangci-lint is unavailable
**Severity:** Low
**Location:** Repository toolchain
**Status:** Fixed in Pass A

**Current code/output:**

```text
golangci-lint: missing
```

**Recommendation:** Install or provide the repository's configured `golangci-lint` binary before relying on lint coverage.

**Rationale:** `go vet` passed and the target Go files are formatted, but the lint gate could not run.

## Stage summaries

### Stage 1

- `go vet ./...`: passed.
- `gofmt -l` on `cmd` and `internal` Go files: passed.
- `golangci-lint run ./cmd/... ./internal/...`: passed with v2.14.0 built by Go 1.27.
- Initial score: 9/10. Final score after the Pass A tooling fix: 10/10.

### Stage 2

- No unchecked type assertions were found.
- No unsafe map or slice access was found in the reviewed paths.
- Uses of `any` are limited to variadic formatting and runner arguments where the values are intentionally heterogeneous.
- Required pointer and injected dependency checks did not reveal a mechanical type-safety finding.
- Score: 10/10.

### Stage 3

#### Medium error handling: Docker tag failures are discarded
**Location:** `internal/ops/satools/satools.go:218-223`
**Status:** Fixed in Pass A

**Current code:**

```go
if err == nil {
	full := "local/sa-" + tool + ":local-test"
	_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```

**Recommendation:** Check the tag command result and report a failed tag as a failed build.

**Rationale:** The command can report a successful image build while the image required by later validation was never tagged.

#### Medium error handling: Static-analysis failures are logged and ignored
**Location:** `internal/ops/sa/sa.go:108-111`
**Status:** Fixed in Pass A

**Current code:**

```go
if err := runner(...); err != nil {
	logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Run all configured tools, collect runner errors, and return an aggregate error after the loop.

**Rationale:** Returning nil after a failed tool makes CI and callers treat incomplete analysis as successful.

#### Medium error handling: Working-directory lookup hides failure
**Location:** `internal/ops/sa/sa.go:150`
**Status:** Fixed in Pass A

**Current code:**

```go
wd, _ := os.Getwd()
```

**Recommendation:** Return a wrapped error when the current directory cannot be read.

**Rationale:** The function otherwise searches from an invalid empty path and can produce a misleading missing-script error.

#### Medium error handling: Git discovery errors are silently converted to empty state
**Location:** `internal/ops/submodule/git.go:96-101, 129, 146, 189`
**Status:** Fixed in Pass A

**Current code:**

```go
indexEntry, _ := m.git(...)
gitDirRaw, _ := m.git(...)
entry, _ := m.git(...)
out, _ := m.git(...)
```

**Recommendation:** handle each command error explicitly, returning the error from required operations and preserving an intentional not-found result only where the Git command contract permits it.

**Rationale:** A failed Git process is indistinguishable from a repository that is not a submodule, clean, or missing a remote branch.

#### Low error handling: CLI output and help errors are discarded
**Location:** `internal/ops/cli/root.go:50, 113, 502`
**Status:** Fixed in Pass A

**Current code:**

```go
_ = cmd.Help()
_, _ = fmt.Fprintln(...)
_, _ = fmt.Fprintf(...)
```

**Recommendation:** return output errors from `RunE` handlers and use `RunE` for the version command.

**Rationale:** Broken pipes and failing writers should stop the command with a clear error.

#### Low test error handling: Test setup errors are discarded
**Location:** `internal/ops/satools/satools_test.go`, `internal/ops/sa/sa_test.go`, `internal/ops/submodule/submodule_test.go`
**Status:** Fixed in Pass A

**Current code:**

```go
_ = os.MkdirAll(...)
_ = os.WriteFile(...)
_ = filepath.Separator
```

**Recommendation:** fail tests immediately on setup errors and remove the unused separator expression.

**Rationale:** A test can otherwise continue against incomplete fixtures or retain a misleading import.

- Score: 7/10.

### Stage 7

- Comments in the reviewed Go files use complete sentences.
- Interactive CLI output is intentionally written with `fmt` from command packages; no service logger bypass was found that can be fixed independently without changing the package's operator-output contract.
- No unexplained TODO or FIXME markers were found.
- Names and exports are scoped to the package responsibilities.
- Score: 10/10.

### Stage 8

#### High outbound resilience: Remote process execution bypasses context and failsafe policies
**Location:** `internal/ops/submodule/git.go:224-234`, `internal/ops/satools/satools.go:229-238`, `internal/ops/sa/sa.go:130-138`
**Status:** Fixed in Pass A

**Current code:**

```go
cmd := exec.Command("git", args...)
...
err := cmd.Run()
```

**Recommendation:** Route outbound process execution through a shared failsafe-go seam with caller context, exponential backoff with jitter, circuit breaking for remote operations, and retry classification. Require a caller deadline before remote work.

**Rationale:** Remote Git, Docker, and static-analysis process calls can hang or create retry storms without cancellation and bounded resilience.

#### Medium observability: Static-analysis logging is an unstructured package-level output helper
**Location:** `internal/ops/sa/sa.go:31-36`
**Status:** Fixed in Pass A

**Current code:**

```go
func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

**Recommendation:** Inject the command's structured logger or a focused output interface and include the repository, tool, and operation as fields.

**Rationale:** Package-level formatted output is difficult to redirect, test, and correlate in automated runs.

#### Medium context safety: Process options do not carry a cancellation or deadline
**Location:** `internal/ops/submodule.Options`, `internal/ops/satools.Options`, and `internal/ops/sa.Options`
**Status:** Fixed in Pass A

**Current code:**

```go
type Options struct {
	...
	Runner ...
}
```

**Recommendation:** Add a required caller context to outbound operation options and fail closed when it is nil or lacks a deadline.

**Rationale:** The current APIs cannot stop a remote process when the CLI or job is canceled and cannot enforce the caller's execution budget.

- Package layout, interface size, timestamp, transaction, HTTP-body, and CLI version/help checks did not reveal additional mechanical findings.
- Score: 5/10.

## Pass A fixes and verification

- Added `internal/ops/process` with caller deadline checks, `exec.CommandContext`, retry classification, exponential backoff with jitter, and per-dependency circuit breakers.
- Routed Git, Docker, build-script, and static-analysis process calls through the shared seam.
- Added command contexts and bounded budgets at CLI entrypoints.
- Returned static-analysis aggregate failures and Docker tag failures.
- Replaced static-analysis package-level formatted logging with an injected structured logger.
- Checked CLI writer errors and test fixture setup errors.
- Verification: `go test ./...`, `go vet ./...`, focused target tests, focused target vet, target formatting, and `golangci-lint v2.14.0` all passed.

Pass A mechanical fixes are complete. The commit and draft pull request are still pending.

## Open questions

None recorded yet.

## Resolutions

| Finding | Severity | Location | Outcome |
|---|---|---|---|
| Missing lint tool | Low | Repository toolchain | Resolved by installing a Go 1.27-compatible lint binary; lint passed. |
| Ignored command and output errors | Medium/Low | `internal/ops/{cli,sa,satools,submodule}` | Fixed in Pass A; failures now return or aggregate explicitly. |
| Unstructured static-analysis logging | Medium | `internal/ops/sa/sa.go` | Fixed in Pass A with an injected structured logger. |
| Unbounded outbound process execution | High/Medium | `internal/ops/process` and callers | Fixed in Pass A with context, retry, and breaker policies. |
