# Documentation Revision Summary (2026-05-07)

*Majordomo — repository operations for evolving software.*

**Scope:** Last revision pass to verify docs match current pipeline implementation  
**Status:** Complete — 3 docs updated, 1 new doc created

---

> **Historical — Jenkins retired.** This document predates removal of Jenkinsfiles, Groovy stages, and `.majordomo-config.groovy`. Behaviour below is a porting spec for the Go/GHA control tower — see [PLAN — Control Tower, GitHub Actions, and Go](../PLAN-control-tower-github-go.md).


## 📋 Changes Made

### 1. [README.md](../README.md)
**Section:** "How the Review Runs"  
**Issue:** Stated "seven stages" but actual pipeline has nine stages plus a guard wrapper  
**Update:** Listed all nine stages with brief descriptions; clarified Pipeline Guard's duplicate-detection role

### 2. [docs/04-pipeline-stages.md](./04-pipeline-stages.md)

#### Section A: Overview (lines 27–50)
**Issue:** Vague description; no mention of Pipeline Guard wrapper or Pipeline Snapshot Guard  
**Update:** 
- Clarified nine named stages inside Pipeline Guard
- Grouped stages by function (setup, guard, core work, post-review)
- Added note about the lock-based guard skipping duplicates

#### Section B: Pipeline Flow Diagram (lines 56–98)
**Issue:** ASCII diagram didn't show actual nesting; missing several stages
**Update:**
- Rewrote to show Pipeline Guard wrapper
- Added all nine stages with brief descriptions
- Added post block handlers (always, success, failure, unstable, aborted)
- Clarified relationships between stages

#### Section C: New Sections (lines 101–165)
**Issue:** Pipeline Guard and Pipeline Snapshot Guard undocumented  
**Update:** Added two new sections:
- **Pipeline Guard & Duplicate Detection** — explains duplicate-event detection, the per-branch lock, and the three scenarios that proceed
- **Pipeline Snapshot Guard** — explains submodule drift detection, the 1-hour approval gate, and the limitation/recommendation

### 3. [docs/08-pipeline-review-findings.md](./08-pipeline-review-findings.md) — **NEW FILE**
**Contents:** Full code review findings documenting:
- 4 WARN issues (early-failure status, static-analysis staleness, lib fallback inconsistency, submodule drift guard timeout)
- 1 INFO issue (Bitbucket URL normalization)
- Linting setup instructions
- 3-tier action items
- Testing recommendations

---

## ✅ Gaps Identified & Resolved

### Gap 1: Pipeline Guard Not Documented
**Was:** Not mentioned in high-level overview  
**Now:** Documented as a wrapper stage with explanation of why it exists

### Gap 2: Pipeline Snapshot Guard (Submodule Drift) Not Documented
**Was:** Code comment only; confusing 1-hour wait not explained anywhere  
**Now:** Full section explaining the drift detection, approval gate, and known limitation

### Gap 3: Stage Count Inaccurate
**Was:** README said "seven stages"  
**Now:** Updated to nine core stages + guard wrapper

### Gap 4: Notify Stage Missing
**Was:** No mention of "Notify: Build In Progress" stage  
**Now:** Listed as stage 5 in README and documented in flow diagram

### Gap 5: Static Analysis Staging
**Was:** Not called out as a separate stage in the overview  
**Now:** Listed as stage 6 with brief description

---

## 🔍 Known Issues Not Yet Fixed

These issues from the code review are documented in `08-pipeline-review-findings.md` but not yet addressed in code:

1. **Early-failure status reporting fragile** — Bitbucket notifications skip if image build fails (cert validation rabbit hole)
2. **Static-analysis diff staleness** — Depends on undocumented assumptions about remote-ref freshness

**Recommendation:** Issue #2 has a clearer fix than #1 (which hit cert validation dead-ends).

---

## ✅ Issues Fixed (2026-05-19)

3. **Shared-library fallback inconsistent** — `logger.groovy` and `executor.groovy` now use the same `fileExists` → fallback → `error` pattern as `notify.groovy`; hard crash replaced with a clear error message
4. **Submodule drift guard timeout** — Timeout moved from hardcoded 1 hour to `cfg.submoduleDriftTimeoutMinutes` (default 60); configurable via `.majordomo-config.groovy`

---

## 📚 Related Files

- [08-pipeline-review-findings.md](./08-pipeline-review-findings.md) — Full review findings
- [PROPOSAL-decouple-bitbucket-notifications.md](./PROPOSAL-decouple-bitbucket-notifications.md) — Exploration of early-failure status fix

---

## ✨ Next Steps

1. **Review the changes** — Check if the pipeline flow diagram and stage descriptions match your mental model
2. **Update media/pipeline.png** — The ASCII flow diagram may not match the current UI; consider updating the screenshot if it's used elsewhere
3. **Consider the remaining finding** — Static Analysis diff staleness (#2) has a clearer fix than the cert validation issue (#1)
4. **Document Bitbucket webhook setup** — The docs should clarify the exact webhook event configuration needed to guarantee fresh remote refs

---

**Reviewed by:** Code review pass (2026-05-07)  
**Files changed:** 3 (README.md, 04-pipeline-stages.md, added 08-pipeline-review-findings.md and PROPOSAL file)
