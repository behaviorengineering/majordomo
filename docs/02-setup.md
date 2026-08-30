# Setup

*Majordomo — repository operations for evolving software.*

Target runtime: **GitHub Actions control tower** + **Go CLI** (`majordomo`). Majordomo is a control plane for evolving repositories; PR review is one workflow on that plane. See **[PLAN — Control Tower, GitHub Actions, and Go](PLAN-control-tower-github-go.md)**.

## What runs today

| Piece | Location | Notes |
|-------|----------|--------|
| Review skills / personas | `agents/` | Rubrics and templates |
| Go control plane | `cmd/majordomo`, `internal/` | `run review`, prep, orchestrate, dispatch, publish, poll, context digest |
| Agent dispatch | `pipelines/scripts/agent-dispatch.sh` | OpenCode wrapper; needs `MAJORDOMO_BIN` or `majordomo` on PATH for all-diffs |
| Agent + SA + forge images | `dockerfiles/` | Dual `public` / `corp` stages |
| Image CI | `.github/workflows/` | SA tools, agent, forge CLI (`gh` / `glab`) |

Pipeline Python is gone. Remaining bash is dispatch and image build only.

## Install the CLI

**Preferred (bootstrap / tower setup):** download a release binary from [GitHub Releases](https://github.com/behaviorengineering/majordomo/releases/latest), put `majordomo` on your `PATH`, then run `majordomo submodule` (or follow the control-tower pin steps in the [PLAN](PLAN-control-tower-github-go.md)).

Pushing a `v*` tag runs [`.github/workflows/release.yml`](../.github/workflows/release.yml) (GoReleaser). Version is injected into `internal/cli.Version` via ldflags.

**From source** (this repo):

```bash
go build -o majordomo ./cmd/majordomo
./majordomo version
```

## Local jobs (same command as CI)

Laptop and the tower review workflow both call `majordomo run review`. Publish is off unless you pass `--publish`. `--until` stops after a stage (`clone`, `sa`, `prep`, `waves`, `finalize`, `prose`, `synth`, `report`, `publish`).

```bash
# From the control-tower repo root (this layout).
go build -o majordomo ./.majordomo/cmd/majordomo

# Mechanical only (no LLM)
./majordomo run review \
  --config-dir majordomo-central-config \
  --repo-id polypus \
  --pr 123 \
  --workdir /path/to/polypus \
  --until prep

# Full review, do not comment on the PR
export ANTHROPIC_API_KEY=YOUR_KEY   # or OPENAI_API_KEY
./majordomo run review \
  --config-dir majordomo-central-config \
  --repo-id polypus \
  --pr 123 \
  --workdir /path/to/polypus

# Context digest (already a local CLI)
./majordomo context digest \
  --config-dir majordomo-central-config \
  --repo-id polypus \
  --workdir /path/to/polypus
```

Stage commands (`prep`, `dispatch`, `orchestrate`, `publish`) still exist for debugging one step. CI is checkout, build, secrets, then `majordomo run review --publish`.

## Embedded LLM gateway (Bifrost)

Judge (strop/DSPy) talks **OpenAI chat completions only** to an in-process Bifrost loopback. Real keys stay in the gateway Account:

| Env | Role |
|-----|------|
| `ANTHROPIC_API_KEY` | Anthropic (primary when present) |
| `OPENAI_API_KEY` | OpenAI |
| `GEMINI_API_KEY` / `GOOGLE_GENERATIVE_AI_API_KEY` / `GOOGLE_API_KEY` | Gemini |
| `MAJORDOMO_MODEL` | Logical model name (gateway maps + fallbacks) |

Provider retries and Anthropic→OpenAI→Gemini failover live in Bifrost. OpenCode children should use `aigateway.ChildEnv` (dummy `OPENAI_API_KEY` + `OPENAI_BASE_URL` to the loopback; real keys stripped).

## Observability and failure dumps

Tracing is on by default (Phoenix optional). On a failed `run review` / `orchestrate`, the full OpenInference trace is written to:

```
{output-dir}/logs/inference-failures/<trace_id>.json
```

or `logs/inference-failures/` when no output dir is set. Attach that JSON to an AI for debug. Set `MAJORDOMO_OTEL_ENDPOINT` (or `OTEL_EXPORTER_OTLP_ENDPOINT`) to `localhost:4317` when Phoenix is running. Disable with `MAJORDOMO_OTEL_ENABLED=0`.

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

**OpenCode LLM auth (agent job, per-run):** inject a provider API key as a secret/env into the agent container. Do not bake keys into the image. `agent-dispatch.sh` requires at least one of `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `OPENCODE_PROVIDER_API_KEY` (use the last for custom OpenAI-compatible gateways, with `baseURL` in `opencode.json` / `OPENCODE_CONFIG_CONTENT` via `{env:OPENCODE_PROVIDER_API_KEY}`). Served-repo SCM tokens are separate: `GH_TOKEN_<OWNER>` / `GITLAB_TOKEN_<OWNER>` (optional `MAJORDOMO_CREDENTIAL_<REPO_ID>`). Do not use unqualified `GH_TOKEN` / `GITLAB_TOKEN` for served repos.

**Publish (GitHub/GitLab):** run the review job inside `majordomo-gh` or `majordomo-glab` so `gh` / `glab` are on PATH. Build `./majordomo` in the job workspace; `majordomo publish --scm github|gitlab` shells to the forge CLI. Bitbucket publish remains HTTP.

## Submodule consumers

If an app repo still vendors this project as `.majordomo/`, use [03 — Manage Submodule](03-manage-submodule.md) (`majordomo submodule`) for pin/update. Prefer the control-tower model so served app repos stay clean.
