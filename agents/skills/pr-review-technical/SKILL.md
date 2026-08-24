# PR Technical Review Skill

This skill specializes the PR Review Agent to produce a single technical review document
focused on code correctness risks, architectural hazards, and specific verification questions
for the engineer doing the deep diff review.

This skill runs as `mode:technical` — it does not produce per-file finding reports.

## Table of Contents
1. [§Prioritization](#prioritization)
2. [§Feedback Integration](#feedback-integration)
3. [§Review Criteria](#review-criteria)
4. [§Pre-Synthesis Enumeration](#pre-synthesis-enumeration)
5. [§Pre-Writing Analysis](#pre-writing-analysis)
6. [§Section Types](#section-types)
7. [§Output](#output)
8. [§Blast Radius](#blast-radius)
9. [§Index Entry Format](#index-entry-format)
10. [§Prose Review](#prose-review)

## §Prioritization

N/A — this skill reads all files in the manifest in manifest order. No tier grouping.

## §Feedback Integration

Before beginning any analysis, check whether `tech_feedback.md` exists in the staging
directory (the same directory that contains `manifest.json`).

If `tech_feedback.md` is absent, proceed normally with no changes to behaviour.

If `tech_feedback.md` is present, read it in full and apply its FAIL corrections silently
during `§Pre-Writing Analysis`.

## §Review Criteria

This skill MUST NOT produce `[CRITICAL]`, `[WARN]`, or `[INFO]` findings.
This skill MUST NOT evaluate style, formatting, or import path conventions.

All content is correctness risk analysis only. Each risk entry must:
- Name the specific file and symbol (class, method, or variable)
- State the failure mode (what goes wrong and under what condition)
- Pose the verification question the reviewer must answer by reading the diff

Do not raise risks that the diff itself already resolves. Do not raise risks that are
hypothetical without a concrete trigger path visible in the diff.

Do not use generic phrases. Prohibited: "may cause issues", "could be a problem",
"needs review", "might break". Required: name the mechanism and the specific failure mode.

## §Pre-Synthesis Enumeration

Before writing any prose, enumerate all files in the manifest and identify:

1. Files containing control-flow logic: controllers, guards, services, session handlers.
   These are candidates for loop termination, retry exhaustion, and race condition risks.

2. Files containing closures, lambdas, or deferred callables. These are candidates for
   late-binding risks.

3. Files containing shared mutable state (locks, class-level registries, `__subclasses__()`).
   These are candidates for dynamic dispatch and concurrency risks.

4. Test files covering the above. Note which failure modes are covered and which are not.

This enumeration is working memory only — do not emit it in the output.

## §Pre-Writing Analysis

Before filling any template slot, complete this analysis in working memory.

**Step 1 — Control flow risks:** For each controller, guard, or service, identify:
- Any loop that could fail to terminate (missing visited-set, unbounded retry, interrupt
  that re-triggers recovery indefinitely)
- Any retry limit that may be insufficient given the failure modes described in the diff
- Any lock or coroutine pattern where state set outside the lock could be observed in a
  partially-updated state by another thread

**Step 2 — Dynamic dispatch risks:** For any pattern using `__subclasses__()`, class
registries, or plugin-style discovery:
- What must be true at import time for the pattern to work correctly?
- What happens if a new subclass is added but not imported before the dispatch runs?

**Step 3 — Closure and late-binding risks:** For any lambda or closure defined in a loop
or factory function:
- Does it capture variables via default arguments (`lambda e, v=var: ...`) or by reference
  (`lambda e: var`)?
- If by reference, what value does `var` hold at call time vs at definition time?

**Step 4 — Concurrency risks:** For any lock acquisition pattern:
- Is mutable shared state set inside or outside the lock?
- Could an exception between state mutation and lock release leave state in an
  inconsistent condition for the next caller?

**Step 5 — Test coverage gaps:** For each risk identified in Steps 1-4:
- Is there a unit test that exercises this exact failure path?
- If not, note the gap.

## §Section Types

**`## ⚠️ Correctness Risks`**
One H3 per risk, in descending order of consequence (most severe first).

H3 heading: a declarative statement of the failure mode only. Name the symbol (class or method). Do not include the file path in the H3 — the file belongs in `Does:`.
Do not use a question mark in any H3 heading.

Body: exactly four labeled fields, each on its own line, separated by a blank line. One sentence per field. No paragraph prose. No multi-clause chains.

```
**Does:** <file and symbol — what the code does, one clause, active voice>

**Trigger:** <the specific condition that causes the failure>

**Consequence:** <what breaks, one mechanism, one sentence>

**Confirm:** <one yes/no question beginning with Does, Is, Are, Has, Will, Did, Can, or Would>
```

Confirm is one question only. If two scenarios require separate questions, raise two separate H3 entries.

Separate each risk entry with a `---` horizontal rule.

Do not include risks that require more than 2 hops of inference from the diff.

**`## 🔍 Verification Checklist`**
One bullet per file that requires a targeted read. Each bullet names the file (full path
from manifest) and the single yes/no question the reviewer must answer by reading it.
Order: highest-consequence questions first.
Last bullet is always a skip entry: "Skip: `<path>` — <one-line reason>."

**`## 🧪 Test Coverage Gaps`**
One bullet per failure mode from `## ⚠️ Correctness Risks` that has no corresponding
unit test. Format: "No test for `<method/class>` `<failure mode>` — `<test file>` covers
`<what it does cover>` but not this path."
If all risks are covered by tests: write "All identified risks have unit test coverage."

## §Output

Read `<skill_dir>/review_timestamp.txt` to get the current Sydney timestamp. Use its content as the value for the `Reviewed at` field in the template.

Do NOT run a shell command — the dispatcher pre-wrote this file before the session started.

Load the output template from the skill directory (available via `--add-dir`):
```
<skill_dir>/templates/tech-review.md
```

Fill every `<!-- FILL: ... -->` slot in the template according to its inline instructions
and the rules in this skill. Remove all `<!-- FILL: ... -->` comments from the final output.
Do not add sections not present in the template. Do not remove sections from the template.

**STRUCTURAL CONSTRAINT — enforce before writing:**
1. List the H2 headings in the template. Verify your output contains exactly those headings.
2. Verify every H3 in `## ⚠️ Correctness Risks` is a declarative statement with no file path and no question mark.
3. Verify every field pair in every risk entry is separated by a blank line.
4. Verify every `**Confirm:**` field begins with an interrogative word and ends with `?`.
5. Verify every bullet in `## 🔍 Verification Checklist` ends with a yes/no question or
   a skip entry.

Write the completed file to: `<output>/tech-review.md`

### Formatting rules

- **Active voice:** Write sentences with a clear subject acting. Preferred: "The client raises `AssertionError`." Prohibited: "causing the client to raise `AssertionError`".
- **One idea per sentence:** Do not chain clauses with "or", "and", or "which". If two things happen, write two sentences in two separate fields or raise two separate entries.
- **No em dashes as connectors:** Use a period or colon instead.
- **Bold for skimmability:** In each H3 body, bold the single noun phrase naming the
  failure mechanism.
- **H3 framing in `## ⚠️`:** Each H3 is a declarative statement of the failure mode, naming the symbol only. No file paths. No question marks.
- **Full paths in `Does:`:** When citing file paths, use the full path as it appears in the manifest. Place it in the `Does:` field, not the H3.

## §Blast Radius

N/A

## §Index Entry Format

N/A — no index in this skill.


