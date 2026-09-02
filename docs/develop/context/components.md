# Components

Context owns the packages that keep the repository context branch fresh, grounded, and gate-checked. See [README.md](README.md) for the tree hub and [overview.md](overview.md) for the slice story.

## Owns

Typology `owns[]`: domain packages on this slice.

| Component | Path | Layer |
|-----------|------|-------|
| internal-contextstore | ./internal/contextstore | domain |
| internal-contextdigest | ./internal/contextdigest | domain |
| internal-contextgate | ./internal/contextgate | domain |
| internal-agenting | ./internal/agenting | domain |

## Surfaces

Typology `surfaces[]`: interaction packages grouped by kind. Domain packages stay under Owns.

| Surface | Kind | Components |
|---------|------|------------|
| _(none)_ | | |

## Cross-slice

| From | To | Kind |
|------|----|------|
| context | operations | reads |
| context | review | reads |
| operations | context | reads |
| review | context | reads |

The context bindings exist because the context branch feeds review with grounding and digest history, while the host boundary supplies the config and runtime hooks needed to keep that branch moving.

- `context -> review` reads because review jobs consume grounding packs and digest history from the context branch.
- `review -> context` reads because review results feed the context digest and gate path before the branch updates.
- `context -> operations` reads because the digest and gate flow rely on shared config, SCM auth, and host runtime behavior.
- `operations -> context` reads because the control plane triggers and manages context updates through the same runtime boundary.

| From | To | Rule |
|------|----|------|
| _(none)_ | | |

The catalog has no cross-slice component bindings on this slice.
