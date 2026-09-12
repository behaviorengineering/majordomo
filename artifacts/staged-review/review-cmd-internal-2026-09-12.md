# Review Plan: cmd-internal
**Date:** 2026-09-12
**Target:** `./cmd/...` and `./internal/...` (no Go diff versus `origin/main`)
**Selected Stages:** 1, 2, 3, 7, 8, then consultant stages 4, 5, 6
**Mode:** Pass A mechanical, followed by Pass B consultant

## Progress Canvas

### Progress

| Stage | Status | Score | Findings |
|---|---|---:|---:|
| 1. Automated Tools | done | 9/10 | 1 medium open |
| 2. Type Safety | done | 10/10 | fixed |
| 3. Error Handling | done | 10/10 | fixed |
| 4. Architecture | pending | — | — |
| 5. Robustness | pending | — | — |
| 6. Testability | pending | — | — |
| 7. Code Clarity | done | 9/10 | — |
| 8. Generation Gates | done | 10/10 | fixed |

**Pass:** A, mechanical complete
**Counts:** critical 0, medium 1, low 0, open 0
**Draft PR/MR:** not created

### Now

Pass A fixes are verified and ready to commit.

### Resolutions

| Type | Severity | Location | Outcome |
|---|---|---|---|
| — | — | — | — |

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

#### Medium Tooling: `golangci-lint` unavailable
**Location:** review environment
**Severity:** Medium

**Current Code:**
`golangci-lint run` could not execute because the `golangci-lint` binary is not installed.

**Recommendation:**
Install or otherwise provide the repository's configured `golangci-lint` binary, then rerun the lint gate before final verification.

**Rationale:** Without the configured linter, lint-specific defects cannot be detected. `go vet ./...` passed and `gofmt -l ./cmd ./internal` returned no files.
**Status:** open

### Stage 2: Type Safety

#### Medium Type Safety: unchecked task field assertions
**Location:** `internal/staging/cross_skill.go:71,140,176`
**Severity:** Medium

**Current Code:**
```go
summaryFiles = append(summaryFiles, t["file"].(string))
inputFile := task["input_file"].(string)
inputFile := task["input_file"].(string)
```

**Recommendation:**
Read each task field with an `ok` check and return a clear error when the required field is absent or has the wrong type.

**Rationale:** A malformed or externally produced task map causes a panic instead of a controlled staging error. The same validation should cover every copy path.
**Status:** fixed in Pass A

### Stage 3: Error Handling

#### Medium Error Handling: discarded operational errors
**Location:** `internal/staging/cross_skill.go:146,182`; `internal/staging/stage_file.go:73`; `internal/staging/routing.go:147`
**Severity:** Medium

**Current Code:**
```go
_ = copyFile(src, dst)
entries, _ := os.ReadDir(sa)
_, _ = dec.Token()
```

**Recommendation:**
Return copy and directory/decoder errors. Validate the closing JSON delimiter instead of discarding the token result.

**Rationale:** Staging can report success with missing inputs or malformed configuration, causing later review steps to operate on incomplete data.
**Status:** fixed in Pass A

#### Medium Error Handling: ignored directory-walk failures
**Location:** `internal/cluster/dep.go:298`; `internal/cluster/doc.go:263,306`; `internal/cache/cluster.go:503`
**Severity:** Medium

**Current Code:**
```go
_ = filepath.WalkDir(repoRoot, walkFn)
_ = filepath.Walk(cacheDir, walkFn)
```

**Recommendation:**
Propagate walk errors from the public helper or return a clearly marked failure instead of partial dependency, document, or cache indexes.

**Rationale:** Permission or filesystem failures currently produce incomplete indexes without notifying the caller.
**Status:** fixed in Pass A

#### Medium Error Handling: orchestration setup errors discarded
**Location:** `internal/orchestrate/run.go:291,407,491,494,541`
**Severity:** Medium

**Current Code:**
```go
_ = os.MkdirAll(...)
repo, _ = os.Getwd()
_ = os.MkdirAll(filepath.Dir(dst), ...)
```

**Recommendation:**
Return directory-creation and working-directory errors, and propagate the copy destination preparation error.

**Rationale:** The orchestration run can continue after it failed to create required output directories or resolve its workspace.
**Status:** fixed in Pass A

#### Medium Error Handling: loop archive failures discarded
**Location:** `internal/agent/loops.go:38,77,78,120,157,159`
**Severity:** Medium

**Current Code:**
```go
_ = os.MkdirAll(logsDir, 0o755)
_ = copyIfExists(summarySrc, archivePath)
```

**Recommendation:**
Return setup and archive-copy failures from the summary and technical loops.

**Rationale:** Iteration evidence is part of the review output; silently losing it makes retries and diagnostics incomplete.
**Status:** fixed in Pass A

#### Medium Error Handling: context-digest state errors discarded
**Location:** `internal/contextdigest/run.go:215,220,223,267,319,502`
**Severity:** Medium

**Current Code:**
```go
openPRNum, _ = forge.findOpenPRNumber(...)
_ = ApplyRewriteWhy(ctxDir, why)
cursorBefore, _ = ReadCursor(ctxDir)
regenFeedback, _ = g.NormalizeReject(...)
meta, _ = readRewriteMeta(ctxDir)
```

**Recommendation:**
Handle each error and stop or return a clear warning only where the operation is explicitly optional.

