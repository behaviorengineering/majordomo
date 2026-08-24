# Portable Submodule Pipeline Pattern

*Majordomo — repository operations for evolving software.*

## 🧭 What You'll Learn

**Getting Started:**
- [What is the pattern](#what-is-the-pattern) - Core model: shared pipeline repo plus consuming project repo
- [Why use it](#why-use-it) - Why teams adopt this pattern for reuse, isolation, and safer rollout

**Implementation:**
- [How it works](#how-it-works) - End-to-end runtime flow from checkout to config-driven execution
- [Versioning the pipeline](#versioning-the-pipeline) - How submodule pinning controls upgrades per project
- [Project-specific customization](#project-specific-customization) - How projects adapt behavior without forking pipeline code

**Advanced Usage:**
- [Best practices](#best-practices) - Governance and rollout habits that keep the pattern stable
- [Related docs](#related-docs) - Where to go next for setup, submodule ops, and stage internals

---

## 🔍 What is the pattern

This guide is for engineers integrating a shared Jenkins pipeline into an application repository using a Git submodule. It explains the runtime model, versioning strategy, and project-level customization points.

This pattern separates reusable pipeline code from project-owned configuration. The Jenkinsfile (the pipeline definition file Jenkins executes) lives in a shared pipeline repository and is consumed as a Git submodule (a repository embedded at a fixed commit) in each project. Each consuming project stores its own pipeline config file at the repository root on its pipeline branch, not on master. Master stays free of pipeline code and pipeline config.

**Placeholder mapping:**
- `<pipeline-submodule-dir>`: Relative path to the shared pipeline submodule inside a consuming repository
- `<pipeline-config-file>`: Project-owned pipeline config file in the consuming repository
- `<script-path>`: Jenkinsfile path inside the shared pipeline submodule

---

## ✅ Why use it

**Master branch stays clean.** We don't commit pipeline code to master. Only app code and project config live there. Pipeline updates don't require commits to your app repo.

**Reusable across projects.** The same pipeline logic can be consumed by many repositories.

**One source of truth.** Pipeline improvements reach all projects when they update their submodule reference.

**Experiment safely.** New features go on pipeline branches. Projects opt in by switching their submodule reference or pinning a commit.

---

## 🔄 How it works

Jenkins starts from the consuming project's pipeline branch, loads pipeline logic from a pinned submodule (fixed to an explicit commit or branch ref), then applies project-specific config from the consuming repository.

**Shared pipeline repository** - This is the single source of reusable pipeline code and can evolve on its own branches.

**Consuming project repository** - This repository references the shared pipeline as a submodule at <pipeline-submodule-dir> and keeps project-owned settings in <pipeline-config-file> on its pipeline branch, not on master.

```text
consuming-project-repo

    <pipeline-branch>                                 <app-change-branch>
    +--------------------------------------------+    +-----------------------------+
    | <pipeline-submodule-dir>/ (submodule pin)  |    | src/, tests/, app changes   |
    | <pipeline-config-file>                     |    |                             |
    +--------------------------------------------+    +-----------------------------+
```

```text
[Jenkins startup]
    checkout <pipeline-branch> + <pipeline-submodule-dir>/ at pinned ref
    load Jenkinsfile from <pipeline-submodule-dir>/<script-path>

[pipeline checkout stage]
    1. checkout scm
         workspace = <pipeline-branch> + <pipeline-submodule-dir>/ at pinned ref
    2. checkout <app-change-branch> with submodule updates disabled
         app code (src/, tests/) = <app-change-branch>
         <pipeline-submodule-dir>/ stays pinned from <pipeline-branch>
    3. optional guard
         fail if branch policy says submodule wiring must not exist on app branches

[pipeline execution]
    read <pipeline-config-file> from workspace root
    run shared stages against app code from <app-change-branch>
```

In the sequence above, `scm` means Source Control Management checkout and `pinned ref` means a fixed Git revision.

The runtime sequence above is the canonical flow for this pattern.

**Important:** Pipeline code is owned by the shared pipeline repository. Local edits inside <pipeline-submodule-dir> in a consuming project are temporary and should not be used for durable change management.

---

## 🎯 Versioning the pipeline

Each consuming repository controls which pipeline version runs by pinning the submodule to a branch or commit. That same pipeline branch carries the project pipeline config file. Master stays untouched while teams can stay on stable, move to experimental branches, or lock to a known-good version. The shared pipeline repository versions independently, so new features do not force simultaneous upgrades across projects.

See your submodule management guide for commands.

---

## 🛠️ Project-specific customization

Customize the pipeline by defining project-owned values in <pipeline-config-file>. Keep behavior switches, credential references, naming rules, and tool thresholds in that file instead of modifying shared pipeline code.

```groovy
// <pipeline-config-file>
return [
    registry: [
        pullDomain:    'registry-pull-domain',
        pushDomain:    'registry-push-domain',
        credentialsId: 'registry-credentials-id',
    ],
    imageNaming: [
        runtimeImagePrefix: 'project-runtime',
        buildImageSuffix:   'deps',
    ],
    qualityGates: [
        lintThreshold:  'error',
        testReportPath: 'reports/tests.xml',
    ],
]
```

The shared stages remain reusable because each project provides only data, not stage logic.

---

## 📌 Best practices

**Submodule versioning:** Pin submodules to specific branches per project (master, stable, etc.). Version the pipeline repository independently with semantic versioning.

**Config is project-owned:** Keep example config contracts in a shared reference file and keep project-specific values in each consuming repository. Never commit consumer-specific settings to the shared pipeline repository.

**Pipeline changes go upstream:** Make durable pipeline code changes in the shared pipeline repository, not in a checked-out submodule copy inside a consuming repository. Local submodule edits are temporary and should not be treated as release changes.

**Test on branches:** Test pipeline changes on a pipeline repository branch before merging to master. Every consuming project picks up the changes when they update their submodule reference.

---

## 🔗 Related docs

- Setup guide in your consuming repository docs: onboarding this pattern
- Submodule management guide in your consuming repository docs: pinning, updating, and rollback workflows
- Module structure conventions guide in your consuming repository docs: layout and naming standards
