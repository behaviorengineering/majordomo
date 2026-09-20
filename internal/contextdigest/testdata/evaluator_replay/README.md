# Evaluator replay fixtures

Captured `Predict:typology_quality*` Process spans from a digest work story
(`module-traces/*.jsonl`). Offline checks recorded outs; live replay runs the
full `typology_slice_catalog` EvaluateWorkflow (feedback + score + consolidator) using
the refine generator fixture I/O.

| Slug | Span |
|------|------|
| `typology_quality_feedback` | Feedback Analysis |
| `typology_quality_score` | Score Generation |
| `typology_quality_consolidator` | Consolidator |

## Live replay

```bash
MAJORDOMO_LIVE_EVALUATOR_REPLAY=1 \
MAJORDOMO_CENTRAL_CONFIG=/path/to/majordomo-central-config \
  go test ./internal/contextdigest/ -run LiveTypologyRefineEvaluateReplay -count=1 -v -timeout 15m
```

`MAJORDOMO_LIVE_GENERATOR_REPLAY=1` also enables this test.