**Rationale:** Ignored cursor, rewrite, gate, and PR state failures can advance or regenerate context from stale state.
**Status:** fixed in Pass A

#### Medium Error Handling: cache and forge synchronization errors discarded
**Location:** `internal/cache/cache.go:77`; `internal/cache/digest.go:293,300`; `internal/reviewrun/clone.go:65`; `internal/satools/satools.go:221`
**Severity:** Medium

**Current Code:**
```go
_ = run("fetch", ...)
_ = run("add", "-A")
_, _ = git(..., "remote", "set-url", ...)
_, _, _ = runCmd(..., "docker", "tag", ...)
```

**Recommendation:**
Return or explicitly classify failures for fetch, staging, remote configuration, and image tagging.

**Rationale:** The caller currently cannot distinguish a successful cache/image operation from a partially completed one.
**Status:** fixed in Pass A

#### Medium Error Handling: gateway and process cleanup errors hidden
**Location:** `internal/aigateway/gateway.go:101,107,141,403,409`; `internal/agent/harness.go:69`
**Severity:** Medium

**Current Code:**
```go
go func() { _ = gw.server.Serve(ln) }()
_ = g.server.Shutdown(ctx)
_ = json.NewEncoder(w).Encode(...)
_ = cmd.Process.Kill()
```

**Recommendation:**
Route server lifecycle failures to a controlled error channel/logger, handle response encoding failures where possible, and classify process-kill errors during timeout cleanup.

**Rationale:** Server startup/shutdown and timeout failures can become invisible, leaving callers with false success or leaked work.
**Status:** fixed in Pass A

#### Low Error Handling: unused refinement feedback
**Location:** `internal/contextdigest/story.go:97`
**Severity:** Low

**Current Code:**
```go
_ = regenFeedback
```

**Recommendation:**
Use the feedback in the story path or remove it from the function contract.

**Rationale:** Silencing an unused parameter obscures whether regeneration feedback is intentionally ignored.
**Status:** fixed in Pass A

### Stage 7: Code Clarity

No actionable clarity findings. Existing `fmt.Print*` calls are inside the repository's
interactive CLI or logging wrappers, and no unexplained TODO/FIXME markers were found.

### Stage 8: Generation Gates

#### Medium Generation: structured reports assembled imperatively
**Location:** `internal/contextdigest/capability_constraints.go:234`; `internal/contextdigest/bootstrap_survey.go:436`; `internal/contextdigest/architecture_polish.go:59`; `internal/contextdigest/compact.go:63`; `internal/agent/techdeep.go:321`
**Severity:** Medium

**Current Code:**
```go
var b strings.Builder
b.WriteString("# Architecture\n\n")
fmt.Fprintf(&b, "- `%s` role=%s ...", ...)
```

**Recommendation:**
Move multi-section report structure into `text/template` constants and pass prepared data to the templates.

**Rationale:** Imperative report construction hides document shape and makes headings, optional sections, and repeated rows harder to review and maintain.
**Status:** fixed in Pass A

#### Critical Observability: Judge inference calls lack client spans
**Location:** `internal/judge/runtime.go:236,252`
**Severity:** Critical

**Current Code:**
```go
out, err := rt.runner.Generate(ctx, cfg, newMapInput(fields, version), nil)
llmusage.RecordExecutionState(ctx, task)
```

**Recommendation:**
Create project-standard OpenTelemetry/OpenInference spans around Generate and Evaluate, preserve the caller context, and end spans with the actual error status.

**Rationale:** Gateway HTTP traces do not show client-side parsing, evaluation, retry, or post-response hangs. Without client spans, configured OTLP exports cannot diagnose those failures.
**Status:** fixed in Pass A

#### Medium Generation: ignored canonical JSON encoding error
**Location:** `internal/cache/cluster.go:788`
**Severity:** Medium

**Current Code:**
```go
canonical, _ := json.Marshal(markdownFiles)
```

**Recommendation:**
Return the encoding error from the artifact collection helper.

**Rationale:** Hashing must not silently continue with an incomplete or invalid canonical representation.
**Status:** fixed in Pass A

## Pass A Verification

- `go test ./cmd/... ./internal/...` passed.
- `go vet ./...` passed.
- `gofmt -l ./cmd ./internal` returned no files.
- `golangci-lint` remains unavailable in the environment.

## Pass A Fix Set

All mechanical findings in Stages 2, 3, and 8 are included. Consultant stages 4, 5, and 6 remain deferred until this fix set is committed and pushed.

## Open Questions

## Resolutions

| Type | Severity | Location | Outcome |
|---|---|---|---|
| Auto-fixed in Pass A | Medium | `internal/staging/cross_skill.go` | Required task fields and copy operations now fail clearly. |
| Auto-fixed in Pass A | Medium | `internal/cluster`, `internal/cache` | Filesystem walks and canonical encoding now propagate errors. |
| Auto-fixed in Pass A | Medium | `internal/orchestrate`, `internal/agent` | Output setup and iteration archive failures now return. |
| Auto-fixed in Pass A | Medium | `internal/contextdigest`, `internal/reviewrun`, `internal/satools` | State, remote, and image-operation errors are handled. |
| Auto-fixed in Pass A | Critical | `internal/judge/runtime.go` | Generate and Evaluate now emit client-side spans. |
| Open tooling | Medium | review environment | `golangci-lint` was unavailable, so lint remains unverified. |
