# Prose Quality Skill

The goal of this skill is to make generated output **pleasant to read for a human**.
A human reader should be able to open the artifact, scan it quickly to orient themselves,
then read the parts they care about without friction. The rules below exist to remove
the specific patterns that make auto-generated text feel machine-produced: connector
dashes that interrupt flow, labels that prefix every sentence, walls of text with no
visual rest, icons that decorate without informing, and emphasis scattered so liberally
it stops meaning anything.

This skill runs as a post-wave batch pass over all output files in the skill output directory.
It does NOT run inline inside other skills. It is dispatched once by the pipeline after all
file-review batches and finalize complete, before synthesis begins.

## Table of Contents
1. [§Execution](#execution)
2. [§Rules](#rules)
3. [§Verification Checklist](#verification-checklist)

## §Execution

You will receive a prompt of the form:
```
PR #<N> output:<output-dir> mode:prose
```

Steps:
1. List all `.md` files directly inside `<output-dir>` (non-recursive, depth 1 only). These are
   either per-file review reports or synthesis outputs (summary.md, tech-review.md, blast-radius.md).
2. For each `.md` file listed:
   a. Read the file.
   b. Apply every rule in §Rules to the content.
   c. Run the §Verification Checklist against the revised content. Fix all failures.
   d. Write the revised content back to the same file path.
3. Do NOT read or write any file outside `<output-dir>`.
4. Do NOT recurse into subdirectories.
5. Do NOT process `index.md` or any file in a `logs/` subdirectory.

Pre-Action Gate: before every tool call, write in your session trace:
```
Action: <what you are about to do>
File: <exact path> / N/A
Authorised by: §Execution step <N>
Decision: PROCEED / BLOCKED
```
If the file is not in the listed output files, write BLOCKED and stop.

## §Rules

### No em dashes as connectors

Em dashes used as connectors (`—`) MUST be replaced with a period or colon.

**Before:**
```
The controller retries navigation on NavigationStepFailedException — the caller gets
NavigationMaxRetriesException after 3 attempts.
```

**After:**
```
The controller retries navigation on NavigationStepFailedException. The caller gets
NavigationMaxRetriesException after 3 attempts.
```

---

### No metadata labels as sentence prefixes

Labels like `VERIFIED —`, `INFERRED —`, `ASSUMED —` used as sentence prefixes interrupt reading
flow without adding meaning at that position. If the confidence level is load-bearing, move it
to a parenthetical at the end. If the content already makes confidence clear, drop the label.

**Before:**
```
VERIFIED — The intent is to migrate from the existing navigator-per-system pattern to a shared
navigation framework.
```

**After (label is load-bearing — keep as parenthetical):**
```
The intent is to migrate from the existing navigator-per-system pattern to a shared navigation
framework. (VERIFIED)
```

**After (label adds nothing — drop it):**
```
The intent is to migrate from the existing navigator-per-system pattern to a shared navigation
framework.
```

---

### No dead-end restatements

A labelled block or closing sentence that only restates what the preceding sentence already
demonstrated MUST be cut or folded into that sentence.

The test: if the Effect/Summary/Result sentence could be inferred directly from the Now/Before
sentence without adding new information, it is a restatement.

**Before:**
```
Now: NavigationGraph stores edges as Transition objects with source/destination pairs;
     NavigationController.navigate_to() finds the shortest BFS path and executes it once.
Effect: Common navigation sequences are defined once; new screens only require registering
        transitions rather than reimplementing full paths in each method.
```

**After:**
```
Now: NavigationGraph stores edges as Transition objects; navigate_to() finds the shortest BFS
     path and executes it once, so new screens only require registering transitions rather than
     reimplementing full navigation paths.
```

---

### No benefit fluff

Phrases that describe the obvious consequence of a stated fact MUST be removed. State the fact.
Do not append what the reader can already infer.

**Before:**
```
LegacyNavigator remains in the codebase with all methods annotated as replaced but not deleted —
migration plan for Phase 4/5 is not visible in the diffs. If other code still imports
LegacyNavigator, this creates two parallel code paths until removal is complete.
```

**After:**
```
LegacyNavigator remains with all methods annotated as replaced but not deleted. Migration plan for
Phase 4/5 is not visible in the diffs. Check whether any callers still import LegacyNavigator.
```

The cut: "this creates two parallel code paths" is the obvious consequence of "not deleted" +
"still imports". The reader already knows this. The actionable question ("check whether any
callers still import it") is what belongs.

---

### No hedging

Speculative language about future state MUST be stated as fact or cut entirely.

**Before:**
```
BillingNavigator and PortalNavigator are not migrated in this PR — intentional scope limit, but
suggests future PRs will mirror this refactor for other systems.
```

**After (can be confirmed from context):**
```
BillingNavigator and PortalNavigator are not migrated in this PR. Other navigators follow in
subsequent PRs.
```

**After (cannot be confirmed):**
```
BillingNavigator and PortalNavigator are not migrated in this PR. Intentional scope limit.
```

---

### Concept density

A single paragraph that introduces more than 7 distinct named concepts MUST be split. Do not
compress 8+ concepts into denser prose.

**Before (8 concepts in one paragraph):**
```
This PR replaces the imperative navigator pattern with a graph-based navigation architecture.
A new NavigationController orchestrates screen recognition, path-finding, and transition
execution via NavigationGraph, ScreenRecogniser, and SessionGuard. The existing LegacyNavigator
class remains intact but annotated; domain logic moves into six new service modules that
delegate to the controller and extract data from 53 new Screen subclasses. The PR also adds
a .majordomo Git submodule and updates .env.template.
```

**After (split at natural boundary):**
```
This PR replaces the imperative navigator pattern with a graph-based navigation architecture.
NavigationController orchestrates screen recognition, path-finding, and transition execution
via NavigationGraph, ScreenRecogniser, and SessionGuard. Domain logic moves into six new
service modules (AppService, AccountService, ConnectionsService, InquiryService,
OfficersService, PortalService) that delegate to the controller.

LegacyNavigator remains with methods annotated as replaced. Screen data extraction moves into
53 new Screen subclasses in terminal/screens/. The PR also adds a .majordomo Git
submodule and updates .env.template with APP__SYSTEMS.
```

This is a flag + split instruction. Do not compress — split.

---

### No multi-clause sentences

A sentence with more than one dependent clause MUST be split into shorter sentences, one idea each.
Dependent clauses are introduced by: `or`, `and`, `which`, `that`, `causing`, `resulting in`, `so that`, `allowing`.

This applies to prose blocks only — not to code, paths, or inline technical identifiers.

**Before:**
```
Requests.Session is not thread-safe, so concurrent cookie updates and connection pooling
state corruption cause intermittent HTTP failures or authentication leaks.
```

**After:**
```
`requests.Session` is not thread-safe. Concurrent requests corrupt cookie state and
connection pooling. The result is intermittent HTTP failures or credential leaks.
```

**Before:**
```
Authentication succeeds on the server but the client raises AssertionError because the
substring is not found, causing all SOL API calls to fail despite valid credentials.
```

**After:**
```
Authentication succeeds on the server. The client raises `AssertionError` because the
substring is not found. All SOL API calls fail despite valid credentials.
```

---

### Blank lines between labeled field pairs

In any block where consecutive lines begin with a bold label (`**Does:**`, `**Trigger:**`,
`**Consequence:**`, `**Confirm:**`, or any `**<Label>:**` pattern), each field MUST be
separated from the next by a blank line.

Without blank lines, Markdown renders the fields as a single concatenated paragraph.

**Before:**
```
**Does:** `navigate_to()` retries `_attempt_navigation()` up to `max_retries` times.
**Trigger:** If `_try_recover()` consistently returns `False`, the loop makes no progress.
**Consequence:** The caller waits for `max_retries * step_timeout` seconds.
**Confirm:** Does `_try_recover()` have any guard to abort early?
```

**After:**
```
**Does:** `navigate_to()` retries `_attempt_navigation()` up to `max_retries` times.

**Trigger:** If `_try_recover()` consistently returns `False`, the loop makes no progress.

**Consequence:** The caller waits for `max_retries * step_timeout` seconds.

**Confirm:** Does `_try_recover()` have any guard to abort early?
```

---

### No decorative icons or emoji

Icons and emoji MUST NOT appear in generated output unless the consuming skill explicitly
defines a symbol set with assigned meanings (e.g. `🔴 = critical`, `🟡 = warning`).

Decorative use — prefixing bullets with ✅, 🔹, 📌, or similar — adds visual noise without
conveying information. A reader who strips icons must lose nothing.

**Before:**
```
## What Improved

✅ **Extracted navigation retry logic into controller**
📌 **Centralized session lifecycle in BaseTerminalService**
```

**After:**
```
## What Improved

**Extracted navigation retry logic into controller**
**Centralized session lifecycle in BaseTerminalService**
```

**Exception — severity legend:** Icons defined by a consuming skill's output format (e.g. a severity legend) are intentional and must be preserved.

**Exception — section-header navigation:** A consuming skill may define a consistent icon set on H2 section headers as visual category anchors (e.g. `## ⚠️ Considerations`, `## ✅ What Improved`). These signal content category and help readers scan a long document. This exception covers H2 headers only — not bullet prefixes or inline prose.

---

### Purposeful emphasis

**Bold** and *italic* MUST mark the single most important word or phrase in a block — not
be applied broadly, decoratively, or to whole sentences.

Rules:
- Bold the first occurrence of a key term, concept, or named component in a section — the noun phrase a skimmer would scan for
- Bold decision outcomes and critical constraints inline (e.g. **never**, **always**, **required**)
- Do not bold more than one noun phrase per paragraph
- Do not bold verbs, adjectives, or transitional phrases ("However", "In contrast")
- Do not bold a phrase that is already conveyed by its position (e.g. a section heading)
- Do not bold more than 5 consecutive words
- Italic marks filenames, paths, and config keys on first mention in a section: *config.yaml*, *Jenkinsfile*
- Italic marks technical terms being defined or distinguished: *orchestration* vs *execution*
- Italic is NOT for general emphasis — it signals "this is a specific technical thing"
- Do not bold or italicise more than 15% of the words in any prose block

**Before:**
```
The **new** `NavigationController` **orchestrates** screen recognition, **path-finding**,
and **transition execution** via `NavigationGraph`, `ScreenRecogniser`, and `SessionGuard`.
```

**After:**
```
The new **`NavigationController`** orchestrates screen recognition, path-finding, and
transition execution via `NavigationGraph`, `ScreenRecogniser`, and `SessionGuard`.
```

---

### Paragraph length

A paragraph that reaches 4 or more sentences MUST be split at the first natural topic boundary.
Each paragraph MUST express one main idea.

**Before:**
```
We use a dispatcher pattern. This entry point validates parameters and delegates to the
component pipeline. Component pipelines are not directly invocable. This separation means
parameter validation lives in one place. Adding a new parameter only requires updating
the dispatcher.
```

**After:**
```
We use a **dispatcher pattern**: all invocations go through a single entry point that
validates parameters and delegates to the target component pipeline.

Component pipelines are not directly invocable. Adding a new parameter only requires
updating the dispatcher.
```

---

### Sentence length

A sentence exceeding 25 words MUST be split at the first conjunction or subordinate clause.
The subject and verb MUST appear within the first 8 words.

This applies to prose blocks only — not to code, paths, or inline technical identifiers.

**Before:**
```
Because Jenkins needs to remain stateless and because cluster access requires credentials
that are difficult to rotate, we decided to adopt GitOps.
```

**After:**
```
We adopted **GitOps** to keep Jenkins stateless. Cluster credentials are difficult to rotate.
Jenkins never accesses Kubernetes directly.
```

---

### List formatting

All bullets within a list MUST begin with the same grammatical form (all verb phrases or all
noun phrases). Bullet text MUST NOT exceed 15 words. Move detail exceeding 15 words to a
following sentence or sub-bullet. Lists MUST NOT nest more than two levels deep.

**Before:**
```
- Validates input parameters before dispatching
- The component pipeline handles the actual work
- Logging at every stage so failures are traceable
- We never use Groovy for business logic
```

**After:**
```
- Validates input parameters before dispatching
- Delegates to the target component pipeline
- Logs start time and completion status at every stage
- Keeps business logic in bash scripts, not Groovy
```

---

### Visual breaks

A blank line MUST appear between every paragraph. Distinct concept blocks under the same H2
that are not sequential steps MUST be separated with a `---` horizontal rule. Two consecutive
code blocks MUST have at least one prose sentence between them. A heading MUST NOT immediately
follow another heading without content between them.

**Before:**
```
## Setup
### Prerequisites
Install Docker and configure credentials.
### Installation
Run the setup script.
```

**After:**
```
## Setup

### Prerequisites

Install Docker and configure credentials.

---

### Installation

Run the setup script.
```

---

### Heading scannability

H2 and H3 headings MUST be noun phrases that describe content. A reader scanning only the
headings MUST be able to reconstruct the document's structure. Single-word headings MUST be
rewritten as descriptive noun phrases. A child heading MUST NOT repeat the key word of its
parent heading.

**Before:**
```
## Overview
## Details
## Notes
```

**After:**
```
## What the Dispatcher Pattern Does
## How Component Pipelines Are Structured
## Known Constraints and Edge Cases
```

---

## §Verification Checklist

Run this checklist against every prose section in the output file before completing.
All items must pass.

- [ ] **Blank lines between labeled fields:** Every `**<Label>:**` field in a consecutive block is separated from the next by a blank line
- [ ] **No em dashes as connectors:** No `—` used to join two clauses
- [ ] **Metadata labels as parentheticals:** `VERIFIED`, `INFERRED`, `ASSUMED` appear at end of sentence, not as prefix
- [ ] **No dead-end restatements:** No Effect/Summary/Result block that only restates the preceding Now/Before
- [ ] **No benefit fluff:** No "this ensures", "this enables", "making X easier" appended to factual statements
- [ ] **No hedging:** No "suggests", "may", "could" speculating about future state
- [ ] **Concept density:** No single paragraph introduces more than 7 distinct named concepts
- [ ] **No multi-clause sentences:** No sentence contains more than one dependent clause introduced by `or`, `and`, `which`, `that`, `causing`, `resulting in`, `so that`, or `allowing`
- [ ] **No decorative icons:** No emoji or icon prefixes unless defined by the consuming skill's symbol set
- [ ] **Purposeful emphasis:** Bold used on first occurrence of key term per section and on decision outcomes only; no bold verbs or adjectives; no more than 5 consecutive words bolded; italics used for filenames, paths, and defined technical terms; no over-emphasis (>15% of words)
- [ ] **Paragraph length:** No paragraph contains 4 or more sentences; each paragraph expresses one main idea
- [ ] **Sentence length:** No prose sentence exceeds 25 words; subject and verb appear within the first 8 words
- [ ] **List formatting:** All bullets in a list share the same grammatical form; no bullet exceeds 15 words; lists not nested more than 2 levels deep
- [ ] **Visual breaks:** Blank line between every paragraph; `---` between distinct concept blocks under the same H2; no back-to-back code blocks; no heading immediately followed by another heading
- [ ] **Heading scannability:** All H2/H3 headings are descriptive noun phrases; no single-word headings; no child heading repeats the parent heading's key word

Failure action: fix the violation in the output file, then re-run the checklist from the top.
