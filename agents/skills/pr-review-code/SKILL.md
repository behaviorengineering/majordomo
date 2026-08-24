# Code Review Skill

This skill specializes the PR Review Agent for source code files.

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
| 1 | Security / Auth | `auth`, `jwt`, `token`, `secret`, `cred`, `encrypt`, `password`, `oauth`, `permission`, `acl`, `session`, `csrf`, `cors` |
| 2 | Input Handling | `valid`, `schema`, `sanitiz`, `deserializ`, `pars` |
| 3 | Core Logic | anything not matched by another tier |
| 4 | Tests | `test`, `spec`, `fixture` |

## §Review Criteria

Classify every finding:

- `[CRITICAL]` - Must fix before merge: logic errors that cause incorrect behaviour; security
  vulnerabilities (OWASP Top 10: injection, broken auth, IDOR, XSS, SSRF, insecure
  deserialisation, credential exposure, broken access control, security misconfiguration);
  data loss or corruption risk.

  **Credential exposure carve-out:** A string value that resolves to a credential at runtime
  via an external lookup is NOT a credential exposure - it is an indirection reference.
  This includes: Jenkins `credentialsId` values, `withCredentials` blocks, Vault paths,
  AWS Secrets Manager names, Kubernetes secret names, and **any map key whose string value
  is clearly a credential ID or reference name rather than an actual secret** (e.g.
  `package-registryCredentialsId: 'my-package-registry-cred'`, `githubCopilotCredentialsId: 'copilot-pat'`).

  **Apply this test:** If the string value looks like a credential store lookup key
  (short identifier, no spaces, does not resemble a token/password format), it is an
  indirection reference - do NOT flag it.

  **Only flag** findings where the string value IS an actual secret: a bearer token, an
  API key, a password, or a private key literal committed directly in source.

  MUST NOT ask the reviewer to "verify" indirection references - apply the carve-out
  definitively and do not raise the finding.
- `[WARN]` - Should fix: unclear naming that obscures intent; missing error handling for
  reachable failure paths; inconsistency with surrounding patterns.

Scope constraint: Report problems with the changes only. Do not summarise what the code does.
Do not comment on unchanged code unless it directly interacts with a changed section.

## §Report Label

```
Priority: Tier <N> - <tier name>
```

## §Static Analysis

Some input files contain a `=== STATIC ANALYSIS ===` section appended after the diff.
This section lists findings from automated linters (e.g. ruff, shellcheck, hadolint) that
already ran against the changed file before this review.

Rules:
- MUST NOT re-raise any finding that is already reported in the `=== STATIC ANALYSIS ===`
  section as a standalone `[WARN]` or `[INFO]` — it is already known.
- MAY reference a SA finding to provide additional `[CRITICAL]` context when the finding
  is directly related to a security or logic error you are raising independently.
  Format: `[CRITICAL] <your finding> (also flagged by SA: <sa line>)`
- If the `=== STATIC ANALYSIS ===` section is absent, review normally.

## §Blast Radius

After completing the initial review of all files, scan for downstream impact before writing the
summary. This step is mandatory - do not skip it even if no CRITICAL findings were raised.

For each changed file, identify which change types occurred (from the diff):

| Change type | What to follow |
|---|---|
| Function/method renamed or signature changed | `grep` codebase for the old name; read any callers found |
| Class renamed or base class changed | `grep` for the class name; read subclasses and instantiation sites |
| Auth / permission logic changed | `glob` for routes, middleware, decorators that reference the changed module |
| Config key added, removed, or renamed | `grep` for the key name across all non-excluded files |
| DB model field added/removed/renamed | `grep` for the field name; read migrations, serialisers, API schemas |
| Public API response shape changed | `grep` for the endpoint path or schema name; read consumers |

How to follow:
1. `grep` the repo for the symbol, key, or path that changed.
2. Read each file that references it (skip files already reviewed in Step 4).
3. Assess whether the caller/consumer is broken, at risk, or safe.
4. If a new problem is found in a referenced but unchanged file, append it to the changed file's
   report:
   ```
   [CRITICAL] Blast radius - `<caller-file>`: <description>
   [WARN] Blast radius - `<caller-file>`: <description>
   ```
5. If no callers found or all are safe: no action needed.

Scope limit: Follow at most 2 hops from each changed file. Stop when callers are unaffected.

## §Index Entry Format

```
- [<file>](./<slug>.md) - Tier <N>: <tier name>
```

List files in tier order (Tier 1 first, Tier 4 last).


