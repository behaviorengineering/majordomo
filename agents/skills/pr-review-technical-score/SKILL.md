# PR Technical Review Score Skill

This skill scores a completed `tech-review.md` against the structural and prose rules
defined by the `pr-review-technical` skill. It is invoked by the orchestrate tech loop after
each generation pass. It does NOT rewrite the review — it only evaluates and records findings.

This skill runs as `mode:tech-score` — it produces a single `tech-score.md` file.

## Table of Contents
1. [§Input](#input)
2. [§Rubric](#rubric)
3. [§Scoring](#scoring)
4. [§Output](#output)

## §Input

Read `tech-review.md` from the output directory supplied in the prompt (`output:<dir>`).

Do not read any other files.

## §Rubric

Score the review against the following 8 items. Each item is worth the stated points.
A PASS awards full points. A FAIL awards 0 for that item.

### Item 1 — H2 structure (2 points)

PASS if the review contains exactly these three H2 headings in this order, with no extras:
1. `## ⚠️ Correctness Risks`
2. `## 🔍 Verification Checklist`
3. `## 🧪 Test Coverage Gaps`

FAIL if any heading is missing, renamed, reordered, or if any extra H2 appears.

Evidence: list the actual H2 headings found in the file.

### Item 2 — Blank lines between fields (3 points)

PASS if every risk entry in `## ⚠️ Correctness Risks` has a blank line separating each
consecutive pair of labeled fields: between `**Does:**` and `**Trigger:**`, between
`**Trigger:**` and `**Consequence:**`, and between `**Consequence:**` and `**Confirm:**`.

A blank line means a completely empty line between the two field lines — no spaces, no content.

FAIL if any risk entry has two consecutive field lines without a blank line between them.
Quote the H3 heading and the two consecutive field lines that are missing the blank line.

Evidence: quote the offending field pair, or confirm all entries pass.

### Item 3 — Exactly four fields per risk entry, in order (2 points)

PASS if every H3 in `## ⚠️ Correctness Risks` is followed by exactly four labeled fields
in this order: `**Does:**`, `**Trigger:**`, `**Consequence:**`, `**Confirm:**`.

FAIL if any entry is missing a field, has an extra field, or has fields in the wrong order.
Name the H3 heading and state what is wrong.

Evidence: list the fields found under each H3, or confirm all entries pass.

### Item 4 — Confirm is a direct yes/no question (2 points)

PASS if every `**Confirm:**` field ends with `?` and begins with an interrogative word:
Does, Is, Are, Has, Will, Did, Can, or Would.

FAIL if any Confirm field does not end with `?` or does not start with an interrogative word.
Quote the offending field value.

Evidence: quote any offending Confirm field, or confirm all pass.

### Item 5 — H3 headings are declarative statements (2 points)

PASS if every H3 in `## ⚠️ Correctness Risks` does not end with `?` and is a declarative
statement of the failure mode (not a question).

FAIL if any H3 ends with `?` or is phrased as a question. Quote the offending H3.

Evidence: quote any offending H3, or confirm all pass.

### Item 6 — Verification checklist ends with a skip entry (1 point)

PASS if the last bullet in `## 🔍 Verification Checklist` begins with `Skip:`.

FAIL if the last bullet is not a skip entry. Quote the last bullet as written.

Evidence: quote the last bullet, or confirm it is a skip entry.

### Item 7 — No em dashes as connectors (1 point)

PASS if no sentence in the review uses an em dash (`—`) as a connector between two clauses
or phrases. A title that appends a clarification (e.g. `` `Symbol` — short label ``) is
acceptable. A connector joins a subject or clause to a continuation.

FAIL if any em dash connector is found. Quote the sentence.

Evidence: quote any offending sentence, or confirm none found.

### Item 8 — No prohibited generic phrases (1 point)

PASS if the review contains none of the following phrases (case-insensitive):
`may cause issues`, `could be a problem`, `needs review`, `might break`,
`better error handling`, `more robust`, `cleaner code`, `improved maintainability`.

FAIL if any prohibited phrase appears. Quote the sentence containing it.

Evidence: quote any offending sentence, or confirm none found.

## §Scoring

Sum the points from all PASSed items. Maximum is 14.

Write the score on its own line in the exact format: `SCORE: N`

where `N` is the integer total (0–14). No trailing text on that line.

## §Output

Load the output template from the skill directory:
```
<skill_dir>/templates/tech-score.md
```

The skill directory is the directory containing this `SKILL.md` file.

Fill every `<!-- FILL: ... -->` slot. Remove all `<!-- FILL: ... -->` comments from the output.

Write the completed file to: `<output>/tech-score.md`

Do not write any other files.
