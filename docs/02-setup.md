# Setup

*Majordomo — repository operations for evolving software.*

Target runtime: **GitHub Actions control tower** + **Go CLI** (`majordomo`). Majordomo is a control plane for evolving repositories; PR review is one workflow on that plane. See **[PLAN — Control Tower, GitHub Actions, and Go](PLAN-control-tower-github-go.md)**.

## What runs today

| Piece | Location | Notes |
|-------|----------|--------|
| Review skills / personas | `agents/` | Rubrics and templates |
| Go control plane | `cmd/majordomo`, `internal/` | prep, orchestrate, dispatch, publish, status, cache, poll, report, build-sa-tools, submodule |
| Agent dispatch | `pipelines/scripts/agent-dispatch.sh` | OpenCode wrapper; needs `MAJORDOMO_BIN` or `majordomo` on PATH for all-diffs |
| Agent + SA + forge images | `dockerfiles/` | Dual `public` / `corp` stages |
| Image CI | `.github/workflows/` | SA tools, agent, forge CLI (`gh` / `glab`) |

Pipeline Python is gone. Remaining bash is dispatch and image build only.

## Build the CLI

```bash
go build -o majordomo ./cmd/majordomo
./majordomo version
```

## Local image builds

```bash
# SA tools (public)
majordomo build-sa-tools

# OpenCode agent image (public)
DOCKER_BUILD_TARGET=public SKIP_PUSH=true \
  bash pipelines/scripts/build-copilot-image.sh \
    local majordomo-agent local-test dockerfiles/Dockerfile.agent

# Forge CLI images (public) — job containers for publish; majordomo binary is built in-job
DOCKER_BUILD_TARGET=public SKIP_PUSH=true \
  bash pipelines/scripts/build-copilot-image.sh \
    local majordomo-gh local-test dockerfiles/Dockerfile.gh
DOCKER_BUILD_TARGET=public SKIP_PUSH=true \
  bash pipelines/scripts/build-copilot-image.sh \
    local majordomo-glab local-test dockerfiles/Dockerfile.glab
```

Corp agents/forge: pass `PACKAGE_REGISTRY_HOST`, `CORP_CA_CERT_URL`, `DEBIAN_REPO_PATH`, `NPM_VIRTUAL_PATH` (agent only), `DOCKER_PULL_DOMAIN`, and registry credentials — see Dockerfile headers.

**OpenCode LLM auth (agent job, per-run):** inject a provider API key as a secret/env into the agent container. Do not bake keys into the image. `agent-dispatch.sh` requires at least one of `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `OPENCODE_PROVIDER_API_KEY` (use the last for custom OpenAI-compatible gateways, with `baseURL` in `opencode.json` / `OPENCODE_CONFIG_CONTENT` via `{env:OPENCODE_PROVIDER_API_KEY}`). SCM tokens (`GITHUB_TOKEN`, etc.) stay separate for clone/publish/status.

**Publish (GitHub/GitLab):** run the review job inside `majordomo-gh` or `majordomo-glab` so `gh` / `glab` are on PATH. Build `./majordomo` in the job workspace; `majordomo publish --scm github|gitlab` shells to the forge CLI. Bitbucket publish remains HTTP.

## Submodule consumers

If an app repo still vendors this project as `.majordomo/`, use [03 — Manage Submodule](03-manage-submodule.md) (`majordomo submodule`) for pin/update. Prefer the control-tower model so served app repos stay clean.
