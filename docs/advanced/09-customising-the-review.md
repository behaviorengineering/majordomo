# Customising the Review

*Majordomo — repository operations for evolving software.*

All customisation goes in the `pipelines` block of *`.majordomo-config.groovy`*. Omit any key to keep the submodule default.

The pipeline ships **eight built-in skills** across three categories. Only `pr-review-code` is **routed by default** — all other skills require explicit routing configuration.

---

## 🏷️ Available Skills

**File-review skills** (routed by file type, produce per-file reports):

| Skill | Default routing | Blast radius |
|---|---|---|
| `pr-review-code` | Source code extensions (Python, JS, Java, Go, etc.) | Yes (mandatory) |
| `pr-review-conf` | Not routed by default. Configure explicitly. | No |
| `pr-review-docs` | Not routed by default. Configure explicitly. | No |
| `pr-review-tests` | Not routed by default. Configure explicitly. | No |

**Synthesis skills** (run automatically after file-review, not routed by file type):

| Skill | What it produces |
|---|---|
| `pr-review-summary` | `summary.md` — high-level PR summary written for developer and reviewer |
| `pr-review-technical` | `tech-review.md` — deep-dive: control flow, concurrency, test coverage gaps |
| `pr-review-blast-radius` | `blast-radius.md` — impact map across the changeset |

**Scoring skills** (used internally by iteration loops, not invoked directly):

| Skill | What it does |
|---|---|
| `pr-review-summary-score` | Scores `summary.md` against a rubric; drives the write/score iteration loop |
| `pr-review-technical-score` | Scores `tech-review.md` against a rubric; drives the tech-review iteration loop |

---

## ⚙️ Configuration Overrides

### Override credentials (central pipeline only)

Four credential keys are configurable per repo. All are optional — omit any key to inherit the org-wide value from `_defaults.groovy`.

| Key | When to override |
|---|---|
| `githubCopilotCredentialsId` | Repo needs a different GitHub Copilot token (e.g. different enterprise seat or scoped PAT) |
| `bitbucketSshCredentialsId` | Repo lives in a Bitbucket project that requires a different SSH key for checkout |
| `artifactoryCredentialsId` | Repo needs a different Artifactory token (e.g. different registry access tier) |
| `bitbucketTokenCredentialsId` | Repo uses a different service account to post PR comments (e.g. repo is in a restricted Bitbucket project) |

```groovy
credentials: [
    githubCopilotCredentialsId:  '<repo-specific-github-copilot-secret-text-credential-id>',
    bitbucketSshCredentialsId:   '<repo-specific-ssh-credential-id>',
    artifactoryCredentialsId:    '<repo-specific-artifactory-credential-id>',
    bitbucketTokenCredentialsId: '<repo-specific-bitbucket-token-credential-id>',
],
```

---

### Override which files each skill receives

`git-diff-prep.py` classifies each changed file using glob patterns. **First matching glob wins.**

Simple form — globs only:

```groovy
pipelines: [
    'pr-review': [
        routing: [
            'pr-review-docs': ['**/*.md', '**/*.rst'],
            'pr-review-conf': ['**/*.yml', '**/*.yaml', '**/*.toml', '**/*.json', 'docs/**'],
            'pr-review-code': ['**'],  // catch-all — must be last
        ],
    ],
],
```

Extended form — add an optional `persona` per skill to control reviewer tone and behaviour. The persona file is loaded from disk at staging time and injected as a behavioural preamble into the agent prompt before the diff:

```groovy
pipelines: [
    'pr-review': [
        routing: [
            'pr-review-docs': [globs: ['**/*.md', '**/*.rst'], persona: '.majordomo/personas/doc-reviewer.md'],
            'pr-review-code': [globs: ['**'],                  persona: '.majordomo/personas/strict-security.md'],
            'pr-review-conf': ['**/*.yml', '**/*.yaml'],        // no persona — short form still valid
        ],
    ],
],
```

A missing or empty persona file fails the build immediately. Omit the `persona` key to use the agent's built-in tone.

---

### Inject team or domain context into the reviewer

`agentContext` tells the review agent domain-specific facts before it reads the diff. Use it to steer the reviewer toward the standards and constraints that matter for a given area of the codebase.

Two formats are supported:

**Global context** — applies to every reviewed file:

```groovy
pipelines: [
    'pr-review': [
        agentContext: [
            global: [
                customRules: [
                    'No credentials hardcoded in source or configuration.',
                ],
            ],
        ],
    ],
],
```

**Glob-scoped context** — first matching glob wins, merged on top of global:

