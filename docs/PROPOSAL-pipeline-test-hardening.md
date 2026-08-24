# PROPOSAL: Pipeline Test Hardening Roadmap

*Majordomo — repository operations for evolving software.*

> **Historical — Jenkins retired.** This document predates removal of Jenkinsfiles, Groovy stages, and `.majordomo-config.groovy`. Behaviour below is a porting spec for the Go/GHA control tower — see [PLAN — Control Tower, GitHub Actions, and Go](../PLAN-control-tower-github-go.md).


## Purpose

Capture the next high-value tests that should be added to reduce regressions across staging, cache, push, publish, and reporting flows.

## Proposed Test Targets

1. git-diff-prep.py git timeout/failure handling
- Assert deterministic exit behavior when git commands timeout or return non-zero.
- Protects staging from silent partial outputs.

2. git-diff-prep.py disconnected history behavior
- Assert no-merge-base and empty-diff paths keep their fail/skip contracts.
- Prevents ambiguous outcomes when branch ancestry is broken.

3. review-cache.py round-trip contract (store -> precheck -> lookup)
- Assert schema compatibility across commands and hit/mismatch behavior.
- Prevents cache drift where one command writes data another cannot consume.

4. push-to-cache.py retry hard-failure path
- Assert stale-info retry failure and non-stale push failure both return non-zero.
- Ensures cache push errors are visible and never silently ignored.

5. publish-pr-summary.py mode matrix harness
- Assert auto, description, comment behavior with empty-body and env variants.
- Protects PR publish semantics during refactors.

6. review-to-junit.py aggregate count stability
- Assert XML tests/failures/skipped counters for mixed CRITICAL/WARN/INFO findings.
- Prevents reporting regressions in Jenkins test trends.

## Suggested Execution Order

1. push-to-cache.py retry hard-failure path
2. review-cache.py round-trip contract
3. git-diff-prep.py failure/disconnected-history handling
4. publish-pr-summary.py mode matrix
5. review-to-junit.py aggregate counters

## Acceptance Criteria

- New tests run under the existing scripts test suite.
- No test requires network calls.
- All behavior assertions are deterministic and isolated with mocks/fixtures.
- Pipeline scripts test suite remains green after each increment.
