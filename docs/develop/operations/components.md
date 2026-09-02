# Components

Operations is the bounded context for the Majordomo control plane, and operators use its CLI surface to run polling, review, and context jobs. See [README.md](README.md) for the tree hub and [overview.md](overview.md) for the slice story.

## Owns

Typology `owns[]`: domain packages on this slice.

| Component | Path | Layer |
|-----------|------|-------|
| internal-poll | ./internal/poll | domain |
| internal-outbound | ./internal/outbound | domain |
| internal-githttps | ./internal/githttps | domain |
| internal-config | ./internal/config | domain |
| internal-aigateway | ./internal/aigateway | domain |
| internal-observability | ./internal/observability | domain |
| internal-submodule | ./internal/submodule | domain |

## Surfaces

Typology `surfaces[]`: interaction packages grouped by kind. Domain packages stay under Owns.

| Surface | Kind | Components |
|---------|------|------------|
| operations-cli | cli | ./internal/cli, ./cmd/majordomo |

## Cross-slice

| From | To | Kind |
|------|----|------|
| context | operations | reads |
| operations | context | reads |
| operations | review | reads |
| review | operations | reads |

The operations bindings exist because the control plane sits between the review workflow, context branch jobs, and the shared host boundary.

- `operations -> review` reads because poll and the CLI launch review runs and publish results.
- `review -> operations` reads because review jobs depend on shared config, SCM auth, and telemetry.
- `operations -> context` reads because the host boundary triggers context digests and branch updates.
- `context -> operations` reads because context jobs use the same runtime hooks and config from the control plane.

| From | To | Rule |
|------|----|------|
| _(none)_ | | |

The catalog has no cross-slice component bindings on this slice.