```groovy
pipelines: [
    'pr-review': [
        agentContext: [
            global: [
                customRules: [
                    [file: '.majordomo/rules/shared/security-baseline.md'],
                ],
            ],
            scoped: [
                'services/payments-api/**': [
                    techStack:   ['python', 'fastapi', 'openapi'],
                    reviewFocus: ['openapi-contract', 'auth'],
                    customRules: [
                        [file: '.majordomo/rules/mesh/mesh-api-contract.md'],
                        'FastAPI error handling must use exception_handlers, not Flask-style decorators.',
                    ],
                ],
                'sops/**': [
                    techStack:   ['docs', 'runbooks'],
                    reviewFocus: ['procedural-accuracy', 'rollback-completeness'],
                    customRules: [
                        'Every operational procedure must include rollback or recovery guidance.',
                    ],
                ],
            ],
        ],
    ],
],
```

**`customRules` supports two entry types:**

| Type | Example | Description |
|---|---|---|
| Inline string | `'No hardcoded credentials.'` | Rule text embedded directly in the config |
| File reference | `[file: '.majordomo/rules/mesh-api.md']` | Content loaded from disk at staging time |

File references are resolved relative to the app repo root. A missing or empty file fails the build immediately with a clear error — no silent fallback.

**Merge behaviour:**
- `global.customRules` are prepended to any scoped `customRules` for the matched file.
- Scoped keys (`techStack`, `reviewFocus`) override the global value for that key entirely.
- Files with no matching scoped glob receive only global context.

The resolved context is embedded in each task's manifest entry and injected into the review prompt as a preamble before the diff.

### Override a skill's review rules

Point to your own skill directory (must contain a `SKILL.md`):

```groovy
pipelines: [
    'pr-review': [
        skills: [
            'pr-review-conf': 'agents/skills/my-conf',  // path relative to your app repo root
            'pr-review-docs': null,
            'pr-review-code': null,                      // null = use submodule default
        ],
    ],
],
```

### Override the orchestrator agent

Replace the shared review protocol itself:

```groovy
pipelines: [
    'pr-review': [
        agent: 'agents/my-pr-review.agent.md',  // path relative to your app repo root
    ],
],
```

### Override the model

Four independent model keys control different pipeline phases. The org-wide defaults live in `majordomo-central-config/_defaults.groovy`. A per-repo config only needs to set the keys it wants to change — deep merge inherits the rest.

| Key | Modes | Fallback |
|-----|-------|---------|
| `model` | file-review batches, finalize, prose | — |
| `summaryModel` | summary synthesis | `model` |
| `technicalModel` | technical + technical-deep | `model` |
| `scoreModel` | score + tech-score | — |

**Central pipeline (per-repo config):**
```groovy
pipelines: [
    'pr-review': [
        model:          'gpt-5-mini',
        summaryModel:   'claude-sonnet-4.5',
        technicalModel: 'claude-sonnet-4.6',
        scoreModel:     'auto',
    ],
],
```

**Submodule / per-repo pipeline (`.majordomo-config.groovy`):**
```groovy
pipelines: [
    'pr-review': [
        model:          'gpt-5-mini',
        summaryModel:   'claude-sonnet-4.5',
        technicalModel: 'claude-sonnet-4.6',
        scoreModel:     'auto',
    ],
],
```

All keys are optional. Omit any key to keep the org-wide default.

---

## � Caching Configuration

To save LLM tokens and accelerate build times, the pipeline clusters changed files and caches analysis/markdown outputs inside a dedicated git branch per project. Caching behaviors are configured under the `cache` block.

### Caching Configuration Options

| Key | Type | Default | Description |
|---|---|---|---|
| `cacheRepo` | `'project' \| 'central'` | `'project'` | Where the cache branch lives. `'project'` uses the app repo; `'central'` uses the central repo. |
| `enableSkips` | `boolean` | `true` | When true, skips analysis entirely for unchanged file clusters and restores cached reports. |
| `enableContinuousRuns` | `boolean` | `false` | When true, new commits update the PR summary rather than bypassing the review. |
| `lockResource` | `string` | `'copilot-cache-{projectId}'` | Shared lock template for serializing concurrent cache flushes. Supports `{projectId}` and `{cacheBranch}` placeholders. |
| `cacheTokenCredentialsId` | `string` | `bitbucketTokenCredentialsId` | Target credentials ID inside Jenkins for pushing cache updates. |
| `retentionDays` | `int` | `180` | Number of days to retain cached entries before pruning. (Floor limit of 30 days is enforced). |

### Caching Configuration Example

Configure overrides in your **`.majordomo-config.groovy`** or the central config **`majordomo-central-config/<repo-slug>.groovy`**:

```groovy
cache: [
    cacheRepo: 'project',
    enableSkips: true,
    enableContinuousRuns: false,
    lockResource: 'copilot-cache-{projectId}',
    retentionDays: 90
]
```

---

## �📦 Adding a New Pipeline

Add an entry under `pipelines` with its own orchestrator, skills, and routing. Create the corresponding `SKILL.md` files in your app repo and the dispatcher picks them up **automatically**. **No changes to the submodule required.**

---

## ⚠️ Exclusion Filters

The exclusion filters for which files are sent to any skill are in `scripts/git-diff-prep.py` under `EXCLUDE_PATTERNS`. Add patterns there to skip generated files, lock files, or other paths that **don't need review**.
