# Proposal: Cluster-SHA Review Cache Stored in Git

*Majordomo — repository operations for evolving software.*

**Status:** Active (Partially Implemented)
**Date:** 2026-07-20
**Last Updated:** 2026-07-20
**Problem:** The pipeline re-runs analysis for clusters that have not changed, even when we already have prior results for those exact clusters.

---

## Implementation Status

Implemented:
- Project-isolated cache branch naming and validation (`majordomo-pr-reviewer-cache/<project-id>`).
- Skill-scoped cache layout (`<skill-name>/analysis-<sha>.json`).
- Cache validity predicate fields and metadata/frontmatter checks.
- Pre-analysis cache gate (retention resolution, pruning, metadata validation, index build).
- Constrained cache push path via `push-to-cache.py` with branch pattern enforcement.
- Configurable cache mode (`project`/`central`) and token credential wiring.

Pending / In Progress:
- Full script-level hardening for all remaining failure paths and edge cases.
- Finalized rollout checklist and ownership handoff across all onboarded repos.

---

## Acceptance and Rollout Criteria

Acceptance criteria:
- Cache-hit skips occur when cluster SHA and validity fields match.
- Cache misses analyze only unmatched clusters.
- Cache push never writes outside the allowed cache branch pattern.
- Cache failures degrade safely (miss + re-analyze) without breaking review publish flow.
- Pipeline script test suite remains green with cache and publish scenarios covered.

Rollout criteria:
- Default mode remains `cacheRepo: 'project'` unless a repo explicitly opts into `central`.
- Per-repo cache retention resolved and logged from configured precedence.
- One successful non-replay Jenkins run confirms skip/reuse behavior and summary publish.

Owner:
- Copilot pipeline maintainers (central config + stage scripts).

---

## Problem Statement

Current behavior:
- Review work can be repeated when the PR commit changes even if many clustered files are unchanged.
- Late-stage failures still force expensive analysis to run again for unchanged clusters.
- Cache decisions are too coarse at whole-PR commit level instead of cluster-content level.

Result: avoidable AI/API spend and longer build time.

---

## Proposed Solution: Cluster-Level Content Cache

Use the existing clustering output as the minimal unit of work.

Mandatory isolation rule:
- Each project must use its own dedicated cache branch.
- Cache artifacts for one project must never be written to or read from another project's branch.

---

## Cache Repo Hosting

Where the cache branch lives is configurable per project. Two choices:

| Option | Description | When to use |
|---|---|---|
| `project` | Cache branch lives in the **app repo** being reviewed | Natural location; cache survives alongside app history |
| `central` | Cache branch lives in the **central pipeline repo** | Single place to manage TTL and access; avoids app repo pollution |

Config key: `cache.cacheRepo: 'project'` or `cache.cacheRepo: 'central'`

Default: `project` — the pipeline already holds a write token for the app repo, so no extra credentials are needed.

To switch a team to `central` without touching their app repo, set `cacheRepo: 'central'` inside the `cache:` block in `majordomo-central-config/<repo-slug>.groovy`. This is the escalation path for teams that object to cache branches appearing in their app repo.

When `cacheRepo: 'project'`:
- Pipeline checks out the cache branch from the app repo SSH URL.
- Uses the same write token already configured for build-status notifications — no additional credential required.

When `cacheRepo: 'central'`:
- Pipeline checks out the cache branch from the central pipeline repo.
- Uses the pipeline repo SSH credential (already available in the central job).
- Requires an explicit opt-in: set `cacheRepo: 'central'` in the project config.

Both options produce the same branch naming and file structure. Only the remote URL differs.

---

For each cluster:
- Compute a deterministic cluster SHA from the cluster content fingerprint.
- Use that SHA as the cache key.
- Name the analysis result artifact with that SHA.
- Store the result artifact in git.

If the cluster SHA has not changed, reuse the stored result and skip analysis for that cluster.

---

## Cache Key and Artifact Naming

```text
cluster-sha = SHA256(fingerprint(cluster-files))
result-file = <skill-name>/analysis-{cluster-sha}.json
```

Cache branch layout:

```text
majordomo-pr-reviewer-cache/<project-id>  (branch root)
├── pr-review-code/
│   └── analysis-<sha>.json
├── pr-review-conf/
│   └── analysis-<sha>.json
└── pr-review-docs/
    └── analysis-<sha>.json
```

