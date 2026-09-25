# Documentation Consistency Skill

This skill specializes the PR Review Agent for cross-corpus documentation consistency.
It does not re-check prose quality or diff correctness — those belong to `pr-review-conf`.
It answers one question: does this change create or expose inconsistencies with the
surrounding documentation corpus?

## Table of Contents
1. [§Prioritization](#prioritization)
2. [§Data Sources](#data-sources)
3. [§Corpus Context Loading](#corpus-context-loading)
4. [§Review Criteria](#review-criteria)
5. [§Consistency Checks](#consistency-checks)
6. [§Report Label](#report-label)
7. [§Index Entry Format](#index-entry-format)
8. [§Prose Review](#prose-review)

## §Prioritization

Review files in the order they appear in the manifest. No tier grouping.
Only `.md` files are in scope. Skip any non-markdown file silently.

## §Data Sources

The manifest for this skill contains three pre-computed doc-context keys:

**`doc_clusters`**: A list of lists. Each inner list is a group of changed `.md` files
connected by direct markdown links. Only multi-file clusters are present (singletons are
omitted). Use this to identify which changed docs are explicitly linked and form a
navigational unit.

**`reverse_links`**: A dict mapping each changed `.md` file to a list of unchanged repo
files that link to it directly. Structure: `{changed_file: [linker_path, ...]}`.
Unchanged docs that link to the changed file are potential blast radius targets.

**`corpus-index.json`**: A file written alongside `manifest.json` in the batch directory.
It is a JSON array, one entry per `.md` file in the repo. Each entry has:

```json
{
  "file": "docs/05-file-orchestration.md",
  "title": "File Orchestration and Batching",
  "headings": ["Overview", "Staging", "Dependency Clustering"],
  "key_terms": ["batch_size", "dep_clusters", "staging_dir"],
  "links_out": ["docs/05.1-staging-and-classification.md"]
}
```

If `corpus-index.json` is absent, skip §Corpus Context Loading and note in the report
that semantic context was unavailable.

## §Corpus Context Loading

Execute this section once per changed file, before writing any findings for that file.

**Step 1 — Extract changed-file terms.**
Read the diff for the current file (already in the `.txt` slug). Collect all:
- Terms introduced or modified in the diff (added `+` lines): backtick-quoted identifiers,
  bold terms, heading text.
- File paths referenced in the diff.
- Any term that appears in `doc_clusters` entries alongside this file.

Call this set `changed_terms`.

**Step 2 — Score corpus entries.**
Read `corpus-index.json`. For each entry (excluding the current file), compute a
relevance score:

| Signal | Score |
|---|---|
| Entry is in `doc_clusters` alongside this file | +3 |
| Entry is in `reverse_links` for this file | +3 |
| Entry shares ≥ 1 term with `changed_terms` (via `key_terms`) | +2 per shared term (max +6) |
| Entry is in the same directory as this file | +1 |
| Entry title contains a word from `changed_terms` | +1 |

**Step 3 — Load top related docs.**
Sort entries by score descending. Load the full content of the top 5 entries with score ≥ 2
using `view`. If fewer than 5 entries meet the threshold, load all that do.

If no entries score ≥ 2, the file has no detectable related corpus. Write a single
`[INFO]` finding: "No related docs detected in corpus — consistency check skipped."
Then stop processing this file.

**Step 4 — Proceed to §Consistency Checks** with the loaded docs as context.

## §Review Criteria

Classify every finding:

- `[CRITICAL]` — Must fix before merge: direct factual contradiction between the changed doc
  and a loaded related doc covering the same concept; a changed heading or anchor that is still
  referenced by a `reverse_links` doc; a definition in the changed doc that directly conflicts
  with a definition in a related doc.
- `[WARN]` — Should fix: same concept referred to by different names across the changed doc
  and related docs; a prerequisite concept introduced in the changed doc but removed or renamed
  in a doc it depends on; a term newly defined in this doc that already has a different
  definition elsewhere in the corpus.
- `[INFO]` — Consider: naming variation that is not strictly wrong but reduces consistency;
  a related doc that would benefit from referencing the change but does not; structural
  parallels that could be aligned.

Scope constraint: all findings MUST trace to a specific comparison between the changed doc
and a named related doc. NEVER report intra-file issues — those belong to `pr-review-conf`.
NEVER summarise content. NEVER report findings about unchanged sections of the changed file
unless a loaded related doc contradicts them.

## §Consistency Checks

For each changed file, after completing §Corpus Context Loading, run the checks below
against the loaded related docs.

**Check 1 — Terminology drift**
For each new or modified term in `changed_terms`:
1. `grep` the loaded related docs for the same term and synonyms.
2. If a related doc uses the same term with a materially different meaning, flag `[WARN]`.
3. If a related doc defines the same concept under a completely different name, flag `[INFO]`.

**Check 2 — Cross-doc contradictions**
For each factual claim introduced or modified in the diff (e.g. "X always runs before Y",
"Z is required", "the default is N"):
1. `grep` the loaded related docs for the subject of the claim.
2. If a related doc makes a conflicting claim about the same subject, flag `[CRITICAL]`.
3. If a related doc makes a claim that appears to be outdated given the change, flag `[WARN]`.

**Check 3 — Anchor and link integrity**
For each heading added, renamed, or removed in the diff:
1. Check `reverse_links` entries for the current file.
2. For each reverse-linker, `grep` it for `#<old-heading-slug>` or `#<new-heading-slug>`.
3. If a reverse-linker references the old heading slug that no longer exists, flag `[CRITICAL]`.

For each outgoing link in the changed doc (`[text](path#anchor)`):
1. `glob` the target path to confirm it exists.
2. `grep` the target file for the anchor heading.
3. If the anchor heading does not exist in the target, flag `[CRITICAL]`.

**Check 4 — Prerequisite context**
For each concept the changed doc now depends on (terms it uses but does not define):
1. Check whether that concept is defined or introduced in any loaded related doc.
2. If the related doc that previously defined it was also changed in this PR (present in
   `doc_clusters`), confirm the definition survived the change.
3. If the definition was removed or renamed and this doc still uses the old term, flag `[WARN]`.

**Check 5 — Reverse-linker currency**
For each file in `reverse_links[current_file]`:
1. Load the reverse-linker using `view`.
2. Check that any description it gives of the current file is still accurate after the change.
3. If the reverse-linker's description contradicts the changed doc's new content, flag `[WARN]`.
4. Limit to the first 3 reverse-linkers if there are more than 3.

## §Report Label

```
Corpus: <N> related doc(s) loaded
```

## §Index Entry Format

```
- [<file>](./<slug>.md) - Consistency: <N> finding(s)
```

## §Prose Review

- Use exact file names when citing a related doc: `` `docs/05-file-orchestration.md` ``.
- Quote the specific conflicting text from both the changed doc and the related doc.
  Do not paraphrase — quote the actual lines.
- Each finding MUST identify: the changed file, the related file, the specific conflict.
- Do not write findings longer than 3 sentences. If more context is needed, quote it.
- Do not use hedging language ("might", "could", "possibly"). State the inconsistency directly.
