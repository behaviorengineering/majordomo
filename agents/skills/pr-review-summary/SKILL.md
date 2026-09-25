# PR Summary Skill

This skill specializes the PR Review Agent to produce a single human-readable summary of all
changes in a PR. It is written for two audiences: the developer who wants to explain their PR,
and the reviewer coming in cold who needs to understand what changed and whether to worry about
it — before reading the technical review (`tech-review.md`) or per-file findings.

The summary covers decisions, migration risks, deployment prerequisites, and business logic
edge cases. Code correctness risks (loop termination, concurrency hazards, closure binding)
belong in `tech-review.md`, not here.

This skill runs as `mode:summary` — it does not produce per-file finding reports.

## Table of Contents
1. [§Prioritization](#prioritization)
2. [§Summary Config](#summary-config)
3. [§Feedback Integration](#feedback-integration)
4. [§Pre-Synthesis Enumeration](#pre-synthesis-enumeration)
5. [§Review Criteria](#review-criteria)
6. [§Section Types](#section-types)
7. [§Pre-Writing Analysis](#pre-writing-analysis)
8. [§Source Lookup](#source-lookup)
9. [§Prose Review](#prose-review)
10. [§Blast Radius](#blast-radius)
11. [§Report Label](#report-label)
12. [§Index Entry Format](#index-entry-format)

> **TL;DR section added after `## 💡 Why This PR Exists`.** See `§Section Types → TL;DR` and Pre-Writing Step 6.

## §Prioritization

N/A — this skill reads all files in the manifest in manifest order. No tier grouping.

## §Summary Config

The manifest may contain a `summary_config` key. If it is absent or empty, apply all default behaviour — no changes.

If present, it has this shape:

```json
{
  "sections": {
    "why":        { "enabled": true,  "instructions": "Focus on API contract decisions only." },
    "tldr":       { "enabled": true },
    "what-built": { "enabled": true },
    "low-risk":   { "enabled": false },
    "judgment":   { "enabled": true,  "instructions": "Flag migration risks and deployment prerequisites only." },
    "focus":      { "enabled": true }
  }
}
```

Section keys map to output sections:

| Key | Output section |
|-----|---------------|
| `why` | `## 💡 Why This PR Exists` |
| `tldr` | `## TL;DR` |
| `what-built` | `## ⚡ What Got Built` |
| `low-risk` | `## 🟢 Low-Risk Changes` |
| `judgment` | `## 🟡 Requires Human Judgment` |
| `focus` | `## 🔍 Where to Focus in the Diff` |

Apply these rules before writing:

- **`enabled: false`** — Omit that section entirely from the output. Also omit the corresponding TL;DR bullet for that section.
- **`instructions`** — Treat as an additional constraint for that section only, applied on top of `§Section Types`. The instruction narrows scope; it does not override structural rules (H3 requirement for `judgment`, code block requirement for `what-built`, etc.).
- A section not listed in `summary_config.sections` defaults to enabled with no extra instructions.

## §Feedback Integration

Before beginning any analysis, check whether `score_feedback.md` exists in the staging
directory (the same directory that contains `manifest.json`). The staging directory is
the `batch_000` directory passed in the prompt as `staging:<path>`.

If `score_feedback.md` is absent, proceed normally with no changes to behaviour.

If `score_feedback.md` is present, read it in full. It is a structured score report from
the previous generation attempt. Each item is marked PASS or FAIL with specific evidence.

For each FAIL item, apply the corresponding correction constraint during `§Pre-Writing Analysis`:

- **Item 1 (H2 structure):** Verify you are using exactly the five H2 headings from the
  template in order. Do not rename, reorder, or add any H2.
- **Item 2 (`⚡` H3 + code blocks):** The `§Pre-Writing Analysis` Step 2 must produce a
  before/after code block pair from actual diff lines before you write the section.
  Do not proceed to writing until you have quoted the exact lines.
- **Item 3 (`🟡` H3 per concern):** Every concern in `## 🟡 Requires Human Judgment` must
  open with `### `. Do not use bullets or bold lead-ins for concerns.
- **Item 4 (generic phrases):** Before writing, grep your draft for the prohibited phrases
  listed in `§Review Criteria`. Remove any found before writing the file.
- **Item 5 (em dash connectors):** Replace any em dash connector (`—` joining two clauses)
  with a period or colon. Title-style uses (e.g. `` `File` — short description ``) are allowed.
- **Item 6 (`💡` names specific artifact):** The opening paragraph of `## 💡 Why This PR
  Exists` must name at least one specific class name, method name, or concrete gap.
  Generic statements without naming an artifact are not acceptable.
- **Item 7 (file names in prose):** Scan every narrative sentence. Move any embedded file
  name out of flowing prose into a bullet lead-in, code block, or skip entry. File names
  must not appear inside sentences.
- **Item 8 (`⚡` caller-facing H3s only):** Scan every H3 heading in `## ⚡ What Got Built`.
  Remove any H3 for internal infrastructure that has no call site in the diff (graph wiring,
  registry construction, screen subclass hierarchies, transition topology). The test H3 is
  always permitted and must NOT be removed. Fold a one-sentence mention into the primary H3
  body if context is needed, or omit it entirely.
- **Item 9 (`🟡` no prescriptive fix):** Scan every H3 body in `## 🟡 Requires Human
  Judgment`. Remove any sentence that directs the team to perform a specific action
  (e.g. "add it as a subclass", "update the identifier", "document the boundary").
  Each H3 must end after stating the observation, the risk, and what the reviewer must
  confirm.
- **Item 10 (`⚡` two-part team consequence):** Scan every H3 body in `## ⚡ What Got Built`
  that follows a before/after code block pair. Replace the prose with the two-part formula:
  (1) what the team no longer has to do, (2) what they now do instead. Remove any sentence
  that documents the API (inputs, outputs, exception names) or describes internal mechanics.
  Remove any general architectural statement that omits the team consequence.
- **Item 11 (no benefit fluff):** Scan every sentence for phrases that append the obvious
  consequence of a stated fact: "this ensures", "this enables", "making X easier", "allowing
  the team to", "so that developers can", "which means", "resulting in". Remove the appended
  consequence clause — state the fact only.
- **Item 12 (no hedging):** Scan every sentence for speculative language about future state
  that cannot be confirmed from the diff: "suggests", "may", "could", "might", "would likely",
  "appears to". Replace with a confirmed fact or cut the sentence.
- **Item 13 (no fabricated diff):** Check every before/after code block. Remove any label or
  inline comment that marks the code as hypothetical, representative, or constructed
  (e.g. `# hypothetical pre-change state`). If the actual removed lines are not in the diff,
  the H3 must be removed entirely — do not replace the fabricated block with a real one.
- **Item 14 (SA as bullet):** Rewrite the static analysis entry as a single bullet if it
  appears as an H3 or lists multiple rule codes. Format: tool name, total count, dominant
  pattern, action. One bullet only.
- **Item 15 (no architectural hazards in 🟡):** Remove any H3 in `## 🟡 Requires Human
  Judgment` that describes a structural risk, dynamic discovery pattern, or concurrency
  hazard. Move the observation to a note at the end of the nearest caller-facing H3 body
  in `## ⚡ What Got Built`, or omit it if no appropriate H3 exists.

Do not acknowledge the feedback in the output. Apply the corrections silently.

## §Pre-Synthesis Enumeration

Before writing any prose, enumerate all files in the manifest and group them by the first
distinctive package segment below any common path prefix. To find it: strip the longest
path prefix shared by all files in the manifest, then use the next directory segment as
the group key (e.g. if most files share `src/automation/`, group by the segment after that:
`terminal`, `services`, `models`, `navigators`, `tests`). For flat repos with no common
prefix, use the top-level directory segment directly.
For each group, record the file count. This enumeration is working memory only — do not emit
it in the output. Use it to ensure the What Changed and Impact Surface sections proportion-
ally represent all large change clusters, not just the most architecturally prominent ones.

The stripped prefix is used for grouping only. When citing file paths in any output section,
use the full path as it appears in the manifest — do not omit the common prefix.
A group with 5 or more files is a major cluster and MUST be represented in the output.

If the manifest contains a `dep_clusters` key, use it to annotate related files within
each enumeration group. Each entry in `dep_clusters` is a list of files connected by
direct imports. When describing a cluster in What Changed or Impact Surface, use these
edges to explain relationships (e.g. "`cms_account_service.py` imports `base_terminal_service.py`
and `cms_transitions.py`") rather than inferring relationships from naming or architecture.

If the manifest contains a `reverse_deps` key, use it to verify scope claims before
writing them. The structure is `{changed_file: [unchanged_files_that_import_it]}`.
Before claiming that a migration is complete or that "all X were updated", check whether
any unchanged files still import the old symbol — if `reverse_deps` lists importers for
a file that was expected to be fully replaced, downgrade the claim from VERIFIED to
INFERRED and note the specific unchanged importers in `## Considerations`.

## §Review Criteria

This skill MUST NOT produce `[CRITICAL]`, `[WARN]`, or `[INFO]` findings.
This skill MUST NOT evaluate code correctness, security, or style.
All analysis is synthesis and orientation only.

Do not use confidence labels (`VERIFIED`, `INFERRED`, `ASSUMED`) in output. State claims
as fact if the diff supports them. If a claim cannot be confirmed, either omit it or
name the specific uncertainty (e.g. "no migration plan is visible in the diff").

Do not use generic phrases. Every statement must name what specifically changed.
Prohibited: "better error handling", "more robust", "cleaner code", "improved maintainability".
Required: name the mechanism and its consequence.

If the manifest contains a `static_analysis` key, use it as the source for SA findings.
The structure is `{tool_name: raw_output_string}`. Count the total findings across all tools,
identify the dominant pattern, and note any findings that require human judgment.
Summarise in `## 🟢 Low-Risk Changes` as a **single bullet** (tool name, count, dominant pattern, action).
Do not list individual findings. Do NOT break down findings by rule code in the bullet — one dominant pattern only.
Do NOT write the SA entry as an H3 — it MUST be a bullet regardless of finding count.
If any finding requires human judgment (e.g. duplicate test definition, shadowed symbol), raise it as an H3 in `## 🟡 Requires Human
with the rule code in the heading.
If `static_analysis` is absent or empty, omit the SA bullet entirely.

If any file in the manifest has a path segment matching `test`, `tests`, `spec`, or `specs`,
that cluster MUST be represented in `## ⚡ What Got Built` with a description of what the
tests cover and how they work.

Claims about migration or refactor scope (e.g. "all X updated", "across the codebase",
"many navigators") MUST be supported by naming at least one specific file from the diff.
Do not infer scope from architecture — derive it from the file list.

## §Section Types

Each section in the output template has a defined purpose and shape. Apply these rules when filling every slot.

**`## TL;DR`**
Four bullets, one per section that follows. Each bullet is one sentence. Use the same specific artifact names as the corresponding section — do not introduce new content or re-explain the Why paragraph.

- **⚡ Built:** one sentence naming the primary deliverable from `## ⚡ What Got Built`.
- **🟢 Low-risk:** one sentence naming the dominant pattern from `## 🟢 Low-Risk Changes`. Omit this bullet entirely if that section contains no entries.
- **🟡 Watch for:** one sentence naming the highest-priority open question from `## 🟡 Requires Human Judgment`.
- **🔍 Start at:** one file to read first (from `## 🔍 Where to Focus`) — skip one file (the skip entry from that section).

No hedging. No generic phrases. No em dashes as connectors.

**`## 💡 Why This PR Exists`**
Opens with the specific pain point: name the class, pattern, or gap that made the old code hard to work with. Paragraph two names the *decisions* made to fix it — not how the machinery works, but what changed about how callers interact with the system. One sentence per decision is enough. Technical specifics (class names, method signatures, algorithm names) belong in the code blocks in `## ⚡ What Got Built`, not here. Close with the intentional scope limit if one exists. If nothing was left out, omit that paragraph. Do not describe files or count them.

**`## ⚡ What Got Built`**
One H3 per candidate that passed the Step 2 gate (call site found in diff). Do NOT create H3s for components that did not pass the gate — no exceptions for "major" or "core" components. Internal infrastructure (graph wiring, registry construction, subclass hierarchies, transition topology) never has a call site and never gets an H3.

The primary H3 MUST open with a before/after code block at the call site — the exact lines quoted in the Step 2 gate. After the code blocks, write the prose sentence declared in the Step 2 gate — the two-part formula: **(1) what the team no longer has to do**, **(2) what they now do instead**. Use the exact clauses from the gate. Do not add a third clause. Do not document the API — no inputs, outputs, or exception names. Do not describe internal mechanics. The code blocks show the technical contrast; the prose names the consequence for the team. Secondary H3s follow the same rule. Tests get their own H3: name the count, describe snapshot mechanics for integration tests, name subsystems covered by unit tests.

Do NOT describe internal mechanics in prose: how the component finds a path, how it builds a registry, how it matches patterns, what methods it calls internally, how transitions are registered, what the graph traversal algorithm does. If a technical detail cannot be shown as a code example at the call site, it does not belong in prose — omit it entirely. Do NOT enumerate implementation internals: class counts, line counts, method counts, directory listings, or transition/entry totals. Those belong in `tech-review.md`. If a count is needed to convey scale (e.g. "18 new tests"), one is sufficient — do not list counts per subsystem or per directory.

Tests H3: if the diff introduces the first tests for this subsystem (no test files existed before this PR), open the H3 with that fact before naming the count. Then name the count, describe snapshot mechanics for integration tests, and name subsystems covered by unit tests. If prior tests existed, omit the prior-state sentence and start with the count.

**`## 🟢 Low-Risk Changes`**
One entry per change that has no architectural consequence and requires no reviewer judgment. Use H3 + optional code block when the change introduces a new usage pattern a reader needs to see. Use a bullet for purely mechanical changes (import path, gitmodule, one-line wiring swap). Do not explain what the component does — only what changed.

**`## 🟡 Requires Human Judgment`**
One H3 per open question the PR creates but does not resolve. Scope: decisions, migration
risks, deployment prerequisites, and edge cases in business logic. Do NOT raise code
correctness bugs, architectural hazards, or loop/concurrency failure modes here — those
belong in `tech-review.md`. Each H3 body is 2-4 sentences: what the diff shows, what the
risk or decision is, what the reviewer must confirm. Name the specific file or method.
Do not raise concerns the PR itself already resolves.

**`## 🔍 Where to Focus in the Diff`**
4-6 bullets. Each names a file or directory and the one question a reviewer needs to answer there. Last bullet is always a skip entry. Use full paths as they appear in the manifest.

## §Pre-Writing Analysis

Before filling any template slot, complete this analysis in working memory. Do not write the output file until all steps are done.

**Step 1 — Why:** What was the specific pain point? Name the class, pattern, or gap. What two or three mechanisms does this PR introduce to fix it? What was left out intentionally?

**Step 2 — What Got Built:** For each component or subsystem introduced or significantly changed in the diff, complete this gate in working memory before deciding whether it gets an H3:

```
Candidate: [component name]
Call site in diff: YES — [quote the exact before lines] / NO
Decision: H3 (call site found) | FOLD (mention in primary H3) | OMIT (no call site, no mention needed)
If H3 — prose sentence (two parts, complete both before writing):
  1. What the team no longer has to do: [one clause]
  2. What they now do instead: [one clause]
```

**CONSTRAINT — no fabricated code:** The before/after lines quoted in the gate MUST be actual removed/added lines from the diff. NEVER construct a hypothetical or representative example. If the exact removed lines are not present in the diff, the call site answer is NO — do not promote the candidate to H3.

A call site is a line in the diff where *calling code* changes — a service, test, or main module that now calls different code than before. Internal wiring (graph construction, registry population, subclass definitions, `__init__` dependency injection) is NOT a call site. If you cannot quote an actual removed/added line from calling code, the answer is NO.

Work through every candidate. Only candidates with a YES become H3s. Do not create an H3 for any candidate with a NO. After completing all gates, proceed to writing.

For each approved H3: write the two-part formula declared in the gate — (1) what the team no longer has to do, (2) what they now do instead. Use exactly those two clauses. Do not draft API documentation (inputs, outputs, exception names). Tests get their own H3: if test files are net-new for the subsystem (none existed before this PR), state the prior coverage gap first, then name the count, what they cover, and how integration tests work. If prior tests existed, start with the count.

**Step 3 — Low-Risk:** List every change with no architectural consequence. For each: does it introduce a new usage pattern (needs H3 + code block) or is it mechanical (bullet only)?

**Step 4 — Requires Human Judgment:** List every open question the diff creates but does
not resolve. Include only: decisions the team must make, migration completeness checks,
deployment prerequisites (CI setup, credentials, emulator access), and edge cases in
business logic. Exclude: code correctness bugs, loop termination risks, concurrency
hazards, architectural hazards (e.g. undiscoverable registrations, unsafe discovery
patterns, dynamic subclass enumeration), and closure binding issues — those belong in
the technical review. For each item: what does the diff show, what is the risk, what
must the reviewer confirm?

Actively check for these two categories that are commonly missed. If found, MUST raise as an H3 — do not omit them:

- **Synchronous blocking in service constructors or startup paths:** Scan every new `__init__`
  method in the diff. If any performs I/O, network calls, or login (directly or via a method call)
  while holding a lock or before the object is fully constructed, MUST raise it as an H3.
  Name the class and method, describe what blocks, and ask whether that is acceptable given
  the deployment model (single instance vs load-balanced, startup latency tolerance).

- **Unhandled business logic branches that surface as exceptions:** Scan every new service method
  that selects behaviour based on a runtime value (product code, account type, config flag, enum).
  If any branch raises an exception (including `NotImplementedError`, `ValueError`, or a custom
  exception) for unrecognised values, MUST raise it as an H3. Name the method, the condition,
  the exception, and ask whether that value can appear in production data.

**Step 5 — Where to Focus:** Name the 4-6 highest-signal files or directories. For each, state the one question a reviewer needs to answer. Identify one entry that can be safely skipped.

**Step 6 — TL;DR:** After completing Steps 1–5, compile the four TL;DR bullets. Each MUST use the same artifact name used in the corresponding section — copy the term exactly. Do not write the TL;DR bullets until all other steps are complete. If `## 🟢 Low-Risk Changes` is empty, omit the Low-risk bullet.



Read `<skill_dir>/review_timestamp.txt` to get the current Sydney timestamp. Use its content as the value for the `Reviewed at` field in the template.

Do NOT run a shell command — the dispatcher pre-wrote this file before the session started.

Load the output template from the skill directory (available via `--add-dir`):
```
<skill_dir>/templates/summary.md
```

The skill directory is the directory containing this `SKILL.md` file.

Fill every `<!-- FILL: ... -->` slot in the template according to its inline instructions
and the rules in this skill. Remove all `<!-- FILL: ... -->` comments from the final output.
Do not add sections not present in the template. Do not remove sections from the template.

**STRUCTURAL CONSTRAINT — enforce before writing:**
1. List the H2 headings in the template. Verify your output contains exactly those headings, no more. The required H2 order is: `💡 Why This PR Exists`, `TL;DR`, `⚡ What Got Built`, `🟢 Low-Risk Changes`, `🟡 Requires Human Judgment`, `🔍 Where to Focus in the Diff`.
2. Verify `## ⚡ What Got Built` contains at least one before/after code block pair. If the diff
   does not contain representative removed/added lines, note that inline — do NOT collapse the
   section into prose paragraphs.
3. Verify `## 🟡 Requires Human Judgment` uses H3 headers for each concern, not bullets.
4. Scan every narrative sentence in `## 🟢 Low-Risk Changes` and `## ⚡ What Got Built` for
   embedded file names (e.g. `foo.py`, `bar.groovy`). File names are only permitted inside
   fenced code blocks, bullet lead-ins, and skip entries in `## 🔍 Where to Focus in the Diff`.
   Move any file name in flowing prose to a bullet lead-in or code block.
5. Verify every H3 in `## ⚡ What Got Built` either (a) has a call site quoted in the Step 2
   gate, or (b) is the test H3. Remove any H3 that fails both conditions.
6. Verify the prose after each before/after code block pair states (1) what the team no longer
   has to do, and (2) what they now do instead. If the prose documents the API instead (inputs,
   outputs, exception names, method signatures), rewrite it using the two-part formula.
7. Scan every sentence for benefit fluff: phrases that append the obvious consequence of a
   stated fact ("this ensures", "this enables", "making X easier", "allowing the team to",
   "so that developers can"). Remove the appended consequence clause.
8. Scan every sentence for hedging about future state that cannot be confirmed from the diff
   ("suggests", "may", "could", "might", "would likely", "appears to"). Replace with a
   confirmed fact or cut the sentence.
Any violation of items 1–8 must be fixed before writing the file.

Write the completed file to: `<output>/summary.md`

### Formatting rules (apply to all slots)

- **Paragraph spacing:** Separate every paragraph and every bullet with a blank line.
  Do not run multiple sentences together unless they form a single indivisible thought.
- **Bold for skimmability:** In each prose paragraph, bold the single load-bearing noun
  phrase a skimmer would scan for. Bold once per paragraph. Do not bold verbs or adjectives.
- **Bullet lead-in:** In `## 🟢 Low-Risk Changes`, bold the subject of each bullet (component
  name or file path) up to the first colon.
- **H3 framing in `## 🟡`:** Each H3 is a question or declarative statement — not a label.
  Body is 2-4 sentences: observation, risk or decision, what to check.
- **No em dashes as connectors:** Use a period or colon instead.

## §Source Lookup

The repo source tree is available for targeted lookups. Use it only to verify a specific
scope claim that cannot be confirmed from the staged diffs alone.

Rules:
- MUST NOT read changed files from the repo — use only the staged diffs for those.
- MAY `grep` the repo for a symbol, class name, or import to verify whether a migration
  or refactor is broader or narrower than the diffs suggest.
- Limit: at most 3 `grep` calls total across the entire summary pass.
- Do not read full file contents from the repo — `grep` results are sufficient.
- If a lookup contradicts a scope claim, state the specific file evidence that limits it.



Then perform this section-by-section readability sweep on the produced file. For each
section listed below, apply the corresponding checks in working memory and fix any
violations before completing.

**`## 💡 Why This PR Exists`**
- Every sentence names a specific artifact (class, method, pattern, gap). No generic claims.

**`## ⚡ What Got Built` — each H3 body**
- No file names appear inside sentences. File names are permitted only in code blocks.
- No internal mechanics: remove any sentence about class counts, line counts, registry
  construction, graph traversal algorithm, subclass registration, transition wiring, or
  pattern-matching internals. If a sentence cannot be understood without knowing how the
  component is built internally, remove it.
- Each H3 body prose (after the code blocks) uses the two-part formula: (1) what the team
  no longer has to do, (2) what they now do instead. API documentation (inputs, outputs,
  exception names, method signatures) is not permitted here.

**`## 🟢 Low-Risk Changes`**
- No file names inside bullet prose. File names belong in bullet lead-ins (bolded, before
  the first colon) or in code blocks — not embedded mid-sentence.
- Bullets for config file changes must not include tool-internal settings (output format flags, verbosity levels, log targets) unless those settings require reviewer action. State only: what the tool is, what it covers, and what action is needed.

**`## 🟡 Requires Human Judgment` — each H3 body**
- No prescriptive sentences. A prescriptive sentence tells the team to perform a specific
  action (e.g. "add it as a subclass", "update the identifier", "run the integration
  tests", "confirm with the domain team"). The body must end after stating the observation,
  the risk, and what the reviewer must confirm — not what to do about it.
- No file names inside flowing sentences. Cite a file only in the form "in `path/file.py`"
  at the start of a sentence, not embedded mid-clause.

**`## 🔍 Where to Focus in the Diff`**
- Each bullet names one file or directory and one question. No multi-question bullets.

## §Blast Radius

N/A

## §Report Label

N/A — no per-file reports in this skill.

## §Index Entry Format

N/A — no index in this skill.
