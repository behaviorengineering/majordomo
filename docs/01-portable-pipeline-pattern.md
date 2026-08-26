# Portable Control-Plane Pattern

*Majordomo — repository operations for evolving software.*

## 🧭 What You'll Learn

**Getting Started:**
- [What is the pattern](#what-is-the-pattern) - Shared engine plus control tower; served repos stay clean
- [Why use it](#why-use-it) - Reuse, isolation, and safer rollout

**Implementation:**
- [How it works](#how-it-works) - Poll → clone → run jobs → publish
- [Versioning the engine](#versioning-the-engine) - Pin `.majordomo/` in the control tower
- [Project-specific customization](#project-specific-customization) - Per-repo YAML without forking engine code

**Advanced Usage:**
- [Best practices](#best-practices) - Governance and rollout habits
- [Related docs](#related-docs) - Setup, submodule ops, review deep dives

---

## 🔍 What is the pattern

Majordomo is a **control plane for evolving repositories**. This guide explains how it separates **reusable engine code** (this repository, vendored as `.majordomo/`) from **org configuration** (control-tower YAML) and **application source** (served repos stay clean).

Jobs (poll, prep, orchestrate, publish, cache, and more) share that plane. **PR review is one workflow** built on it, not the definition of the product.

**Default (pull mode):** A control-tower GitHub Actions workflow polls SCM APIs for work (today: open PRs that need review). Served app repos add nothing — no workflows, no config files, no submodule in the app repo.

**Legacy submodule mode:** Some consumers still vendor `.majordomo/` inside an app repo for local tooling. That path is optional and shrinking — the tower model is canonical.

**Placeholder mapping:**
- `<pipeline-submodule-dir>`: Path to pinned majordomo code in the control tower (typically `.majordomo/`)
- `<repo-config>`: `majordomo-central-config/<repo-slug>.yaml`

---

## 💡 Why use it

- **Reuse:** One control plane, many repos and jobs.
- **Isolation:** Bump the tower's `.majordomo` pin without touching every app repo.
- **Safer rollout:** Test engine changes on the tower branch before org-wide pin updates.
- **No pollution:** Default onboarding does not merge CI files into application default branches.
- **Room to grow:** New jobs plug into the same poll / config / cache / publish edges.

---

## ⚙️ How it works

```text
SCM (GitHub / GitLab / Bitbucket / self-hosted)
        |
        v
Control-tower repo (GitHub Actions)
  ├── .majordomo/  @ pinned commit  (this repo)
  ├── majordomo-central-config/
  │     ├── _defaults.yaml
  │     └── <repo-slug>.yaml
  └── .github/workflows/
        majordomo-poll.yml
        majordomo-review.yml   # example job: PR review

        |
        v
Clone target repo @ change head
Run majordomo jobs (e.g. prep → SA → orchestrate → publish)
```

The tower reads `<repo-config>`, checks out the target revision, runs `majordomo` jobs, and posts results via SCM adapters (`gh` / `glab` / Bitbucket HTTP). Agent-backed steps use `agent-dispatch.sh` (OpenCode).

---

## 🎯 Versioning the engine

The control tower pins `.majordomo/` to an explicit commit. Bump the submodule pointer in the tower repo to roll out engine changes — served app repos do not need merges for default pull mode.

For legacy app-repo submodules, see [03 — Manage Submodule](03-manage-submodule.md).

---

## 🛠️ Project-specific customization

Customize per repo in `majordomo-central-config/<repo-slug>.yaml`. Deep-merge with `_defaults.yaml`. Keep credential **names** in YAML; store secret **values** in GitHub Actions secrets.

```yaml
registry:
  pullDomain: registry-pull.example.com
  pushDomain: registry-push.example.com

pipelines:
  pr-review:
    routing:
      pr-review-code: ["**"]
    model: claude-sonnet-4.5

staticAnalysis:
  - dockerfile: dockerfiles/sa-tools/ruff.Dockerfile
    command: check --output-format=concise
    glob: "**/*.py"
```

Shared engine code stays reusable because each project supplies config data, not orchestration code.

---

## 📌 Best practices

**Pin explicitly:** Tower submodule pin is the release lever — tag or SHA, not floating branches in production.

**Config is org-owned:** Per-repo YAML lives in the control tower, not in application repos (default mode).

**Engine changes go upstream:** Durable changes land in this repository (`behaviorengineering/majordomo`), then the tower bumps its pin.

**Test on branches:** Run tower workflows against fixture repos before bumping the org-wide pin.

---

## 🔗 Related docs

- [02 — Setup](02-setup.md) — CLI, images, local builds
- [03 — Manage Submodule](03-manage-submodule.md) — legacy app-repo vendoring (`majordomo submodule`)
- [04 — How the Review Works](04-how-the-review-works.md) — PR review workflow deep dive
- [04.1 — Pipeline stages](advanced/04.1-pipeline-stages-reference.md) — review stage map
- [PLAN — Control Tower, GitHub Actions, and Go](PLAN-control-tower-github-go.md)
