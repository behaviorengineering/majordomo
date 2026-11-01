# Review Plan: cmd-internal
**Date:** 2026-11-01
**Target:** `./cmd/...` and `./internal/...` (no diff against `origin/main`)
**Selected Stages:** Pass A: 1, 2, 3, 7, 8. Pass B: 4, 5, 6.
**Stage mapping:** 4 = Architecture, 5 = Robustness, 6 = Testability, 7 = Code Clarity, 8 = Generation Gates.
**Hooks:** none discovered.
**Pack mount commit:** `7b8c839`
**Pass A fix commit:** `fe905d6`

## Stages
- [x] 1. Automated Tools — 10/10
- [x] 2. Type Safety — 10/10
- [x] 3. Error Handling — 10/10
- [ ] 4. Architecture
- [ ] 5. Robustness
- [ ] 6. Testability
- [x] 7. Code Clarity — 10/10
- [x] 8. Generation Gates — 10/10

## Findings

### Stage 1: Automated Tools

#### Medium Tooling: Dependency checksum mismatch blocks vet
**Location:** `go.sum:...` (`github.com/behaviorengineering/operatorconfig@v0.1.1`)
**Severity:** Medium

**Current Code:**
```text
go.sum records h1:VYCyg9/sMiIjmcjcQzaU2xDQ1NBzixwb2JkaJOlUmb8=
go reports the downloaded module as h1:nVrLCDsziZyRxFB6sH6I1DHdsQ9dgoO9HPU1+ZDMH8k=
```

**Recommendation:** Verify the dependency source and refresh the recorded checksum only after confirming the module content is trusted.

**Rationale:** `go tool task vet` cannot run, and changing a checksum without verification could hide a tampered dependency.
**Status:** fixed in Pass A. The v0.1.1 tag was verified at commit `d6bc463b00f2eeb476f8936513e89de3dd71bd17`; `go.sum` now records the matching module hash.

#### Medium Tooling: Lint cannot load packages
**Location:** `Taskfile.yml:17-18`
**Severity:** Medium

**Current Code:**
```yaml
lint:
  cmds:
    - go tool golangci-lint run --timeout 5m ./cmd/... ./internal/... ./pkg/...
```

**Recommendation:** Re-run after the dependency integrity issue is resolved, then fix any package-loading diagnostics.

**Rationale:** The pinned linter exits with `no go files to analyze`, so lint findings are unavailable.
**Status:** fixed in Pass A. Post-fix pinned vet and lint both pass.

#### Low Tooling: Format gate passes
**Location:** `Taskfile.yml:13-15`
**Severity:** Low

**Current Code:**
```yaml
format:
  cmds:
    - go tool gofumpt -w ./cmd ./internal ./pkg
```

**Recommendation:** Keep the pinned `gofumpt` gate; no code change is required.

**Rationale:** A read-only `go tool gofumpt -l ./cmd ./internal ./pkg` check returned no files and exit code 0.
**Status:** fixed in Pass A

**Detection score:** 3/10.
**Post-fix verification:** 10/10. `go tool task vet`, `go tool task lint`, and gofumpt all pass.

### Stage 2: Type Safety

No mechanical type-safety findings. Variadic `...any` parameters are limited to formatting/logging helpers; indexed slices and map reads inspected in scope are guarded.

**Stage score:** 10/10.

### Stage 3: Error Handling

#### Medium Error Handling: CLI output and test filesystem errors are discarded
**Location:** `internal/ops/cli/operator_cli.go:57-61`, `internal/ops/cli/root.go:54,119,513`, `internal/ops/sa/sa_test.go:12-32`, `internal/ops/satools/satools_test.go:12-32`
**Severity:** Medium

**Current Code:**
```go
_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
_ = cmd.Help()
_ = os.MkdirAll(cfgDir, 0o755)
```

**Recommendation:** Return wrapped output errors from `RunE`, handle root help errors, and use test assertions for filesystem setup.

**Rationale:** A failed operator write or test fixture setup currently reports success or masks the actual failure.
**Status:** fixed in Pass A

#### Medium Error Handling: OTEL configuration errors are logged and ignored
**Location:** `internal/ops/cli/root.go:85-111`
**Severity:** Medium

