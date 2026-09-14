# Cluster CoT replay fixtures

Legacy fixture for the offline **recorded `none`** merge gate. Prefer
`testdata/generator_replay/` for new spans and the shared live suite
(`MAJORDOMO_LIVE_GENERATOR_REPLAY=1`, `-run LiveDigestGeneratorReplay`).

## Live replay (compat)

```bash
MAJORDOMO_LIVE_CLUSTER_REPLAY=1 \
  go test ./internal/contextdigest/ -run LiveTypologyClusterReplay -count=1 -v -timeout 5m
```
