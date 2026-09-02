# Components

Review owns the packages that stage changes, run the judge, and publish review results. See [README.md](README.md) for the tree hub and [overview.md](overview.md) for the slice story.

## Owns

Typology `owns[]`: domain packages on this slice.

| Component | Path | Layer |
|-----------|------|-------|
| internal-staging | ./internal/staging | domain |
| internal-cluster | ./internal/cluster | domain |
| internal-diff | ./internal/diff | domain |
| internal-sa | ./internal/sa | domain |
| internal-satools | ./internal/satools | domain |
| internal-orchestrate | ./internal/orchestrate | domain |
| internal-filereview | ./internal/filereview | domain |
| internal-reviewrun | ./internal/reviewrun | domain |
| internal-judge | ./internal/judge | domain |
| internal-judge-evaluation-digest | ./internal/judge/evaluation/digest | domain |
| internal-judge-evaluation-summary | ./internal/judge/evaluation/summary | domain |
| internal-judge-evaluation-tech | ./internal/judge/evaluation/tech | domain |
| internal-judge-modules | ./internal/judge/modules | domain |
| internal-agent | ./internal/agent | domain |
| internal-workspace | ./internal/workspace | domain |
| internal-workspace-opencode | ./internal/workspace/opencode | domain |
| internal-report | ./internal/report | domain |
| internal-cache | ./internal/cache | domain |
| internal-publish | ./internal/publish | domain |
| internal-status | ./internal/status | domain |

## Surfaces

Typology `surfaces[]`: interaction packages grouped by kind. Domain packages stay under Owns.

| Surface | Kind | Components |
|---------|------|------------|
| _(none)_ | | |

## Cross-slice

| From | To | Kind |
|------|----|------|
| context | review | reads |
| operations | review | reads |
| review | context | reads |
| review | operations | reads |

The review bindings exist because the review workflow needs context-branch grounding and digest history, while the host boundary supplies config, SCM adapters, and publish hooks.

- `review -> context` reads because review jobs look up grounding packs and digest history from the context branch.
- `review -> operations` reads because review runs through shared config, SCM auth, and runtime helpers.
- `context -> review` reads because the context digest and gate path examines review outputs before it updates the branch.
- `operations -> review` reads because the host CLI and shared adapters launch and post the review workflow.

| From | To | Rule |
|------|----|------|
| _(none)_ | | |

The catalog has no cross-slice component bindings on this slice.
