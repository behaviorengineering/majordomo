# How the Review Works

*Majordomo — repository operations for evolving software.*

- [Staging](#why-staging-exists)
- [File reviewers vs synthesis reviewers](#file-reviewers-vs-synthesis-reviewers)
- [Execution timeline](#the-execution-timeline)
- [What the technical reviewer sees](#what-the-technical-reviewer-actually-sees)
- [Score loop](#the-score-loop)

---

```
  PR opened
       │
       ▼
  ① STAGING        git-diff-prep.py — classify → cluster → batch
       │
       ├─────────────────────────────────────┐
       ▼                                     ▼
  ② FILE REVIEW                      ② SYNTHESIS REVIEW
     per file, parallel batches         whole diff, round 1 only
     [CRITICAL] / [WARN] / [INFO]       no classification tags
       │                                     │
       ▼                                     ▼
  ③ FINALIZE                          ③ SCORE LOOP
     pre-filter [CRIT]/[WARN] only        writer → scorer → feedback
     → summary.md + index.md              up to 3× (tech) / 5× (summary)
     no scorer                            best attempt kept at cap
       │                                     │
       └──────────────┬──────────────────────┘
                      ▼
               ④ PUBLISH
                  summary.md → Bitbucket PR comment
```

---

## 🔍 Why Staging Exists

```
PR branch diff
       │
       ▼
  git-diff-prep.py
       │
       ├─ classify ──► route to skill (code / docs / tests)
       ├─ size ──────► full_and_diff │ diff_only │ diff_chunk
       └─ cluster ───► group related files into batches
       │
       ▼
  manifest.json  +  batch-plan.json  +  .txt input files
```

If a file is too large for `full_and_diff`, the agent only sees the diff — mitigations in unchanged lines are invisible, which is why the technical reviewer raises **"Confirm:"** questions it can't answer itself.

---

## 🔀 File Reviewers vs Synthesis Reviewers

```
┌──────────────────────────────┐   ┌──────────────────────────────┐
│      FILE REVIEWERS          │   │    SYNTHESIS REVIEWERS       │
│  pr-review-code              │   │  pr-review-technical         │
│  pr-review-docs              │   │  pr-review-summary           │
│  pr-review-tests             │   │  pr-review-blast-radius      │
│                              │   │                              │
│  one file at a time          │   │  all diffs at once           │
│  one <slug>.md per file      │   │  one doc for the whole PR    │
│  [CRITICAL] / [WARN] / [INFO]│   │  Confirm: questions only     │
└──────────────────────────────┘   └──────────────────────────────┘
         both start in round 1, no dependency between them
```



---

## ⚙️ The Execution Timeline

```
Round 1 (parallel):
  ├── pr-review-technical    ← whole diff, one tech-review.md
  ├── pr-review-summary      ← whole diff, one summary.md
  ├── pr-review-blast-radius ← whole diff, coupling analysis
  └── pr-review-code batch_001  ← first 15 files

Rounds 2+:
  └── pr-review-code batch_002, 003...  ← remaining files

Finalize (after all file batches):
  └── pre-filter to [CRITICAL]/[WARN] only → summary.md + index.md
      (hundreds of per-file reports would exhaust one agent context)

Score loop (synthesis outputs only):
  └── score → feedback → rewrite → repeat
```

---

## 🔍 What the Technical Reviewer Actually Sees

```
  full_and_diff  (file < 500 lines)        diff_only  (file too large)
  ┌────────────────────────────┐           ┌────────────────────────────┐
  │  unchanged code  ✓         │           │  unchanged code  ✗         │
  │  changed code    ✓         │           │  changed code    ✓         │
  │  mitigations visible       │           │  mitigations invisible     │
  │  → reviewer self-resolves  │           │  → reviewer raises Confirm:│
  └────────────────────────────┘           └────────────────────────────┘
```

**"Confirm:" questions are correct behaviour** — the reviewer defers to the human when context is missing. The question tells you exactly where to look.

---

## 🔄 The Score Loop

```
  writer agent
       │
       ▼
  tech-review.md / summary.md
       │
       ▼
  scorer agent  (sees only the output, not the writer's reasoning)
       │
       ├─ PASS ──────────────────────► publish
       │
       └─ FAIL ──► feedback.md
                        │
                        ▼
                   writer agent (reads feedback, rewrites silently)
                        │
                        └─ repeat up to 3× (tech) / 5× (summary)
                           best attempt kept when cap is reached
```

When the cap is hit without a pass, the highest-scoring attempt is kept.

---

→ For script-level detail on each stage: [advanced/04.1-pipeline-stages-reference.md](advanced/04.1-pipeline-stages-reference.md)