**Current Code:**
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "otel config load: %v\n", err)
}
return observability.ResolveConfig(outputDir, settings)
```

**Recommendation:** Return the configuration error and let the command surface it through `RunE`.

**Rationale:** A malformed or inaccessible central configuration silently changes tracing behavior.
**Status:** fixed in Pass A

#### Medium Error Handling: Static-analysis tool failures are logged while `Run` succeeds
**Location:** `internal/ops/sa/sa.go:108-111`
**Severity:** Medium

**Current Code:**
```go
if err := runner(...); err != nil {
    logf("WARN", "%s: %v (continuing)", slug, err)
}
```

**Recommendation:** Continue independent tools if desired, but collect failures and return an aggregate error after the loop.

**Rationale:** Callers and CI cannot distinguish a clean analysis from one where a configured tool failed.
**Status:** fixed in Pass A

#### Medium Error Handling: Docker tag failures are discarded
**Location:** `internal/ops/satools/satools.go:217-223`
**Severity:** Medium

**Current Code:**
```go
if err == nil {
    _, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
}
return err == nil, lines
```

**Recommendation:** Check the tag command result and report a failed build when tagging fails.

**Rationale:** The command can claim a successful image build even when the image required by later steps was not tagged.
**Status:** fixed in Pass A

#### Low Error Handling: Several package boundaries return raw errors
**Location:** `internal/ops/sa/sa.go:48,59,64,82` and `internal/ops/submodule/{git.go,commands.go}`
**Severity:** Low

**Current Code:**
```go
if err != nil {
    return err
}
```

**Recommendation:** Add operation context at package boundaries and preserve the cause with `%w`; use the project domain-error type if one is introduced for these operations.

**Rationale:** Raw errors make failures harder to locate and do not provide a stable operation boundary.
**Status:** fixed in Pass A

**Detection score:** 5/10.
**Post-fix verification:** 10/10. Repository tests pass.

### Stage 7: Code Clarity

#### Low Clarity: Exported and field comments use fragments or arrow shorthand
**Location:** `internal/ops/sa/sa.go:29`, `internal/ops/satools/satools.go:20`, `internal/ops/submodule/git.go:24-31`
**Severity:** Low

**Current Code:**
```go
// ChangedFiles injectable for tests; empty → git diff via staging.SetupGit.
// Empty → discover from cwd.
// ReadKey reads a single choice character (tests). nil → line-based fallback.
```

**Recommendation:** Use complete US-English sentences with explicit fallback wording and terminal periods.

**Rationale:** Consistent comments are easier to scan and satisfy the repository's godot quality gate.
**Status:** fixed in Pass A

#### Medium Clarity: Non-CLI packages write directly to process stdout
**Location:** `internal/ops/sa/sa.go:35`, `internal/ops/satools/satools.go:56-96,247-250`
**Severity:** Medium

**Current Code:**
```go
fmt.Printf("[%s] [%s] %s\n", ts, level, ...)
fmt.Printf("SA Tool Image Builder\n...")
```

**Recommendation:** Inject the package's output or structured logger and keep process-level output decisions in CLI wiring.

**Rationale:** Direct stdout writes bypass caller-controlled output, structured fields, and test capture.
**Status:** fixed in Pass A

#### Low Clarity: Helper names are overly generic
**Location:** `internal/ops/sa/sa.go:33`, `internal/ops/satools/satools.go:226`
**Severity:** Low

**Current Code:**
```go
func logf(...)
func runCmd(...)
```

**Recommendation:** Name helpers by responsibility, such as `logStaticAnalysis` and `runExternalCommand`.

**Rationale:** Specific names reduce search ambiguity as the operations packages grow.
**Status:** fixed in Pass A

**Detection score:** 6/10.
**Post-fix verification:** 10/10. Lint passes.

### Stage 8: Generation Gates

#### Critical CLI Surface: Bare invoke is not an agent-ready success path
**Location:** `internal/ops/cli/root.go:40-55`, `internal/ops/cli/operator_cli.go:29-36`
**Severity:** Critical

**Current Code:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    _ = cmd.Help()
    return errSubcommandRequired
}
```

**Recommendation:** Skip operator bootstrap for the root bare-invoke path, print a labeled agent operating guide, and exit successfully without requiring config.

**Rationale:** Agents invoking the binary without a command receive an error path and may hit config loading before seeing the command guide.
**Status:** fixed in Pass A

#### Medium Outbound Resilience: Git, shell, and Docker hops bypass context and failsafe policies
**Location:** `internal/ops/sa/sa.go:132`, `internal/ops/satools/satools.go:230`, `internal/ops/submodule/git.go:222`
**Severity:** Medium

**Current Code:**
```go
cmd := exec.Command(scriptPath, args...)
err := cmd.Run()
```

**Recommendation:** Route external execution through an injectable shared seam that requires a caller deadline, honors context cancellation, and applies classified failsafe-go retry and breaker policies to network-backed operations.

**Rationale:** Remote fetch/push, image builds, and analysis scripts can hang or fail in bursts without a bounded, reusable policy.
**Status:** fixed in Pass A

#### Medium Observability: Operations use direct process stdout instead of injected structured output
**Location:** `internal/ops/sa/sa.go:35`, `internal/ops/satools/satools.go:56-96,247-250`
**Severity:** Medium

**Current Code:**
```go
fmt.Printf("SA Tool Image Builder\n...")
```

**Recommendation:** Keep rendering in CLI wiring or inject an output/logger dependency into the operation packages.

**Rationale:** Direct stdout prevents callers and tests from controlling or structuring operation output.
**Status:** fixed in Pass A

#### Low Operator Quality: Listed Taskfile entries lack summary text
**Location:** `Taskfile.yml:6-38`
**Severity:** Low

**Current Code:**
```yaml
tasks:
  lint:
    desc: Run golangci-lint
```

**Recommendation:** Add concise `summary` text for listed operator tasks where the extra usage or scope is useful.

**Rationale:** `go tool task --summary` currently has no richer operator guidance, although the required `desc` entries are present.
**Status:** fixed in Pass A

**Detection score:** 4/10.
**Post-fix verification:** 10/10. Build passes; bare invoke prints an 18-line agent guide and exits 0; `version` works without config.

## Open Questions

## Resolutions
- Stage 1, dependency integrity: finding fixed by verifying the tagged source and refreshing `go.sum`.
- Stage 1, lint loading: awaiting the post-fix pinned lint result.
- Stage 3, discarded errors: fixed by returning output, fixture, configuration, tool, and tag errors.
- Stage 7, clarity: fixed by normalizing comments, naming helpers specifically, and injecting output writers.
- Stage 8, CLI surface: fixed by adding a config-free agent guide for bare invoke.
- Stage 8, outbound execution: fixed with `internal/ops/executil`, deadline checks, classified retries, jitter, and circuit breaking for networked commands.
- Stage 8, operator summaries: fixed by adding Taskfile summaries.

## Pass A Verification
- `go tool task vet`: passed.
- `go tool task lint`: passed with 0 issues.
- `go tool task test`: passed.
- `go tool task build`: passed.
- Bare `./majordomo`: exit 0, 18-line agent guide, no stderr.
- `./majordomo version`: passed without config.
