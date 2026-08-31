# Control-tower GitHub Actions (reference)

Majordomo **directs** what control-tower workflows should do; each tower **implements** them under `.github/workflows/` with org-specific registry tags, secrets, and config paths.

## Split of responsibility

| Layer | Owns |
|-------|------|
| **Majordomo** (`.majordomo/` submodule) | Forge Dockerfiles, `build-copilot-image.sh`, CLI stages, scripts in `pipelines/scripts/tower/`, reference YAML in this directory |
| **Tower** | Workflow entrypoints, `majordomo-central-config/`, GHCR (or other) image tags, org secrets and repository variables |

Tower workflows should call shared scripts instead of copying bash:

```bash
bash .majordomo/pipelines/scripts/tower/verify-forge-cli.sh
bash .majordomo/pipelines/scripts/tower/enrich-context-repos-matrix.sh context-repos.json
bash .majordomo/pipelines/scripts/tower/clone-served-repo.sh served/repo
```

## Repository variables (tower)

After publishing forge images from the tower repo:

- `MAJORDOMO_GH_IMAGE` — e.g. `ghcr.io/<org>/<tower>/majordomo-gh:latest`
- `MAJORDOMO_GLAB_IMAGE` — e.g. `ghcr.io/<org>/<tower>/majordomo-glab:latest`

## Reference workflows

Copy or diff against these files when adding or updating tower workflows:

| File | Purpose |
|------|---------|
| [majordomo-forge-images.yml](majordomo-forge-images.yml) | Build/push `majordomo-gh` and `majordomo-glab` to your registry |
| [majordomo-review.yml](majordomo-review.yml) | PR review inside SCM-matching forge container |
| [majordomo-context-digest.yml](majordomo-context-digest.yml) | Scheduled context digest (matrix by served repo) |
| [majordomo-context-gate.yml](majordomo-context-gate.yml) | Single-repo digest acceleration |

Replace `YOUR_GHCR_REGISTRY` (e.g. `ghcr.io/xynova/majordomo-tower`) in the forge-images workflow.

## Sync habit

When bumping the `.majordomo` submodule pin, re-diff tower `.github/workflows/majordomo-*.yml` against this directory. Script behavior changes with the pin; workflow shape should stay aligned.

See also [PLAN — Control Tower](../docs/PLAN-control-tower-github-go.md) and tower [onboarding-pull-mode.md](../../../docs/onboarding-pull-mode.md) in a consumer repo.
