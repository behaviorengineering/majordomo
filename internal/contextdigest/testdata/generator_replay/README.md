# Generator CoT replay fixtures

Captured `Predict:<task>` Process spans from a local digest work story
(`module-traces/*.jsonl`). Used to:

1. Offline: load recorded outputs through Go gates (no LLM).
2. Live: re-run that generator alone against Polypus with the same inputs.

Covered today (gitboard seed):

| Task | Fixture |
|------|---------|
| `typology_slice_grouping` | `gitboard_typology_slice_grouping_span.json` |
| `typology_slice_catalog` | `gitboard_typology_slice_catalog_span.json` |

Other `judge.DigestTasks()` still appear as skipped subtests until you drop a
matching `gitboard_<task>_span.json` here. RLM tasks dump under `rlm-traces/` and
use `testdata/rlm_replay/` (`MAJORDOMO_LIVE_RLM_REPLAY=1`).

The older `testdata/cluster_replay/` fixture keeps the recorded `none` merge
gate for regression.

## Live replay

Polypus must be up (`http://127.0.0.1:1320/v1/models`).

```bash
cd tmp/majordomo-cluster-wt   # or .majordomo
MAJORDOMO_LIVE_GENERATOR_REPLAY=1 \
MAJORDOMO_CENTRAL_CONFIG=/path/to/majordomo-central-config \
  go test ./internal/contextdigest/ -run LiveDigestGeneratorReplay -count=1 -v -timeout 15m
```

Compat: `MAJORDOMO_LIVE_CLUSTER_REPLAY=1` also enables the suite (and the
cluster-only `LiveTypologyClusterReplay` test).

Each live subtest writes a fresh TraceSession under the test temp
`module-traces/` and logs output fields.
