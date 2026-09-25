# Tests Review Skill

This skill specializes the PR Review Agent for test files.

## Table of Contents
1. [§Prioritization](#prioritization)
2. [§Review Criteria](#review-criteria)
3. [§Report Label](#report-label)
4. [§Static Analysis](#static-analysis)
5. [§Blast Radius](#blast-radius)
6. [§Index Entry Format](#index-entry-format)
7. [§Prose Review](#prose-review)

## §Prioritization

Group the unique `file` values from `reviewable` into the tiers below. Review in tier order.
Within each tier, preserve the original manifest order.

A file matches the FIRST tier whose keywords appear in its path.

| Tier | Name | Path matches (substring) |
|------|------|--------------------------|
| 1 | Security Tests | `auth`, `token`, `cred`, `secret`, `encrypt`, `permission`, `oauth`, `session` |
| 2 | Integration / E2E | `integration`, `integ`, `e2e`, `end_to_end`, `functional` |
| 3 | Unit Tests | anything not matched by another tier |

## §Review Criteria

Classify every finding:

- `[CRITICAL]` - Must fix before merge: assertion that can never fail regardless of implementation
  behaviour (vacuous test); hardcoded credentials or secrets in test data or fixtures; test that
  mutates shared global state without cleanup, leaving subsequent tests in an undefined state;
  mocked return value that directly contradicts the real API contract, hiding a broken integration.

- `[WARN]` - Should fix: test exercises a code path but asserts nothing; test name does not
  describe the scenario or expected outcome; over-mocking (mocking the system under test itself);
  duplicate test coverage with no distinguishing scenario; non-deterministic test relying on
  ordering, wall-clock time, or randomness without a fixed seed.

Scope constraint: Report problems with the changes only. Do not summarise what the test does.
Do not comment on unchanged tests unless they directly interact with a changed fixture or helper.

## §Report Label

```
Priority: Tier <N> - <tier name>
```

## §Static Analysis

Some input files contain a `=== STATIC ANALYSIS ===` section appended after the diff.
This section lists findings from automated linters that already ran against the changed
test file before this review.

Rules:
- MUST NOT re-raise any finding already reported in the `=== STATIC ANALYSIS ===` section
  as a standalone `[WARN]` or `[INFO]` — it is already known.
- MAY reference a SA finding to provide additional `[CRITICAL]` context when the finding
  is directly related to a vacuous assertion or contract violation you are raising independently.
  Format: `[CRITICAL] <your finding> (also flagged by SA: <sa line>)`
- If the `=== STATIC ANALYSIS ===` section is absent, review normally.

## §Blast Radius

Run blast radius only for **shared fixtures and test helpers**. Skip for individual test files.

A "shared fixture or helper" is any changed file that matches one of: `conftest`, `fixture`,
`helper`, `util`, `factory`, `builder` in its path.

For each changed shared fixture or helper, apply the checks below.
Run at most 1 hop from the changed file. Stop when all referencing files are assessed.

| Change type | What to follow |
|---|---|
| Fixture renamed or signature changed | `grep` test files for the old fixture name; flag any test that still uses it as `[CRITICAL] Blast radius` |
| Fixture return value or shape changed | `grep` test files for the fixture name; read each consumer; flag as `[WARN] Blast radius` if the consumer's assertion relies on the old shape |
| Shared helper removed or renamed | `grep` all test files for the old name; flag any file that still references it as `[CRITICAL] Blast radius` |

How to follow:
1. `grep` the test tree for the fixture or helper name that changed.
2. Read each file that references it (skip files already reviewed in Step 4).
3. Assess whether the reference is broken or safe.
4. If a problem is found in a referenced but unchanged file, append it to the changed file's report:
   ```
   [CRITICAL] Blast radius - `<other-file>`: <description>
   [WARN] Blast radius - `<other-file>`: <description>
   ```
5. If no references found or all are safe: no action needed.

## §Index Entry Format

```
- [<file>](./<slug>.md) - Tier <N>: <tier name>
```

List files in tier order (Tier 1 first, Tier 3 last).


