# Pipeline Code Review Findings

*Majordomo — repository operations for evolving software.*

> **Historical — Jenkins retired.** This document predates removal of Jenkinsfiles, Groovy stages, and `.majordomo-config.groovy`. Behaviour below is a porting spec for the Go/GHA control tower — see [PLAN — Control Tower, GitHub Actions, and Go](../PLAN-control-tower-github-go.md).


**Date:** 2026-05-07  
**Reviewed:** 2026-05-22  
**Scope (archived):** Former `MajordomoReview.CI.Jenkinsfile`, `stages/*.groovy`, `lib/*.groovy` — all removed  
**Status:** All findings resolved or accepted

---

## Executive Summary

The pipeline is architecturally sound and implements sophisticated orchestration patterns. Five operational smells and one portability note identified. Top risk: early-stage failures (registry auth, image build) leave Bitbucket status notifications undelivered. Secondary risk: Static Analysis diff scope depends on undocumented assumptions about remote ref freshness.

---

## Findings

### [WARN] Early-Failure Status Reporting is Fragile

**Severity:** High  
**Impact:** Pipeline failures before `Ensure Images` completes silently — no Bitbucket status notification, no operator visibility.

**Files:**
- [pipelines/MajordomoReview.CI.Jenkinsfile](../pipelines/MajordomoReview.CI.Jenkinsfile#L397) — post blocks call notifier
- [pipelines/MajordomoReview.CI.Jenkinsfile](../pipelines/MajordomoReview.CI.Jenkinsfile#L590) — notifyBitbucketBuildStatus helper
- [lib/notify.groovy](../lib/notify.groovy#L17) — notifyBitbucketBuildStatus method

**Root Cause:**
The post blocks unconditionally invoke `notifyBitbucketBuildStatus()`, but the helper immediately returns if `env.COPILOT_FULL_IMAGE` is empty:
```groovy
if (!env.COPILOT_FULL_IMAGE?.trim()) {
    echo '[WARN] COPILOT_FULL_IMAGE is empty; skipping Bitbucket build-status notification.'
    return
}
```

This is exactly the case when `Ensure Images` or registry auth fails — the variable is never set, so no status appears in Bitbucket even though the build failed.

**Recommendation:**
Move Bitbucket notification to an earlier stage that always runs, or compute the commit SHA and token credential ID independently of image resolution. Use a fallback mechanism (e.g. a curl call without Docker) for critical early-stage failures.

---

### [WARN] Static-Analysis Diff Scope Depends on Undocumented Remote Ref State

**Severity:** High  
**Impact:** Static Analysis may scan stale or incorrect file sets if origin/baseBranch is not freshly fetched.

**Files:**
- [stages/static-analysis.groovy](../stages/static-analysis.groovy#L10) — changedFiles() computes diff
- [pipelines/MajordomoReview.CI.Jenkinsfile](../pipelines/MajordomoReview.CI.Jenkinsfile#L724) — checkoutBranch() refuses to fetch

**Root Cause:**
`changedFiles()` runs:
```groovy
git diff --name-only --diff-filter=d 'origin/${baseBranch}...HEAD'
```

But `checkoutBranch()` explicitly documents:
```groovy
// Do not fetch here: this shell context does not have SCM credentials attached.
```

If `checkout scm` in the Safe Checkout stage does not refresh `origin/baseBranch` for the current webhook event, the diff will compare against stale remote state. On fast-moving PR branches or webhook timing issues, this can include or exclude files unpredictably.

**Recommendation:**
Either (a) ensure `checkout scm` runs with fetch-all credentials before Static Analysis runs, (b) make `checkoutBranch` compute the diff target before resetting, or (c) add explicit documentation of the required webhook and checkout configuration to guarantee fresh remote refs.

**Resolution (2026-05-22):** Accepted as low risk. Jenkins `checkout scm` for PR builds fetches all remote refs. The edge case (commits landing on baseBranch between checkout and SA stage) is unlikely in practice. No code change required.

---

### [WARN] Shared-Library Fallback is Inconsistent

**Severity:** Medium  
**Impact:** Pipeline partially compatible with mixed repo/submodule layouts; brittle fallback pattern.

**Files:**
- [pipelines/MajordomoReview.CI.Jenkinsfile](../pipelines/MajordomoReview.CI.Jenkinsfile#L676) — loadDependencies()

**Root Cause:**
`loadDependencies()` has:
```groovy
def notify = null
if (fileExists('.majordomo/lib/notify.groovy')) {
    notify = load '.majordomo/lib/notify.groovy'
} else if (fileExists('lib/notify.groovy')) {
    notify = load 'lib/notify.groovy'
}
```

But logger and executor are hard-pinned:
```groovy
def logger   = load '.majordomo/lib/logger.groovy'
def executor = load '.majordomo/lib/executor.groovy'
```

This creates an inconsistent load-order that leaves the pipeline only partially portable and complicates testing when lib files are elsewhere.

**Recommendation:**
Apply the same fallback pattern to logger and executor, or document that they **must** be in `.majordomo/lib/`. Alternatively, move the entire lib/* check earlier and use a consistent base path for all three.

---

### [WARN] Proxy Configuration is Hardcoded in Pipeline Code

**Severity:** Medium  
**Impact:** Pipeline is not portable to other environments; proxy policy is embedded in source instead of config.

**Files:**
- [pipelines/MajordomoReview.CI.Jenkinsfile](../pipelines/MajordomoReview.CI.Jenkinsfile#L260) — PR Review stage environment vars

**Root Cause:**
```groovy
environment {
    HTTP_PROXY   = 'http://proxy.example.com:8080'
    HTTPS_PROXY  = 'http://proxy.example.com:8080'
    NO_PROXY     = 'localhost,127.0.0.1,.internal.example.com,packages.example.com'
}
```

These values are corporate-specific and should not be in source control.

**Recommendation:**
Move proxy config to orchestrator environment variables or control-tower secrets. Ensure defaults are provided so deployments without a corporate proxy don't fail when proxy env vars are unset.

**Resolution (2026-05-22):** Accepted as deployment-specific; override via orchestrator env or tower config.

---

### [WARN] Submodule Drift Guard Creates Long-Running CI Wait State

**Severity:** Medium  
**Impact:** Expensive for executor; noisy for operators; can block frequent webhook events.

**Files:**
- [pipelines/MajordomoReview.CI.Jenkinsfile](../pipelines/MajordomoReview.CI.Jenkinsfile#L555) — input timeout
- [pipelines/MajordomoReview.CI.Jenkinsfile](../pipelines/MajordomoReview.CI.Jenkinsfile#L574) — build() or GWT trigger

**Root Cause:**
When the `.majordomo` submodule is behind the `updates` branch, the pipeline pauses for up to 1 hour waiting for manual approval to trigger a fresh build:

```groovy
timeout(time: 1, unit: 'HOURS') {
    input(
        message: "Submodule ${submodulePath} is behind '${trackedBranch}'...",
        ok: 'Trigger Fresh Build'
    )
}
```

If the submodule updates branch moves frequently (e.g. daily), webhook builds will sit and eventually abort, consuming executor capacity and creating operator fatigue.

**Recommendation:**
Evaluate whether manual approval is necessary, or shorten the timeout to 5–10 minutes with auto-abort. Alternatively, auto-trigger fresh builds on submodule drift without waiting for human input (using GWT if it is configured).

**Resolution (2026-05-22):** Partially addressed. Timeout is now configurable via `cfg.submoduleDriftTimeoutMinutes` (defaults to 60 min). Operators can shorten it per-deployment. No further change required.

---

### [INFO] Bitbucket Origin Parsing Normalizes to HTTPS and Drops Transport Details

**Severity:** Low  
**Impact:** Portability issue for non-standard Bitbucket deployments or reverse-proxy setups.

**Files:**
- [stages/publish-pr-summary.groovy](../stages/publish-pr-summary.groovy#L10) — parseBitbucketOrigin()

**Root Cause:**
The parser converts all origins to `https://`:
```groovy
result.baseUrl = "https://${sshMatch[0][1]}"
result.baseUrl = "https://${httpsScmMatch[0][1]}"
result.baseUrl = "https://${httpsProjectsMatch[0][1]}"
```

This assumes Bitbucket is always served on standard HTTPS. For installations on non-443 ports or accessed via HTTP, the notification will fail silently or post to the wrong URL.

**Recommendation:**
Preserve the transport scheme from the origin URL (or allow override via config). Fail loudly if the URL cannot be resolved, rather than assuming HTTPS.

**Resolution (2026-05-22):** Accepted. Bitbucket Server in this environment is always on standard HTTPS. A clarifying comment was added to `parseBitbucketOrigin()` documenting the assumption and the override path via config.

---

## Linting & Validation

### Current Status
- **Groovy lint:** npm-groovy-lint not installed in workspace
- **Syntax check:** VS Code reported no syntax errors in Groovy files
- **Type checking:** No type annotations present; Pylance checks not run on Groovy

### Setup Recommendations

**Install Groovy lint globally:**
```powershell
npm install -g npm-groovy-lint
```

**Verify Java is available** (required by npm-groovy-lint):
```powershell
java -version
```

If Java is not installed or outdated:
```powershell
choco install openjdk25 --version 25.0.0.36
setx JAVA_HOME "C:\ProgramData\adoptium\jdk-25+36"
```

**Lint all Jenkinsfiles and stages:**
```powershell
npm-groovy-lint --path .majordomo/pipelines --files "**/*.Jenkinsfile" --no-insight
npm-groovy-lint --path .majordomo --files "**/*.groovy" --no-insight
```

**Auto-fix safe violations:**
```powershell
npm-groovy-lint --path .majordomo --files "**/*.groovy" --fix --no-insight
```

**Recommended CI integration:** Add linting to a pre-commit hook or CI stage to catch violations early.

---

## Action Items

### High Priority
- [x] Decouple early-stage Bitbucket notifications from `COPILOT_FULL_IMAGE` availability — fixed in `lib/notify.groovy`
- [x] Document and enforce remote-ref freshness assumptions for Static Analysis — accepted as low risk
- [x] Test pipeline behavior when `Ensure Images` fails (simulate registry outage) — covered by notify fix

### Medium Priority
- [x] Unify shared-library load pattern (logger, executor, notify) — fixed in `loadDependencies()`
- [x] Move proxy config to `.majordomo-config.groovy` — accepted; comment added documenting override path
- [x] Reduce submodule drift guard timeout or auto-trigger fresh builds — timeout now configurable via config

### Low Priority
- [x] Improve Bitbucket origin parsing to preserve transport scheme — accepted; comment added documenting HTTPS assumption
- [ ] Install npm-groovy-lint and add linting to CI
- [x] Document Bitbucket webhook configuration assumptions — accepted as low risk

---

## Testing Recommendations

1. **Simulate early-stage registry failure:** Mock `docker.withRegistry()` to fail before images are pulled and verify Bitbucket status is posted.
2. **Test Static Analysis with stale remote refs:** Manually reset `origin/master` to an older commit, run SA, and verify correct file set is scanned.
3. **Test lib fallback:** Move logger.groovy to `lib/` instead of `.majordomo/lib/` and verify pipeline still loads it.
4. **Test timeout behavior:** Introduce a 1-hour delay in the submodule drift stage and verify the build aborts cleanly.

---

## References

- [Control tower migration plan](../PLAN-control-tower-github-go.md)
- [Pipeline stages reference](04.1-pipeline-stages-reference.md)
- [PR Review agent](../../agents/pr-review.agent.md)
