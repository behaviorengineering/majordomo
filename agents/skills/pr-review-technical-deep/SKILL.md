# PR Technical Deep Review Skill

This skill runs a focused second pass on files cited in `tech-review.md`. It has full file
content for each cited file and must decide for each existing risk entry: CLOSED (mitigation
found) or CONFIRMED (risk stands). Confirmed risks get a concrete remediation.

This skill runs as `mode:technical-deep` — one agent call per cited file, all in parallel.
It does not produce per-file finding reports. It produces one `<slug>.md` per file.

## Table of Contents
1. [§Input Format](#input-format)
2. [§Review Criteria](#review-criteria)
3. [§Pre-Analysis](#pre-analysis)
4. [§Decision Protocol](#decision-protocol)
5. [§Section Types](#section-types)
6. [§Output](#output)
7. [§Blast Radius](#blast-radius)
8. [§Index Entry Format](#index-entry-format)
9. [§Prose Review](#prose-review)

## §Input Format

Each staging input file contains two sections separated by `=== FULL FILE ===`:

```
=== RISKS FROM TECH-REVIEW ===
<copied risk entries from tech-review.md for this file only>

=== FULL FILE: <path> ===
<complete current file content>

=== DIFF ===
<diff for this file>
```

For chunked files, the input contains one chunk of the full file. The chunk header states:
`=== FULL FILE CHUNK <N> of <M>: <path> ===`

Read all chunks before making any CONFIRMED/CLOSED decision. The dispatcher writes one
manifest entry per chunk; all chunks for a file accumulate into one `<slug>.md`.

## §Review Criteria

This skill MUST NOT produce `[CRITICAL]`, `[WARN]`, or `[INFO]` findings.
This skill MUST NOT raise new risks. Its scope is limited to the risks already in the input.
This skill MUST NOT evaluate style, formatting, or import conventions.

For each risk entry in the input, produce exactly one of:

**CLOSED** — the full file contains a mitigation that directly addresses the risk.
- State which line or symbol provides the mitigation (be specific: function name, line range).
- One sentence only.

**CONFIRMED** — the full file does not contain a mitigation for this risk.
- Keep the original `Does:` / `Trigger:` / `Consequence:` fields verbatim.
- Replace `Confirm:` with `**Remediation:**` — a concrete, actionable fix (not a question).
- Remediation must name the specific change: which function, what guard condition, what
  exception handler, which test case. NEVER write "consider adding error handling".

## §Pre-Analysis

Before writing any output, complete this analysis in working memory.

**Step 1 — Map risks to symbols.** For each risk entry, extract the symbol named in `Does:`.
Locate that symbol in the full file content.

**Step 2 — Check for mitigations.** For each risk, look for:
- Try/finally or context manager wrapping the cited operation
- Guard conditions (len check, None check, min_length validator) on the cited path
- Locks or thread-local patterns around the cited shared state
- Import ordering guarantees or `__init__.py` constraints that enforce load order
- Existing unit tests that exercise the exact failure path

**Step 3 — Classify.** Mark each risk CLOSED or CONFIRMED based on Step 2 findings.

**Step 4 — Write remediations.** For each CONFIRMED risk, draft the remediation before
writing output. The remediation must be implementable without further clarification.

## §Decision Protocol

```
For each risk entry in the input:

  Candidate: <H3 heading text>
  Symbol located in full file: YES / NO
  Mitigation found: YES (cite line/function) / NO
  Decision: CLOSED | CONFIRMED

  If CLOSED:
    Evidence: <function name or line range containing the mitigation>
    Output: one-line closed entry (see §Section Types)

  If CONFIRMED:
    Remediation field: <concrete fix — function, guard, test>
    Output: full risk block with Remediation replacing Confirm (see §Section Types)
```

## §Section Types

**CLOSED entry:**
```markdown
### ~~<original H3 heading>~~

**Closed:** mitigation found — `<symbol or line range>` <one sentence describing what it does>.
```

**CONFIRMED entry:**
```markdown
### <original H3 heading>

**Does:** <verbatim from input>

**Trigger:** <verbatim from input>

**Consequence:** <verbatim from input>

**Remediation:** <concrete fix — name the function, guard, or test to add/change>
```

Separate each entry with `---`.

Order: CONFIRMED entries first (descending severity as originally ordered), then CLOSED entries.

## §Output

Read `<skill_dir>/review_timestamp.txt` to get the current Sydney timestamp.

Load the output template from the skill directory:
```
<skill_dir>/templates/tech-review-deep.md
```

Fill every `<!-- FILL: ... -->` slot in the template. Remove all `<!-- FILL: ... -->` comments
from the final output. Do not add sections not present in the template.

Write output to `<output>/<slug>.md` — one file per cited source file.

## §Blast Radius

N/A

## §Index Entry Format

N/A — this skill does not produce an `index.md`.

## §Prose Review

N/A — this skill does not go through the prose rewrite pass.
