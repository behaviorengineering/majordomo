# Chronology

Newest first.

### 2026-09-01 - majordomo - 6792391c5baa

- **Did:** Ship majordomo CLI image and fix GitLab glab/git auth (#9)
- **Because:** * Add majordomo CLI image and tower extract helper.
- **In order to:** advance context cursor on default first-parent tape
- **Evidence:** commit 6792391c5baa6535dd19980b9c792b5090165b8c; files: .github/workflows/majordomo-forge-cli.yml, dockerfiles/Dockerfile.cli, internal/cli/root.go, internal/contextdigest/forge.go, internal/contextdigest/run_test.go, internal/githttps/auth.go, internal/githttps/auth_test.go, pipelines/github-actions/tower/README.md, pipelines/github-actions/tower/majordomo-cli-image.yml, pipelines/github-actions/tower/majordomo-context-digest.yml, pipelines/github-actions/tower/majordomo-context-gate.yml, pipelines/github-actions/tower/majordomo-review.yml, pipelines/scripts/build-copilot-image.sh, pipelines/scripts/tower/extract-majordomo-cli.sh

### 2026-09-01 - majordomo - 7a1fc98ab39f

- **Did:** Add auto-patch releases for agent-oriented version tags. (#10)
- **Because:** Wire manage-go-releases practice: patch on releasable main merges, skip docs/chore/ci, keep manual tag release, and document the policy for agents.
- **In order to:** advance context cursor on default first-parent tape
- **Evidence:** commit 7a1fc98ab39f524e01ab48f04716eaa8a1e94d96; files: .github/workflows/auto-patch-release.yml, README.md
