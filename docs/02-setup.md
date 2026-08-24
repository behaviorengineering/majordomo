# Setup

*Majordomo — repository operations for evolving software.*

> **Jenkins is retired.** Pipeline-from-SCM jobs, Generic Webhook Trigger setup, `.majordomo-config.groovy`, and the `setup-majordomo*.py` Jenkins API helpers have been removed from this repository.
>
> Target runtime: **GitHub Actions control tower** + **Go CLI**. Follow **[PLAN — Control Tower, GitHub Actions, and Go](PLAN-control-tower-github-go.md)**.

## What still runs today

| Piece | Location | Notes |
|-------|----------|--------|
| Review skills / personas | `agents/` | Unchanged |
| Staging / dispatch / publish scripts | `pipelines/scripts/` | CI-agnostic Python/bash |
| Agent + SA Docker images | `dockerfiles/` | Dual `public` / `corp` stages |
| Image CI | `.github/workflows/` | `--target public` on GitHub-hosted runners |
| Go control plane | `cmd/majordomo`, `internal/` | Prep/report first; poll/orchestrate next |

## Local image builds

```bash
# SA tools (public)
python scripts/build-sa-tools.py

# Copilot / agent image (public)
DOCKER_BUILD_TARGET=public SKIP_PUSH=true \
  bash pipelines/scripts/build-copilot-image.sh \
    local copilot-cli local-test dockerfiles/copilot-cli.Dockerfile
```

Corp agents: pass `PACKAGE_REGISTRY_HOST`, `CORP_CA_CERT_URL`, `DEBIAN_REPO_PATH`, `PIP_INDEX_PATH`, `NPM_VIRTUAL_PATH`, `DOCKER_PULL_DOMAIN`, and registry credentials — see Dockerfile headers.

## Submodule consumers

If an app repo still vendors this project as `.majordomo/`, keep using [03 — Manage Submodule](03-manage-submodule.md) for pin/update. Do **not** add Jenkins job script paths; orchestration moves to the control-tower workflows described in the plan.

## Historical docs

Proposal and revision notes under `docs/PROPOSAL-*` and `docs/REVISIONS-*` are archived Jenkins-era material (banner at top of each file). Operational guides under `docs/advanced/` describe the script-based pipeline and control-tower target.
