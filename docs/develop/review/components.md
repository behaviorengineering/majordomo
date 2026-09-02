# Components

<!-- typology:generated -->

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


| From | To | Rule |
|------|----|------|
| _(none)_ | | |