Notes:
- Files are grouped by skill name to keep the same cluster SHA distinct across skills.
- The same file cluster reviewed by different skills produces different outputs — the skill prefix prevents collision without encoding the skill name into the SHA itself.
- `fingerprint(cluster-files)` is `sorted("{path}:{git_blob_sha}" for each file)` joined by `\n`, then SHA256.
- Cache invalidation on instruction/prompt/rubric/model changes is handled by the validity predicate in the frontmatter — no separate version salt is needed.
- File format is JSON; naming must stay SHA-based.

### Result File Metadata (Frontmatter)

Each cached analysis file should include metadata frontmatter used for cache validation.

```yaml
---
cluster_sha: "<sha256>"
fingerprint_version: "v1"
cluster_files:
    - "src/a.py"
    - "src/b.py"
cluster_files_hash: "<sha256>"
model_id: "<model-name>"
model_revision: "<optional-revision>"
instruction_bundle_hash: "<sha256>"
prompt_template_hash: "<sha256>"
scoring_rubric_hash: "<sha256>"
output_schema_version: "v1"
analysis_payload_hash: "<sha256>"
created_at: "2026-07-18T12:34:56Z"
---
```

Notes:
- `instruction_bundle_hash` is computed from the full set of instruction files used during analysis.
- `analysis_payload_hash` is computed from the result body and prevents unnecessary rewrites when payload content is unchanged.
- `cluster_files` is the canonical, sorted list of files in the cluster at analysis time.
- `cluster_files_hash` is the hash of the normalized `cluster_files` list and should match the files used in the cluster fingerprint.

### Cache Validity Predicate

A cached result is valid only when all of the following match the current run context:
- `cluster_sha`
- `fingerprint_version`
- `cluster_files` and `cluster_files_hash`
- `model_id` and `model_revision` (when available)
- `instruction_bundle_hash`
- `prompt_template_hash`
- `scoring_rubric_hash`
- `output_schema_version`

If any field differs, treat as cache miss and regenerate.

### Immutable Write Policy

- Cache hit: do not modify the existing file.
- Cache miss: write a new file for that SHA.
- Regeneration path: if a rerun occurs and `analysis_payload_hash` is identical to the existing file, skip git update to avoid churn.

### Worked Example: `instruction_bundle_hash`

Goal: produce one deterministic hash for the exact instruction set used by analysis.

Input files (example):

```text
.github/instructions/copilot.instructions.md
.github/instructions/neurodivergent.instructions.md
agents/python-coder.agent.md
```

Computation recipe:
1. Resolve to canonical relative paths.
2. Sort paths lexicographically.
3. For each path in order, append to a byte buffer:
     - path line (`<path>\n`)
     - file length in bytes (`<length>\n`)
     - raw file bytes
     - separator (`\n--EOF--\n`)
4. Compute `SHA256(buffer)`.

Pseudo-code:

```text
files = sort([
    ".github/instructions/copilot.instructions.md",
    ".github/instructions/neurodivergent.instructions.md",
    "agents/python-coder.agent.md"
])

buffer = bytes()
for f in files:
    content = read_bytes(f)
    buffer += utf8(f + "\n")
    buffer += utf8(str(len(content)) + "\n")
    buffer += content
    buffer += utf8("\n--EOF--\n")

instruction_bundle_hash = sha256_hex(buffer)
```

This avoids false cache misses from file-order differences and catches real changes in instruction content.

---

## Lifecycle

### 0. Select project cache branch

Before any cache lookup or write:
- Resolve project identifier (for example: repo slug or service key).
- Map project identifier to dedicated cache branch (for example: `cache/<project-id>`).
- Checkout/sync only that project cache branch for cache operations.

This ensures strict project isolation and prevents cross-project contamination.

Example mapping:
- `institutional-lending-soap-integration-adapter-v1-api` -> `majordomo-pr-reviewer-cache/institutional-lending-soap-integration-adapter-v1-api`
- `web-legacy-integration-adapter-v1-api` -> `majordomo-pr-reviewer-cache/web-legacy-integration-adapter-v1-api`

Deterministic mapping rule:
- `project-id = lower(repoSlug).replace('_', '-').replace('/', '-')`
- `cache-branch = "majordomo-pr-reviewer-cache/${project-id}"`
- Reject cache read/write when resolved branch does not match this rule.

Project retention policy:
- Cache retention must be configurable per project (not global).
- Example config key: `cache_retention_days`.
- Cleanup and orphan-reset logic must read retention from the current project config.
- If project config is missing, use a safe default and log which default was applied.

