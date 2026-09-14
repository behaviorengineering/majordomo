# RLM TraceDir replay fixtures

Captured RLM `TraceDir` metadata (`context` + `query`) and `final_answer` from a
local digest work story (`rlm-traces/<task>/*.jsonl`). Used to:

1. Offline: run recorded finals through Go parsers (no LLM).
2. Live: `CreateRLMModule` + `RLMComplete` against Polypus with the same query.

Covered today (gitboard seed):

| Task | Fixture | Notes |
|------|---------|-------|
| `typology_inspect` | `gitboard_typology_inspect_span.json` | `internal/cliexec` |
| `bootstrap_story` | `gitboard_bootstrap_story_span.json` | `readme` section |
| `typology_cluster_audit` | `gitboard_typology_cluster_audit_span.json` | synthetic recorded YAML (seed had no `final_answer`) |

`typology_objective_grounding` skips until a TraceDir dump exists.

**Truncation:** dspy-go TraceDir session metadata still stores ~503 chars of
context ending in `...`. After the strop `RLMComplete` sidecar lands, prefer
`rlm-traces/<task>/rlm_inputs.jsonl` (`type=inputs`) for full context + query,
and pair with `type=result` for `final_answer`. Until the next digest reseed
with that strop build, fixtures here may still use truncated metadata.

## Live replay

```bash
cd tmp/majordomo-cluster-wt   # or .majordomo
MAJORDOMO_LIVE_RLM_REPLAY=1 \
MAJORDOMO_CENTRAL_CONFIG=/path/to/majordomo-central-config \
  go test ./internal/contextdigest/ -run LiveDigestRLMReplay -count=1 -v -timeout 30m
```

Each live subtest writes a fresh TraceDir under the test temp `rlm-traces/<task>/`.
