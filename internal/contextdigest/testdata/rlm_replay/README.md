# RLM TraceDir replay fixtures

Captured RLM `TraceDir` metadata (`context` + `query`) and `final_answer` from a
local digest work story (`rlm-traces/<task>/*.jsonl`). Used to:

1. Offline: run recorded finals through Go parsers (no LLM).
2. Live: `CreateRLMModule` + `RLMComplete` against Polypus with the same query.

Covered today (gitboard seed):

| Task | Fixture | Notes |
|------|---------|-------|
| `typology_inspect` | `gitboard_typology_inspect_span.json` | `internal/cliexec` |
| `bootstrap_story` | `gitboard_bootstrap_story_span.json` | `readme` section; query = `bootstrap_story_rlm_v2` raw-markdown contract; recorded answer still YAML `markdown: \|` leak for offline unwrap |
| `bootstrap_story` | `gitboard_bootstrap_story_*_envelope_span.json` | envelope leaks from local-seed `20260917-041109Z` (`mission`, `architecture`, `grounding`; truncated context; queries aligned to v2) |
| `typology_slice_grouping_audit` | `gitboard_typology_slice_grouping_audit_span.json` | synthetic recorded YAML (seed had no `final_answer`) |

`typology_slice_meaning` skips until a TraceDir dump exists.

**Prompt contract:** Live bootstrap_story replay uses the main span `query` (refreshed for
`bootstrap_story_rlm_v2`: raw markdown body, no YAML/`markdown: |` envelope). Recorded
finals on all bootstrap_story spans stay as leaked envelopes so offline unwrap stays covered.

**Envelope fixtures:** `gitboard-local-seed-20260917-041109Z` recorded finals that wrap section
bodies in `yaml` / `markdown: |`. Offline tests (`TestRLMReplayFixturesOffline`,
`TestBootstrapStoryEnvelopeFixturesOffline`) assert `parseBootstrapStoryMarkdownAnswer`
strips those wrappers so stored story markdown never keeps the envelope.

**Truncation:** Prefer TraceDir/`rlm_inputs.jsonl` (`type=inputs` + `type=result`)
for full context. Fixtures refreshed from `gitboard-20260914-143414Z` keep full
context for inspect and cluster_audit. Bootstrap `readme` context is full from
that run; `recorded_final_answer` is from a prior successful seed because this
reseed errored on the README RLM step. Envelope spans intentionally use a short
stub context (`context_truncated: true`) for parser-only offline coverage.

## Live replay

```bash
cd tmp/majordomo-cluster-wt   # or .majordomo
MAJORDOMO_LIVE_RLM_REPLAY=1 \
MAJORDOMO_CENTRAL_CONFIG=/path/to/majordomo-central-config \
  go test ./internal/contextdigest/ -run LiveDigestRLMReplay -count=1 -v -timeout 30m
```

Each live subtest writes a fresh TraceDir under the test temp `rlm-traces/<task>/`.