Retention precedence (highest to lowest):
1. Project repo override (`cache_retention_days` in project-specific config).
2. Central pipeline default for that project in Jenkins central config.
3. Global default (for example: 180 days).

Validation rules:
- Reject non-numeric or negative values.
- Enforce a minimum floor (for example: 30 days) to avoid accidental aggressive pruning.
- Log the resolved value and source level (project, central, or global) for each run.

### 0.1 Pre-analysis cache gate (first cache check)

This gate runs before any file analysis begins.

Required actions:
- Resolve `cache_retention_days` using precedence rules.
- Prune expired cache files for the current project cache branch.
- Validate cache metadata format/frontmatter for remaining files.
- Build in-memory index of valid cache entries keyed by `cluster_sha`.

Only after this gate passes should the pipeline start cluster analysis decisions.

### 1. Discover clusters

Run existing clustering logic for the PR diff.

### 2. Compute cluster SHA

For each cluster, compute deterministic SHA from `fingerprint(cluster-files)` using sorted `path:blob_sha` pairs.

### 3. Restore / reuse

For each cluster SHA:
- Check whether `<skill-name>/analysis-{cluster-sha}.json` exists in the git-tracked cache location.
- If present, load cached result and mark cluster as `cache-hit`.
- If missing, mark as `cache-miss`.

### 4. Analyze only misses

Run expensive analysis only for `cache-miss` clusters.

### 5. Persist new results

Write one result file per newly analyzed cluster using the SHA-based filename and commit it to git.

### 6. Merge outputs

Build final PR output from:
- reused cached cluster results, and
- newly generated cluster results.

### 7. Persist to project-isolated branch

- Commit new cache artifacts only to that project's dedicated cache branch.
- Do not mix multiple projects in one cache branch.
- Do not read cache artifacts from default branch unless explicitly configured.

Write-back procedure (after each successful batch analysis):
1. Resolve the target repo URL from `cacheRepo` config (`project` or `central`).
2. Validate the target branch name against the cache branch naming rule.
3. Checkout or create the cache branch in an isolated worktree (does not touch the main workspace).
4. Write the SHA-named result file with frontmatter metadata.
5. Commit only that file.
6. Push via the constrained push helper (see Safe Push Mechanism below).
7. If push fails, log warning and continue — write-back is best-effort and non-fatal.

### Safe Push Mechanism

All git pushes to cache branches must go through `push-to-cache.py`.

This script is the sole allowed path for writing to cache branches. Direct `git push` from the pipeline is forbidden.

Constraints enforced by `push-to-cache.py`:
- Accepts exactly one argument: the target branch name.
- Validates the branch name against `^majordomo-pr-reviewer-cache/[a-z0-9][a-z0-9-]*$` before any git call.
- Hard-fails with exit code 1 if branch does not match the pattern.
- Constructs the push refspec as `HEAD:refs/heads/majordomo-pr-reviewer-cache/<project-id>` explicitly.
- Never accepts a free-form refspec from the caller.
- Uses `--force-with-lease` to avoid overwriting concurrent writes.
- Logs the resolved remote URL, branch, and refspec before pushing.

```text
python3 push-to-cache.py --remote <url> --branch majordomo-pr-reviewer-cache/<project-id> --worktree <path>
  → validates branch pattern
  → git -C <worktree> push <url> HEAD:refs/heads/majordomo-pr-reviewer-cache/<project-id> --force-with-lease
  → exit 0 on success, exit 1 on pattern violation or push failure
```

Environment variables read by `push-to-cache.py`:
- `BITBUCKET_TOKEN` — injected via Jenkins `withCredentials` before calling the script.
- Used as the password in `https://<service-account-user>:<token>@<host>` git remote URL.

YACC commit email compliance:
- HTTPS pushes authenticated with a personal access token are attributed to the token owner's registered Bitbucket email automatically.
- No `git user.email` configuration is required in the script — the service account email satisfies YACC's email matcher without any extra steps.

No credentials are stored, written, or logged by the script.

Jenkins callers must inject `BITBUCKET_TOKEN` via `withCredentials` before invoking the script and must not pass it as a CLI argument.

### 8. Prune by project retention before history reset

- Read `cache_retention_days` from the current project config.
- Delete cache files whose `created_at` is older than that threshold.
- Commit the pruned state.
- Perform orphan reset so trimmed files are removed from reachable branch history.

---

## Jenkins Behavior

