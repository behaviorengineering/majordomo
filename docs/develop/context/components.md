# Components

<!-- typology:generated -->

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


| From | To | Rule |
|------|----|------|
| _(none)_ | | |

