# Documentation Review Skill

This skill specializes the PR Review Agent for documentation and configuration files.

## Table of Contents
1. [§Prioritization](#prioritization)
2. [§Review Criteria](#review-criteria)
3. [§Report Label](#report-label)
4. [§Blast Radius](#blast-radius)
5. [§Index Entry Format](#index-entry-format)
6. [§Prose Review](#prose-review)

## §Prioritization

Review files in the group order below. Within each group, preserve the original manifest order.

| Group | Criteria |
|-------|----------|
| 1 | Configuration files (`.yml`, `.yaml`, `.toml`, `.json`, `.env`) |
| 2 | Reference documentation (API docs, changelogs, architecture docs) |
| 3 | All other markdown and text files |

## §Review Criteria

Classify every finding:

- `[CRITICAL]` - Must fix before merge: factually incorrect instructions that will cause user
  failure; broken internal links or anchors; config values that reference non-existent resources
  or paths; security-sensitive values (tokens, passwords) committed in plain text; instructions
  that contradict the actual implementation.
- `[WARN]` - Should fix: ambiguous steps; missing prerequisites; inconsistent terminology across
  the document; outdated references to renamed files, commands, or config keys; code blocks
  with wrong language fencing; missing or misleading examples.
- `[INFO]` - Consider: passive voice reducing clarity; overly long sentences; structural
  suggestions; minor wording improvements.

Scope constraint: Report problems with the changes only. Do not summarise what the content says.
Do not comment on unchanged sections unless they directly contradict a changed section.

If a doc change references code symbols that may have changed, flag it as `[WARN]` and note
that code review is handled by the `pr-review-code` skill.

## §Report Label

```
Group: <N> - <group name>
```

## §Blast Radius

Run blast radius only for **Group 1 configuration files**. Skip for Group 2 and Group 3.

For each changed config key or value in a Group 1 file, apply the checks below.
Run at most 1 hop from the changed file. Stop when all referenced files are assessed.

| Change type | What to follow |
|---|---|
| Config key added, removed, or renamed | `grep` all non-excluded files for the old key name; flag any file that still uses it as `[WARN] Blast radius` |
| Config value referencing an external ID (connection ID, project key, server name) | `grep` all config files (`.yml`, `.yaml`, `.toml`, `.json`, `.properties`) for the same ID; if the value appears nowhere else or conflicts with another file's value, flag as `[CRITICAL] Blast radius` |
| Internal file path or resource reference changed | `glob` for the referenced path; if it does not exist in the repo, flag as `[CRITICAL] Blast radius` |

How to follow:
1. `grep` or `glob` the repo for the symbol, key, or path that changed.
2. Read each file that references it (skip files already reviewed in Step 4).
3. Assess whether the reference is broken, inconsistent, or safe.
4. If a problem is found in a referenced but unchanged file, append it to the changed file's report:
   ```
   [CRITICAL] Blast radius - `<other-file>`: <description>
   [WARN] Blast radius - `<other-file>`: <description>
   ```
5. If no references found or all are consistent: no action needed.

## §Index Entry Format

```
- [<file>](./<slug>.md) - Group <N>: <group name>
```

List files in group order (Group 1 first, Group 3 last).


