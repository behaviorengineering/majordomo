<p align="center">
  <img src="media/banner.webp" alt="Majordomo — Simple. Dapper. Code Steward." width="100%" />
</p>

# Majordomo — repository operations for evolving software.

Majordomo is a **control plane for software repositories that keep changing**. It holds durable org config and cache, polls or reacts to change across git hosts, and runs jobs against a served repo. **Structured PR review is one shipped workflow**, not the whole product.

Runtime today: **Go CLI** (`majordomo`) on a **GitHub Actions control tower**, with OpenCode for agent jobs and focused Docker images for SA and forge CLIs.

---

**[Architecture plan](docs/PLAN-control-tower-github-go.md)** — control tower, GitHub Actions, Go CLI, SCM-agnostic ops.  
**[Portable pipeline pattern](docs/01-portable-pipeline-pattern.md)** — shared engine, clean served repos.

---

## 💡 Why This Exists

**Repos evolve; one-off scripts do not.** Teams need a durable place to onboard repos, keep credentials and policy in one tower, remember what already ran (poll cursors, review cache), and grow new jobs without polluting every application default branch.

Majordomo separates:

| Layer | Role |
|-------|------|
| **Control tower** | Org YAML, secrets, GHA workflows; pins this repo as `.majordomo/` |
| **Go control plane** | Deterministic jobs: poll, prep, orchestrate, publish, status, cache, report, tooling |
| **Agent / SA images** | OpenCode skills and linters when a job needs them |
| **Served repos** | Stay clean in default pull mode |

PR review uses that plane today (classify → agent waves → publish). The same plane is how future repo operations should land.

## 🧭 What ships today

| Capability | How |
|------------|-----|
| **Poll** | Discover PRs/MRs that need work (`majordomo poll`) |
| **Prep / orchestrate** | Stage diffs, run agent waves, checkpoints (`prep`, `orchestrate`, `dispatch`) |
| **Publish / status** | Comment, description, or check via `gh` / `glab` / Bitbucket HTTP |
| **Cache** | Review-cache + poll-cursor on the served repo |
| **Report** | JUnit, HTML, all-diffs |
| **Tooling** | `build-sa-tools`, `submodule` |

| Layer | Status |
|-------|--------|
| **Go CLI** (`cmd/majordomo`, `internal/`) | Active |
| **Agents / skills** (`agents/`) | Active — rubrics for review (and future agent jobs) |
| **Agent dispatch** (`pipelines/scripts/agent-dispatch.sh`) | Active — OpenCode (set `MAJORDOMO_BIN` when needed) |
| **Docker images** (`dockerfiles/`) | Active — agent, SA tools, forge CLI (`gh` / `glab`) |
| **GitHub Actions** (`.github/workflows/`) | Image CI; tower poll/review in the control-tower repo |

## 🐳 Images (GitHub Actions)

Public builds (no proxy, no corporate package registry) run on push/PR when Dockerfiles change:

- [`.github/workflows/sa-tools.yml`](.github/workflows/sa-tools.yml) — eslint, hadolint, ruff, shellcheck, bandit, mypy
- [`.github/workflows/majordomo-agent.yml`](.github/workflows/majordomo-agent.yml) — OpenCode agent runtime image
- [`.github/workflows/majordomo-forge-cli.yml`](.github/workflows/majordomo-forge-cli.yml) — `majordomo-gh` / `majordomo-glab` images

Corp builds use the same Dockerfiles with `--target corp` and `PACKAGE_REGISTRY_*` build-args (see Dockerfile headers).

Local smoke:

```bash
DOCKER_BUILD_TARGET=public SKIP_PUSH=true \
  bash pipelines/scripts/build-copilot-image.sh local sa-ruff local-test \
  dockerfiles/sa-tools/ruff.Dockerfile

majordomo build-sa-tools          # public (default)
majordomo build-sa-tools --corp   # needs PACKAGE_REGISTRY_* + credentials
```

## 🏷️ One workflow: PR review

Review is the first end-to-end job on the plane. Changed files route to skills automatically for most projects.

