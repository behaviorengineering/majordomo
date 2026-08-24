# Prep golden fixtures

The Go/Python prep parity check lives in `internal/staging/golden_test.go`.

It builds a temporary git repo (initial `main` + `feature` branch with code/docs/config
changes), runs `staging.Run` and `pipelines/scripts/git-diff-prep.py` against
`origin/main...HEAD`, then compares `batch-plan.json` skill/batch metadata and
`manifest.json` reviewable files / agents.

No committed tree is required under this directory; the test is self-contained.
