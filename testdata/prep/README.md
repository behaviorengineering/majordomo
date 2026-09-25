# Prep fixtures

`internal/staging` golden coverage builds a temporary git fixture (see
`TestPrepProducesReviewableManifest`), runs `staging.Run`, and asserts the
batch plan and manifest contain reviewable skills and files.
