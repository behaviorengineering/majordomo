# Portable Submodule Pipeline Pattern

*Majordomo — repository operations for evolving software.*

## 🧭 What You'll Learn

**Getting Started:**
- [What is the pattern](#what-is-the-pattern) - Core model: shared pipeline repo plus control tower
- [Why use it](#why-use-it) - Reuse, isolation, and safer rollout

**Implementation:**
- [How it works](#how-it-works) - End-to-end flow from poll/webhook to config-driven review
- [Versioning the pipeline](#versioning-the-pipeline) - Pin `.majordomo/` in the control tower
- [Project-specific customization](#project-specific-customization) - Per-repo YAML without forking pipeline code

**Advanced Usage:**
- [Best practices](#best-practices) - Governance and rollout habits
- [Related docs](#related-docs) - Setup, submodule ops, stage internals

---

## 🔍 What is the pattern

This guide explains how Majordomo separates **reusable pipeline code** (this repository, vendored as `.majordomo/`) from **org configuration** (control-tower YAML) and **application source** (served repos stay clean).

**Default (pull mode):** A control-tower GitHub Actions workflow polls SCM APIs for open PRs. Served app repos add nothing — no workflows, no config files, no submodule in the app repo.

**Legacy submodule mode:** Some consumers still vendor `.majordomo/` inside an app repo for local scripts or transitional Jenkins setups. That path is optional and shrinking — the tower model is canonical.

**Placeholder mapping:**
- `<pipeline-submodule-dir>`: Path to pinned majordomo code in the control tower (typically `.majordomo/`)
- `<repo-config>`: `majordomo-central-config/<repo-slug>.yaml`

---

## 💡 Why use it

- **Reuse:** One review engine, many repos.
- **Isolation:** Bump the tower's `.majordomo` pin without touching every app repo.
- **Safer rollout:** Test pipeline changes on the tower branch before org-wide pin updates.
- **No pollution:** Default onboarding does not merge CI files into application default branches.

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
        majordomo-review.yml

        |
        v
Clone target repo @ PR head
Run majordomo prep → SA → orchestrate → publish
```

The orchestrator reads `<repo-config>`, checks out the PR branch, runs scripts from `.majordomo/pipelines/scripts/`, and publishes results via SCM adapters.

---

## 🎯 Versioning the pipeline

The control tower pins `.majordomo/` to an explicit commit. Bump the submodule pointer in the tower repo to roll out pipeline changes — served app repos do not need merges for default pull mode.

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

Shared scripts stay reusable because each project supplies data, not orchestration logic.

---

## 📌 Best practices

**Pin explicitly:** Tower submodule pin is the release lever — tag or SHA, not floating branches in production.

**Config is org-owned:** Per-repo YAML lives in the control tower, not in application repos (default mode).

**Pipeline changes go upstream:** Durable changes land in this repository (`behaviorengineering/majordomo`), then the tower bumps its pin.

**Test on branches:** Run tower workflows against fixture repos before bumping the org-wide pin.

---

## 🔗 Related docs

- [02 — Setup](02-setup.md) — what runs today (scripts, images, Go CLI)
- [03 — Manage Submodule](03-manage-submodule.md) — legacy app-repo vendoring
- [04.1 — Pipeline stages](advanced/04.1-pipeline-stages-reference.md) — script map
- [PLAN — Control Tower, GitHub Actions, and Go](PLAN-control-tower-github-go.md)
