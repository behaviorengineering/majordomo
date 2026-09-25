# PR Summary Score Skill

This skill scores a completed `summary.md` against the structural and prose rules defined
by the `pr-review-summary` skill. It is invoked by the orchestrate summary loop after each generation
pass. It does NOT rewrite the summary — it only evaluates and records findings.

This skill runs as `mode:score` — it produces a single `score.md` file.

## Table of Contents
1. [§Input](#input)
2. [§Rubric](#rubric)
3. [§Scoring](#scoring)
4. [§Output](#output)

## §Input

Read `summary.md` from the output directory supplied in the prompt (`output:<dir>`).
Read `score_feedback.md` from the staging directory if it exists — this is the previous
iteration's score. Use it to verify whether the issues it flagged have been addressed.

## §Rubric

Score the summary against the following 6 items. Each item is worth the stated points.
A PASS awards full points. A FAIL awards 0 for that item.

### Item 1 — H2 structure (2 points)

PASS if the summary contains exactly these five H2 headings in this order, with no extras:
1. `## 💡 Why This PR Exists`
2. `## ⚡ What Got Built`
3. `## 🟢 Low-Risk Changes`
4. `## 🟡 Requires Human Judgment`
5. `## 🔍 Where to Focus in the Diff`

FAIL if any heading is missing, renamed, reordered, or if any extra H2 appears.

Evidence: list the actual H2 headings found in the file.

### Item 2 — `⚡` contains H3 + before/after code blocks (2 points)

PASS if `## ⚡ What Got Built` contains at least one H3 heading AND at least one pair of
consecutive fenced code blocks where the first opens with `# Before:` and the second opens
with `# After:`.

FAIL if either the H3 or the before/after code block pair is absent.

Evidence: quote the H3 text and the first two lines of each code block.

### Item 3 — `🟡` uses H3 per concern (2 points)

PASS if every concern in `## 🟡 Requires Human Judgment` is introduced by an H3 heading.
An H3 is a line beginning with `### `.

FAIL if any concern uses a bullet, bold lead, or plain paragraph instead of an H3.

Evidence: list the H3 headings found in the section, or quote the first non-H3 concern.

### Item 4 — No prohibited generic phrases (1 point)

PASS if the summary contains none of the following phrases (case-insensitive):
`better error handling`, `more robust`, `cleaner code`, `improved maintainability`,
`improved readability`, `more maintainable`, `better organized`.

FAIL if any prohibited phrase appears. Quote the sentence containing it.

### Item 5 — No em dashes as connectors (1 point)

PASS if no sentence uses an em dash (`—`) as a connector between two clauses or phrases.
A title that appends a clarification (e.g. `` `File` — short description ``) is acceptable.
A connector joins a subject/clause to a continuation (e.g. "The handler — which runs at startup — does X").

FAIL if any em dash connector is found. Quote the sentence.

### Item 6 — `💡` names a specific class, pattern, or gap (2 points)

PASS if the opening paragraph of `## 💡 Why This PR Exists` names at least one specific
class name, method name, design pattern, or concrete gap (e.g. "the 900-line `CmsController`",
"the lack of a session recovery path", "the hardcoded config in `settings.py`").

FAIL if the paragraph uses only generic statements (e.g. "the code was hard to maintain",
"there was no clear separation of concerns") without naming a specific artifact.

Evidence: quote the opening paragraph.

### Item 7 — No file names embedded in narrative prose (2 points)

PASS if no sentence in the summary embeds a file name (e.g. `foo.py`, `bar.groovy`) inside
flowing prose. File names are permitted inside fenced code blocks, bullet lead-ins, and
skip entries in `## 🔍 Where to Focus in the Diff`.

FAIL if any narrative sentence contains a file name outside those allowed locations.
Quote the sentence.

Evidence: quote any offending sentence, or confirm none found.

### Item 8 — `⚡` H3s exist only for caller-facing changes (2 points)

PASS if every H3 in `## ⚡ What Got Built` represents either:
(a) a change that a developer calling the API would write different code because of, OR
(b) a test H3 (any H3 whose heading contains "test" or "tests" — tests are explicitly
    required to have their own H3 by the generation skill).

H3s for internal infrastructure — graph wiring, screen registry construction, screen
subclass hierarchies, transition topology, pattern-matching internals — must not appear.
Internal detail mentioned for context must be folded into a primary H3 body in one
sentence, not given its own H3.

Also FAIL if any H3 heading in `## ⚡ What Got Built` contains a class count, line count,
or file count (e.g. "113 Screen subclasses", "12 new files", "900-line class rebuilt").
Counts describing scale belong in body prose at most; they MUST NOT appear in H3 headings.

FAIL if any H3 exists for an internal component that has no call site in the diff AND
is not a test H3. Quote the H3 heading.

Evidence: quote any offending heading, or confirm none found.

### Item 9 — `🟡` states concern without prescribing fix (1 point)

PASS if every H3 body in `## 🟡 Requires Human Judgment` ends after stating the
observation, the risk, and what the reviewer must confirm. Sentences that prescribe an
action for the team to take (e.g. "add it as a subclass", "update `REQUIRED_IDENTIFIERS`",
"document the boundary", "confirm with the domain team") are not permitted.

FAIL if any H3 body contains an instructional sentence directing the team to perform a
specific action. Quote the sentence.

Evidence: quote any offending sentence, or confirm none found.

### Item 10 — `⚡` prose uses the two-part team consequence formula (2 points)

PASS if the prose after each before/after code block pair states both:
(1) what the team no longer has to do, and
(2) what they now do instead.
The prose must NOT document the API (no inputs, outputs, exception names, parameter types,
or return types). It must NOT describe internal mechanics (how the component works inside).
It must NOT be a general architectural statement that omits the team consequence.

FAIL if the prose after any code block pair:
- reads as API documentation, OR
- describes only how the component works internally, OR
- is a general statement without naming what the team stops doing and what they do instead.
Quote the offending sentence.

Evidence: quote any offending sentence, or confirm none found.

### Item 11 — No benefit fluff (1 point)

PASS if no sentence in the summary appends the obvious consequence of a stated fact.
Prohibited patterns: "this ensures", "this enables", "making X easier", "allowing the team
to", "so that developers can", "which means", "resulting in" when the consequence is
already implied by the fact stated in the same sentence or the one before it.

FAIL if any sentence appends an obvious consequence that the reader can already infer
from the preceding fact. Quote the sentence.

Evidence: quote any offending sentence, or confirm none found.

### Item 12 — No hedging (1 point)

PASS if no sentence speculates about future state using unconfirmed language.
Prohibited patterns: "suggests", "may", "could", "might", "would likely", "appears to"
when used to speculate about intent, future PRs, or outcomes that cannot be confirmed
from the diff.

FAIL if any sentence speculates about future state that is not confirmed by the diff.
Quote the sentence.

Evidence: quote any offending sentence, or confirm none found.

### Item 13 — No fabricated diff lines (2 points)

PASS if every before/after code block pair in `## ⚡ What Got Built` contains only lines
that can be verified as actual removed/added lines from the diff. A block is fabricated
if it is labelled "hypothetical", "representative", or "pre-change state", or if the
comment inside the block indicates the lines are constructed rather than quoted.

FAIL if any code block is labelled as hypothetical, representative, or constructed, OR
if any comment inside a code block reads as an apology for invented content
(e.g. `# hypothetical pre-change state`, `# representative example`).
Quote the offending label or comment.

Evidence: quote the offending label/comment, or confirm none found.

### Item 14 — Static analysis entry is a bullet, not an H3 (1 point)

PASS if the static analysis entry in `## 🟢 Low-Risk Changes` is a bullet point (line
starting with `-`) and does NOT list individual rule codes beyond the single dominant
pattern. A bullet that names one tool, one total count, one dominant pattern, and one
action is correct.

FAIL if:
- The static analysis entry appears as an H3 heading (`### `), OR
- The bullet breaks findings down by multiple rule codes (e.g. lists `UP045`, `I001`,
  `TC002`, `N818` separately).
Quote the offending entry.

Evidence: quote the SA entry as written, or confirm it is a single-pattern bullet.

### Item 15 — `🟡` contains no architectural hazards (1 point)

PASS if `## 🟡 Requires Human Judgment` contains no H3 that describes an architectural
hazard, code correctness risk, or undiscoverable-registration pattern. Permitted content:
decisions to make, migration completeness, deployment prerequisites, business logic
edge cases. Not permitted: dynamic subclass enumeration risks, concurrency hazards, loop
termination risks, closure binding issues, or registry discovery patterns.

FAIL if any H3 body describes a structural risk that belongs in `tech-review.md` rather
than a judgment call for the reviewer. Quote the H3 heading.

Evidence: quote any offending H3 heading, or confirm none found.

## §Scoring

Sum the points from all PASSed items. Maximum is 23.

Write the score on its own line in the exact format: `SCORE: N`

where `N` is the integer total (0–19). No trailing text on that line.

## §Output

Load the output template from the skill directory:
```
<skill_dir>/templates/score.md
```

The skill directory is the directory containing this `SKILL.md` file.

Fill every `<!-- FILL: ... -->` slot. Remove all `<!-- FILL: ... -->` comments from the output.

Write the completed file to: `<output>/score.md`

Do not write any other files.
