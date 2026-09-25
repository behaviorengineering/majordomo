# PR Blast Radius Skill

This skill specializes the PR Review Agent to produce a blast radius report for a PR.
It answers one question: what in the live repo is still coupled to the changed files,
and does it matter?

This skill runs as `mode:summary` — it does not produce per-file finding reports.

## Table of Contents
1. [§Prioritization](#prioritization)
2. [§Data Sources](#data-sources)
3. [§Verification Protocol](#verification-protocol)
4. [§Review Criteria](#review-criteria)
5. [§Output](#output)
6. [§Source Lookup](#source-lookup)
7. [§Prose Review](#prose-review)
8. [§Blast Radius](#blast-radius)
9. [§Report Label](#report-label)
10. [§Index Entry Format](#index-entry-format)

## §Prioritization

N/A — this skill reads the manifest and grepping the live repo. No file tier grouping.

## §Data Sources

The manifest for this skill contains two pre-computed dependency keys:

**`dep_clusters`**: A list of lists. Each inner list is a group of changed files connected
by direct imports. Only multi-file clusters are present (singletons are omitted).
Use this to identify which changed files are tightly coupled and share blast radius.

**`reverse_deps`**: A dict mapping each changed file to a list of unchanged repo files
that import it directly. Structure: `{changed_file: [importer_path, ...]}`.
This is the primary source for seam point identification.

Both keys may be absent or empty if no dependency data was available at staging time.
If both are absent, write the `## Live Seam Points` section with a statement that no
dependency data was available and manual inspection is recommended.

## §Verification Protocol

The manifest `reverse_deps` data was computed at staging time. The live repo may have
changed since then — an importer may have already been updated in this PR or a follow-up.

For each entry in `reverse_deps`, grep the live repo to confirm the import is still present:

```
grep -rn "<changed_file_basename_without_extension>" <importer_path>
```

Rules:
- MUST verify every `reverse_deps` entry before including it as a seam point
- If the import is confirmed present: include as a live seam point
- If the import is not found: skip the entry — it has already been resolved
- Limit: no cap on grep calls for verification. Use as many as needed to verify all entries.
- For scope claims (§Scope Verification section): limit 5 additional grep calls beyond
  the verification passes above

## §Review Criteria

This skill MUST NOT produce `[CRITICAL]`, `[WARN]`, or `[INFO]` findings.
This skill MUST NOT evaluate code correctness, security, or style.

Each seam point entry MUST:
- Name the specific files involved (changed file and confirmed importer)
- State what the seam means in context (what the changed file was, what replaced it if anything)
- State what breaks or stays live if the seam is not resolved
- End with one actionable confirmation step for the reviewer

Do not include seam points where the consequence is not meaningful (e.g. a test file
that imports a changed helper — that is expected and low-risk).

Do not use em dashes as connectors. Use a period or colon.

## §Output

Read `<skill_dir>/review_timestamp.txt` to get the current Sydney timestamp. Use its content as the value for the `Reviewed at` field in the template.

Do NOT run a shell command — the dispatcher pre-wrote this file before the session started.

Load the output template from the skill directory (available via `--add-dir`):
```
<skill_dir>/templates/blast-radius.md
```

The skill directory is the directory containing this `SKILL.md` file.

Fill every `<!-- FILL: ... -->` slot according to its inline instructions and the rules
in this skill. Remove all `<!-- FILL: ... -->` comments from the final output.
Do not add sections not present in the template. Do not remove sections from the template
unless the slot instruction explicitly permits omission.

Write the completed file to: `<output>/blast-radius.md`

## §Source Lookup

The live repo is available via `--add-dir` (repo root is passed as an additional context
directory in summary mode). Use `grep` to verify imports as described in §Verification Protocol.

Rules:
- MUST NOT read full file contents from the repo — grep results are sufficient
- MAY read a specific line range if a grep match needs context to interpret
- Do not fabricate import paths — derive them from grep results only



## §Blast Radius

N/A — this skill IS the blast radius skill.

## §Report Label

N/A — no per-file reports in this skill.

## §Index Entry Format

N/A — no index in this skill.