**File-review skills** (routed by file type, produce per-file reports):

| Skill | Default routing | Blast radius |
|---|---|---|
| `pr-review-code` | Source code (Python, JS, Java, Go, and 20+ more extensions) | Yes (mandatory) |
| `pr-review-docs` | `**/*.md`, `**/*.rst` | No |
| `pr-review-conf` | `**/*.yml`, `**/*.yaml`, `**/*.toml`, `**/*.json`, `**/*.ini`, and more | No |
| `pr-review-tests` | Not routed by default — add explicit routing to include test files | No |

**Synthesis skills** (run after file-review):

| Skill | What it produces |
|---|---|
| `pr-review-summary` | `summary.md` — high-level PR summary |
| `pr-review-technical` | `tech-review.md` — deep-dive |
| `pr-review-blast-radius` | `blast-radius.md` — impact map |

`majordomo prep` classifies each changed file using glob patterns. Exclusion filters live in `internal/staging`.

See [09 — Customising the review](docs/advanced/09-customising-the-review.md) for agent context, routing overrides, and skill paths (control-tower YAML).

## 📦 Install (binary)

Use the released CLI to bootstrap a control tower or manage a `.majordomo/` pin before you have the submodule checked out.

1. Download the `majordomo` archive for your OS from the [latest release](https://github.com/behaviorengineering/majordomo/releases/latest), unpack it, and put `majordomo` on your `PATH`.
2. Confirm the build:

```bash
majordomo version
```

3. In a tower or legacy app repo, manage the submodule pin:

```bash
majordomo submodule
```

Releases are cut from `v*` tags via GoReleaser (`.goreleaser.yaml`, `.github/workflows/release.yml`). Each release includes archives for linux/darwin/windows (`amd64`/`arm64`) and notes since the previous tag.

**From source** (developers working in this repo):

```bash
go build -o majordomo ./cmd/majordomo
./majordomo version
```

## 📚 Docs

**Architecture:**
- [PLAN — Control Tower, GitHub Actions, and Go](docs/PLAN-control-tower-github-go.md)

**Go CLI (common commands):**

```bash
majordomo poll --config-dir majordomo-central-config --out pending-reviews.json
majordomo prep <base-branch> <staging-dir> \
  [--routing path] [--agent-context path] [--summary-config path]
majordomo orchestrate \
  --pr <n> --staging-dir <dir> --output-dir <dir> \
  [--base-branch <b> | --skip-prep] [--concurrency 6]
majordomo dispatch <pr> <staging-dir> <output-dir> [--summary|--finalize|--prose|...]
majordomo publish --scm github|gitlab|bitbucket <pr> <summary.md> auto|comment|description
majordomo status --scm github|gitlab|bitbucket <commit-sha> INPROGRESS|SUCCESSFUL|FAILED
majordomo cache validate-branch majordomo-pr-reviewer-cache/<id>
majordomo cache push --remote <url> --branch <name> --worktree <dir>
majordomo cache precheck|lookup|store|restore ...
majordomo report junit <review-output-dir> <junit-output-dir>
majordomo report html <input.md> <output.html>
majordomo report all-diffs <manifest.json> <output.txt> [--cap N]
majordomo build-sa-tools [--dry-run] [--corp]
majordomo submodule
```

Set `MAJORDOMO_SCRIPTS` if `pipelines/scripts` is not discoverable from cwd; set `MAJORDOMO_BIN` when dispatch must find the CLI off PATH.

**Start here:**
- [01 — Portable Pipeline Pattern](docs/01-portable-pipeline-pattern.md)
- [02 — Setup](docs/02-setup.md) — install, local dev, and image CI
- [03 — Manage Submodule](docs/03-manage-submodule.md)

**Review workflow (deep dive):**
- [04 — How the Review Works](docs/04-how-the-review-works.md)
- [05 — File Orchestration](docs/advanced/05-file-orchestration.md)
- [06 — PR Summary Flow](docs/advanced/06-pr-summary-flow.md)
- [07 — Example Summary](docs/advanced/07-example-summary.md)
