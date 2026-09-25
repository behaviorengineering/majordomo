# CLI

Operators run Majordomo through the `operations-cli` surface (`./cmd/majordomo` and `./internal/cli`). The root binary wires cobra commands; domain work stays in the `review` and `context` slices.

| Component | Path |
|-----------|------|
| internal-cli | ./internal/cli |
| cmd-majordomo | ./cmd/majordomo |

## Commands

| Command | Purpose |
|---------|---------|
| `majordomo version` | Print binary version |
| `majordomo poll` | Poll SCM APIs for open PRs/MRs that need review |
| `majordomo prep <base-branch> <staging-dir>` | Classify diffs, cluster files, write staging manifest |
| `majordomo dispatch <pr-number> <staging-dir> <output-dir>` | Run one Judge batch (in-process strop) |
| `majordomo orchestrate` | Run review waves, checkpoints, finalize, and synthesis loops |
| `majordomo run review` | Clone if needed, run SA, orchestrate, optionally publish |
| `majordomo sa` | Run staticAnalysis tools from central config into `.sa/` |
| `majordomo publish <pr-number> <summary-file> <mode>` | Publish summary to PR/MR |
| `majordomo status <commit-sha> <state>` | Post commit/check status |
| `majordomo cache …` | Review and poll cache on the served repo (`validate-branch`, `push`, `poll-get`, `poll-set`, `precheck`, `lookup`, `store`, `restore`) |
| `majordomo context validate` | Validate a context-branch worktree |
| `majordomo context digest` | Catch up the context branch when the cursor is behind default HEAD |
| `majordomo context gate` | Show gate.json sidecar state |
| `majordomo context repos` | List served repos eligible for context digest |
| `majordomo report junit\|html\|all-diffs` | Convert findings and staging diffs into report artifacts |
| `majordomo build-sa-tools` | Build local SA tool Docker images |
| `majordomo submodule` | Interactive manager for a vendored `.majordomo` submodule |

See [components.md](components.md) for the package inventory and [overview.md](overview.md) for the control-plane story.
