# Components

<!-- typology:generated -->

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


| From | To | Rule |
|------|----|------|
| _(none)_ | | |

