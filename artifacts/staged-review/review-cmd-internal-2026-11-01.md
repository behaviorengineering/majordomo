# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...`
**Selected Stages:** Pass A: 1, 2, 3, 7, 8; Pass B: 4, 5, 6
**Base:** `origin/main`
**Hooks:** none found

## Stages
- [x] 1. Automated Tools
- [x] 2. Type Safety
- [x] 3. Error Handling
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity
- [x] 8. Generation Gates

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: Quality task scope is valid, but lint cannot load packages
**Location:** `Taskfile.yml:19-28`
**Severity:** Medium
**Current Code:**
```yaml
format:
  cmds:
    - go tool gofumpt -w ./cmd ./internal ./pkg
lint:
  cmds:
    - go tool golangci-lint run --timeout 5m ./cmd/... ./internal/... ./pkg/...
vet:
  cmds:
    - go vet ./cmd/... ./internal/... ./pkg/...
```
**Recommendation:** Diagnose and restore golangci-lint package loading after the dependency-integrity blocker is resolved; keep the valid package scope.
**Rationale:** `go tool task lint` exits with `context loading failed: no go files to analyze`; the package scope itself is valid because `pkg/` exists.
**Status:** open (lint loader remains blocked by module integrity)

#### High Dependency Integrity: Vet is blocked by a checksum mismatch
**Location:** `go.sum:240`
**Severity:** High
**Current Code:**
```text
github.com/behaviorengineering/operatorconfig v0.1.1 h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
```
**Recommendation:** Restore a module artifact matching the published Go checksum before changing the dependency or checksum file. Do not replace the trusted checksum with the mismatching downloaded artifact.
**Rationale:** Go reports a downloaded checksum of `h1:nVrLCDsziZyRxFB6sH6I1DHdsQ9dgoO9HPU1+ZDMH8k=` while `sum.golang.org` confirms the committed checksum. Vet and package loading therefore fail closed.
**Status:** open (external dependency artifact mismatch)

#### Low Format: No formatting findings
**Location:** `./cmd/...`, `./internal/...`, `./pkg/...`
**Severity:** Low
**Current Code:** `go tool gofumpt -l` returned no files.
**Recommendation:** None.
**Rationale:** The current Go sources are formatted.
**Status:** non-issue

**Stage score:** 5/10. Task listing and formatting passed; lint and vet did not complete.

### Stage 2: Type Safety

No findings. The reviewed `cmd/` and `internal/` Go files do not use unchecked type assertions, unsafe map/pointer dereferences, or empty-interface values where a concrete type is required. The `any` values are formatting variadic arguments.

**Stage score:** 10/10.

### Stage 3: Error Handling

#### Medium Error Boundary: OTEL configuration errors are logged and ignored
**Location:** `internal/ops/cli/root.go:84-108`
**Severity:** Medium
**Current Code:**
```go
if err != nil {
	fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
} else {
	// use loaded settings
}
return observability.ResolveConfig(outputDir, settings)
```
**Recommendation:** Return the configuration error to the command boundary instead of silently using default settings.
**Rationale:** A malformed or missing repository configuration can disable the intended telemetry endpoint while the command continues as if configuration succeeded.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis tool failures are swallowed
**Location:** `internal/ops/sa/sa.go:108-112`
**Severity:** Medium
**Current Code:**
```go
if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
	logf("WARN", "%s: %v (continuing)", slug, err)
}
```
**Recommendation:** Collect tool failures and return them after all configured tools have run, or stop and return the first failure.
**Rationale:** Returning nil after a failed analyzer makes a failed static-analysis gate appear successful.
**Status:** fixed in Pass A

#### Medium Resource/Process Error: Docker tag failures do not affect the build result
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
**Recommendation:** Check the tag command error and mark the build failed with diagnostic output.
**Rationale:** A missing published tag causes later consumers to fail even though this command reports a successful build.
**Status:** fixed in Pass A

