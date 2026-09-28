# Pipeline trajectory (strop consumer)

Digest and other hosts attach `orchestration.WithPipelineTrajectory` for the run session. strop records `PipelineAttempt` entries from composition and stepplan loops with `FailureClass`:

- `transient_infra`: pace/retry only; do not run LLM compensate plans.
- `semantic_gate`: normal refine, then optional `PhaseCompensator` (digest cluster merge uses `clusterOutFromCompensateApply`).
- `hard` / `cancelled`: fail closed or stop.

Resume: read `TrajectoryFromContext(ctx).Snapshot()` plus existing checkpoints (`workspace.yaml`, stepplan files).

Pin `github.com/behaviorengineering/strop` at `v0.5.8` or newer for `FailureClass`, `PipelineTrajectory`, and composition trajectory hooks.