Expected stage behavior:
- Clustering always runs (to know candidate units and SHAs).
- Cache gate runs immediately after clustering and before any analysis loop.
- Analysis stage loops over clusters and skips work for cache hits.
- Final summary/aggregation always runs using combined cached + fresh cluster artifacts.

Recommended execution order:
1. Cluster changed files.
2. Run pre-analysis cache gate (retention resolution, prune, metadata validation, cache index load).
3. For each cluster, check cache index first.
4. Analyze only misses.
5. Persist new cache files and assemble final output.

Minimal guard condition:

```groovy
if (clusterCacheExists(clusterSha)) {
    echo "Cache hit for ${clusterSha}; skipping analysis"
} else {
    runClusterAnalysis(cluster)
    writeClusterResult(clusterSha)
    pushToCacheBranch(clusterSha)  // calls push-to-cache.py — constrained push only
}
```

SSH credential wiring:
- `cacheRepo: 'project'` reuses `credentials.bitbucketTokenCredentialsId` (the existing write token) — no extra credential needed.
- `cacheRepo: 'central'` uses `credentials.bitbucketTokenCredentialsId` from central defaults — same token, different remote URL.
- An optional `credentials.cacheTokenCredentialsId` overrides both when a dedicated cache-only HTTP token is preferred.

---

## Failure Scenarios

| Scenario | Outcome |
|---|---|
| Cache file read fails | Log warning, treat as cache miss, re-analyze cluster |
| Cache file is corrupt | Log warning, treat as cache miss, overwrite with fresh result |
| Git commit/push of new cache files fails | Build can continue with computed results; cache update retried next run |
| Non-deterministic fingerprint input | Causes false misses; fix fingerprint normalization |
| `push-to-cache.py` branch pattern validation fails | Hard-fail — pipeline aborts; never pushes to wrong branch |
| SSH credential missing for cache push | Log error, skip push — analysis result is still available in workspace for this build |
| `cacheRepo` points to wrong remote URL | Push fails; log error, skip push — non-fatal |

Cache lookup failures must not fail the build.
Push validation failures (pattern mismatch) must always hard-fail — this is the security boundary.

---

## Tradeoffs

| | Cluster-SHA in Git | Whole-PR commit cache |
|---|---|---|
| Granularity | Fine (per cluster) | Coarse (whole PR) |
| Reuse on partial PR changes | High | Low |
| Storage growth | Many small files | Fewer larger artifacts |
| Traceability | Native git history | External artifact history |
| Invalidation on review logic changes | Via config version salt | Usually manual |

---

## Closed Decisions

1. ~~What exact cache directory should hold `analysis-{cluster-sha}.*` files?~~ **Resolved:** skill-scoped layout — `<skill-name>/analysis-{cluster-sha}.json` under the cache branch root.
2. ~~What is the canonical cluster fingerprint function in the current pipeline code?~~ **Resolved:** file paths + blob SHAs (Option B). Fingerprint input is `sorted("{path}:{git_blob_sha}" for each file in cluster)` joined by `\n`, then SHA256. This catches content changes within a stable cluster, not just membership changes.
3. ~~What value should be used for `review-config-version` and how is it versioned?~~ **Resolved:** not needed. Instruction, prompt, rubric, and model changes are all covered by the frontmatter validity predicate. No separate version salt required.
4. ~~Do we commit cache artifacts on every run or only on successful analysis stage completion?~~ **Resolved:** commit and push immediately after each individual batch analysis succeeds. Every artifact produced must be persisted so subsequent runs can skip that cluster regardless of whether later stages fail.
5. ~~What is the canonical branch naming convention?~~ **Resolved:** `majordomo-pr-reviewer-cache/<project-id>` — prefixed with the tool name for clear identification in any repo's branch list.
6. ~~Which file path and schema should define the project repo override for `cache_retention_days`?~~ **Resolved:** `cache.retentionDays` key in `majordomo-central-config/<repo-slug>.groovy`. Already wired into the stage with full precedence resolution (project → central → global → floor).
7. ~~Should `push-to-cache.py` use a dedicated `cacheSshCredentialsId` by default?~~ **Resolved:** reuse `bitbucketTokenCredentialsId` HTTP token for both `project` and `central` modes; `cacheTokenCredentialsId` is an optional per-repo override only. No SSH keys involved.
8. ~~Escalation path to switch to `cacheRepo: 'central'` without app-repo change?~~ **Resolved:** set `cacheRepo: 'central'` in `cache:` block of `majordomo-central-config/<repo-slug>.groovy`.
