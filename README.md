# Majordomo — repository operations for evolving software.

![Elevator pitch](media/elevator-pitch.jpeg)

Majordomo runs structured pull-request reviews: classify changed files, route them to review skills, run static analysis, and publish markdown reports. The long-term runtime is a **Go control plane on GitHub Actions** (control-tower model). Jenkins and Groovy have been removed from this repository.

---

🎉🎊✨ **[Migration plan](docs/PLAN-control-tower-github-go.md)** — control tower, GitHub Actions, Go CLI, SCM-agnostic reviews.  
**[Portable pipeline pattern](docs/01-portable-pipeline-pattern.md)** — why the review shape exists.

---

[![PR summary example](media/pr-summary.png)](docs/07-example-summary.md)

## 💡 Why This Exists

**Code review is a bottleneck.** Reviewers see too much at once, miss security issues under time pressure, and have no systematic way to ask "what else does this change break?"

This project treats a PR review as a structured process. Each file is reviewed in priority order: security first, input handling next, logic correctness and test coverage last. After all files are reviewed, a separate pass maps blast-radius across the whole changeset. Reports convert to JUnit XML and HTML so CI can surface the result.

## 🧭 Current direction

| Layer | Status |
|-------|--------|
| **Agents / skills** (`agents/`) | Active — review rubrics and personas |
| **Review scripts** (`pipelines/scripts/`) | Active — staging, dispatch, summary, publish helpers |
| **Docker images** (`dockerfiles/`) | Active — public/corp dual-stage builds; GHA workflows |
| **Go CLI** (`cmd/majordomo`, `internal/`) | In progress — prep, report, poll stubs |
| **GitHub Actions** (`.github/workflows/`) | Image CI for SA tools + copilot-cli (`--target public`) |
| **Jenkins / Groovy** | Removed — see the [migration plan](docs/PLAN-control-tower-github-go.md) |

## 🐳 Images (GitHub Actions)

Public builds (no proxy, no corporate package registry) run on push/PR when Dockerfiles change:

- [`.github/workflows/sa-tools.yml`](.github/workflows/sa-tools.yml) — eslint, hadolint, ruff, shellcheck, bandit, mypy
- [`.github/workflows/copilot-cli.yml`](.github/workflows/copilot-cli.yml) — agent runtime image

Corp builds use the same Dockerfiles with `--target corp` and `PACKAGE_REGISTRY_*` build-args (see Dockerfile headers).

Local smoke:

```bash
DOCKER_BUILD_TARGET=public SKIP_PUSH=true \
  bash pipelines/scripts/build-copilot-image.sh local sa-ruff local-test \
  dockerfiles/sa-tools/ruff.Dockerfile

python scripts/build-sa-tools.py          # public (default)
python scripts/build-sa-tools.py --corp   # needs PACKAGE_REGISTRY_* + credentials
```

## 🏷️ Customising the Review

The review routes changed files automatically. No routing config is needed for most projects.

**File-review skills** (routed by file type, produce per-file reports):

| Skill | Default routing | Blast radius |
|---|---|---|
| `pr-review-code` | Source code (Python, JS, Java, Go, and 20+ more extensions) | Yes (mandatory) |
| `pr-review-docs` | `**/*.md`, `**/*.rst` | No |
| `pr-review-conf` | `**/*.yml`, `**/*.yaml`, `**/*.toml`, `**/*.json`, `**/*.ini`, and more | No |
| `pr-review-tests` | Not routed by default — add explicit routing to include test files | No |

**Synthesis skills** (run automatically after file-review):

| Skill | What it produces |
|---|---|
| `pr-review-summary` | `summary.md` — high-level PR summary |
| `pr-review-technical` | `tech-review.md` — deep-dive |
| `pr-review-blast-radius` | `blast-radius.md` — impact map |

`pipelines/scripts/git-diff-prep.py` classifies each changed file using glob patterns. Exclusion filters live under `EXCLUDE_PATTERNS` in that script.

See [09 — Customising the review](docs/advanced/09-customising-the-review.md) for agent context, routing overrides, and skill paths (control-tower YAML).

## 📚 Docs

**Roadmap:**
- [PLAN — Control Tower, GitHub Actions, and Go](docs/PLAN-control-tower-github-go.md)

**Go CLI (in progress):**

```bash
go build -o majordomo ./cmd/majordomo
./majordomo version
./majordomo prep <base-branch> <staging-dir> \
  [--routing path] [--agent-context path] [--summary-config path]
./majordomo report junit <review-output-dir> <junit-output-dir>
./majordomo report html <input.md> <output.html>
./majordomo poll   # stub — Phase 1
```

`majordomo prep` ports `git-diff-prep.py` (exit codes `0` / `1` / `2`). Prefer the binary when present; set `MAJORDOMO_PREP=python` or `MAJORDOMO_REPORT=python` to force the Python scripts.

**Start here:**
- [01 — Portable Pipeline Pattern](docs/01-portable-pipeline-pattern.md)
- [02 — Setup](docs/02-setup.md) — local dev and image CI
- [03 — Manage Submodule](docs/03-manage-submodule.md)

**Understanding the review:**
- [04 — How the Review Works](docs/04-how-the-review-works.md)
- [05 — File Orchestration](docs/advanced/05-file-orchestration.md)
- [06 — PR Summary Flow](docs/advanced/06-pr-summary-flow.md)
- [07 — Example Summary](docs/advanced/07-example-summary.md)