#### Low Error Handling: Probe and setup errors are discarded
**Location:** `internal/ops/sa/sa.go:150`, `internal/ops/submodule/git.go:96-100,129,146,189`, and test setup helpers
**Severity:** Low
**Current Code:**
```go
wd, _ := os.Getwd()
indexEntry, _ := m.git(...)
out, _ := m.git(...)
_ = os.WriteFile(...)
```
**Recommendation:** Handle each error explicitly, fail closed for destructive decisions, and make test setup fail through `t.Fatal`.
**Rationale:** Discarded errors make repository state and test fixtures look valid when their operations failed.
**Status:** fixed in Pass A

**Stage score:** 4/10.

### Stage 7: Code Clarity

#### Low Comment: Inline field comment lacks terminal punctuation
**Location:** `internal/ops/submodule/git.go:38`
**Severity:** Low
**Current Code:**
```go
parentRoot string // empty if none
```
**Recommendation:** End the comment with a period.
**Rationale:** Consistent sentence punctuation keeps generated documentation and lint output clear.
**Status:** fixed in Pass A

**Stage score:** 9/10.

### Stage 8: Generation Gates

#### High CLI Surface: Bare invocation is not agent-ready
**Location:** `internal/ops/cli/root.go:37-59`
**Severity:** High
**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
	_ = cmd.Help()
	return errSubcommandRequired
},
```
**Recommendation:** Print a structured agent operating guide on bare invocation and exit successfully; keep detailed flags under `help`.
**Rationale:** Automation currently receives an error for a normal no-argument probe, and the output omits the required role, lifecycle, documentation, and safe-mutation guidance.
**Status:** fixed in Pass A

#### Medium Outbound Resilience: Remote git commands bypass the resilient execution seam
**Location:** `internal/ops/submodule/git.go:219-236`
**Severity:** Medium
**Current Code:**
```go
cmd := exec.Command("git", args...)
cmd.Dir = cwd
err := cmd.Run()
```
**Recommendation:** Thread the Cobra context into the submodule manager and execute git through the existing `gitvalet/pkg/gitexec` runner, which enforces caller deadlines, retry classification, and circuit breaking.
**Rationale:** Fetch, pull, and push are network-backed process hops. Direct execution has no cancellation budget or transient-failure resilience.
**Status:** fixed in Pass A

#### Low Generation Gate: Task quality scope was incorrectly suspected as stale
**Location:** `Taskfile.yml:13-28`
**Severity:** Low
**Current Code:** Quality tasks include `./pkg`, and this checkout does contain `pkg/`.
**Recommendation:** None. Keep the complete module scope.
**Rationale:** The original suspicion came from an incomplete file search; the path is valid.
**Status:** non-issue

**Stage score:** 5/10.

## Open Questions

### Dependency integrity
- `operatorconfig v0.1.1` still downloads with a checksum different from the trusted `go.sum` and `sum.golang.org` entry. The mismatch was not auto-resolved.

## Resolutions

| Finding | Severity | Outcome |
|---|---|---|
| Lint package loading | Medium | Still open, full lint exits before analysis while module loading is blocked. |
| `operatorconfig` checksum mismatch | High | Still open, trusted checksum preserved and no unsafe `go.sum` replacement made. |
| OTEL configuration error propagation | Medium | Auto-fixed in Pass A, resolver now returns configuration errors. |
| Static-analysis failures | Medium | Auto-fixed in Pass A, failures are joined and returned after the tool run. |
| Docker tag result | Medium | Auto-fixed in Pass A, tag failure now fails the build result. |
| Ignored probe and fixture errors | Low | Auto-fixed in Pass A, probes fail closed and test setup uses `t.Fatal`. |
| CLI agent guide | High | Auto-fixed in Pass A, bare invocation prints a structured guide and exits successfully. |
| Remote git resilience | Medium | Auto-fixed in Pass A, commands use context deadlines and `gitexec` resilience. |
| Comment punctuation | Low | Auto-fixed in Pass A. |
