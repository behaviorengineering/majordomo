# Setup Guide: Integrating Majordomo into Your App Repo

*Majordomo — repository operations for evolving software.*

## 🧭 What You'll Learn

**Getting Started:**
- [Local Implementation Model](#-local-implementation-model) - How this repo implements the generic pattern
- [Prerequisites](#-prerequisites) - Tools and credentials needed before you begin
- [Adding to Your Project](#-adding-to-your-project) - Create the branch, add the submodule, and copy the config

**Implementation:**
- [How The Repo Wiring Works](#-how-the-repo-wiring-works) - Which repo Jenkins clones, where the Jenkinsfile comes from, and which branch stages run on
- [Standard Pipeline Setup](#-standard-pipeline-setup) - Use `setup-majordomo.py` for per-repo pipeline jobs
- [Central Pipeline Setup](#-central-pipeline-setup) - Use `setup-majordomo-central.py` for shared central job setup
- [Webhook Setup](#-webhook-setup) - Wire up Generic Webhook Trigger and Bitbucket events

**Advanced Usage:**
- [Authentication and Username Resolution](#-authentication-and-username-resolution) - How `ARTIFACTORY_USER` is used, including `@` splitting fallback
- [Update Existing Jobs](#-update-existing-jobs) - How update mode selects and patches existing jobs
- [Manual Job Creation](#-manual-job-creation) - Fallback manual Jenkins configuration path

---

## 🔍 Local Implementation Model

This guide is for engineers adding this shared Jenkins pipeline submodule to an application repository. It covers repository wiring, Jenkins job setup, and Bitbucket webhook configuration.

This document is the local implementation guide for the generic pattern in [Portable Submodule Pipeline Pattern](01-portable-pipeline-pattern.md).

In this repository, the generic placeholders map to concrete values:

- `<pipeline-submodule-dir>` maps to `.majordomo/`
- `<pipeline-config-file>` maps to `.majordomo-config.groovy`
- `<script-path>` maps to `pipelines/MajordomoReview.CI.Jenkinsfile`
- `<pipeline-branch>` maps to `pipelines` (convention — any name works, but `pipelines` is the recommended default)
- `<app-change-branch>` maps to the PR source branch

```text
consuming-project-repo

    <pipeline-branch>                                 <app-change-branch>
    +--------------------------------------------+    +-----------------------------+
    | .majordomo/  (submodule pin)         |    | src/, tests/, app changes   |
    | .majordomo-config.groovy             |    |                             |
    +--------------------------------------------+    +-----------------------------+
```

---

## ⚠️ Prerequisites

- **Artifactory Docker repository** to push and cache the `copilot-cli` image
- **Jenkins project** configured as a Pipeline from SCM (Source Control Management)
- **Bitbucket repository** with a webhook configured to trigger the Jenkins job on PR events
- **Fine-grained personal access token** with the Copilot Requests permission, injected as `GITHUB_TOKEN`

---

## 🔍 How The Repo Wiring Works

The important rule is: **Jenkins clones your app repo, not this shared repo directly.**

This setup uses three moving parts:

1. **This repo**: the shared pipeline codebase. It is added into your app repo as the `.majordomo` submodule.
2. **Your app repo pipeline branch**: the branch that contains the `.majordomo` submodule pointer and the root `.majordomo-config.groovy` file.
3. **Your app repo PR source branch**: the branch that actually triggered the webhook and whose diff gets reviewed.

- The Jenkins job SCM points at your app repo.
- The Jenkinsfile is loaded from `.majordomo/pipelines/MajordomoReview.CI.Jenkinsfile` inside that checkout.
- The config file is read from your app repo root, not from inside the submodule.
- For PR webhooks, diff-sensitive stages switch to the webhook source branch before running analysis and review.

ASCII view:

```text
Bitbucket PR webhook
    |
    v
Jenkins job
  SCM = your app repo
  Script Path = .majordomo/pipelines/MajordomoReview.CI.Jenkinsfile
    |
    v
Initial checkout of app repo pipeline branch
  |- .majordomo/                       <- shared pipeline code submodule
  |- .majordomo-config.groovy         <- app-repo runtime config
    |
    v
Pipeline starts
  |- Safe Checkout / Validate Config
  |- Ensure Images
  |- For PR builds: checkout CHANGE_BRANCH
    |
    v
Static Analysis + PR Review run against the webhook PR source branch
```

---

## 📦 Adding to Your Project

The pipeline lives on a dedicated branch in your app repo. Master stays untouched.

Create the branch first:

```bash
cd <your-app-repo>
git checkout -b pipelines
```

> The branch name `pipelines` is a convention, not a requirement. Use any name that fits your team's conventions.

Add this repo as a Git submodule at `.majordomo/` on that branch:

```bash
cd <your-app-repo>
git submodule add ssh://git@bitbucket.srv.westpac.com.au/a01a0f/majordomo.git .majordomo
git add .gitmodules .majordomo
git commit -m "Add .majordomo pipeline as submodule"
git push origin pipelines
```

Copy the config template from the submodule to your repo root and fill in your values:

```bash
cp .majordomo/example.majordomo-config.groovy .majordomo-config.groovy
```

The complete config reference is in `example.majordomo-config.groovy`. At minimum, set your registry and credential values:

```groovy
return [
    registry: [
        pullDomain:    '<your-registry-pull-domain>',
        pushDomain:    '<your-registry-push-domain>',
        credentialsId: '<your-docker-credentials-id>',
    ],
    credentials: [
        githubCopilotCredentialsId: '<your-copilot-token-credential-id>',
    ],
]
```

Omit any key to keep the pipeline default. To control file routing or override review skills, see [Customising the Review](../README.md#customising-the-review).

Run `python .majordomo/scripts/submodule.py` to update, switch branches, or pin the submodule. See [Manage Submodule](03-manage-submodule.md) for first-time setup and manual git commands.

---

## 🔗 Standard Pipeline Setup

Use `setup-majordomo.py` when each app repository owns its own Jenkins pipeline job.

**Create a new standard job:**

```bash
python .majordomo/scripts/setup-majordomo.py \
    --api-token <your-jenkins-api-token> \
    --job-name <job-name> \
    --folder <jenkins-folder> \
    --repo-url ssh://git@bitbucket.srv.westpac.com.au/.../<your-app-repo>.git \
    --create-job
```

**Update an existing standard job:**

```bash
python .majordomo/scripts/setup-majordomo.py \
    --api-token <your-jenkins-api-token> \
    --update-job
```

**Key flags:**

| Flag | Default | Notes |
|------|---------|-------|
| `--jenkins-url` | `https://jenkins.srv.westpac.com.au` | Target a different Jenkins instance |
| `--username` | local part of `ARTIFACTORY_USER` | e.g. `L212278` from `L212278@westpacgroup.com` |
| `--api-token` | `JENKINS_API_TOKEN` env var | Jenkins API token for authentication |
| `--scm-credentials-id` | `credentials.bitbucketSshCredentialsId` from config | Override the SSH credential used for Git SCM |
| `--create-job` | — | Create the job; fails if it already exists |
| `--update-job` | — | Update the job; skips if config is unchanged |
| `--dump-xml` | — | Write the generated job XML to a file instead of posting it to Jenkins |

This script validates `.majordomo-config.groovy`, verifies credential IDs in Jenkins, and configures `.majordomo/pipelines/MajordomoReview.CI.Jenkinsfile` as the job script path. When a credential check fails, the script prints available credentials of the expected type so you can identify the correct ID without logging into Jenkins manually.

<details>
<summary><strong>📋 Example output</strong> (click to expand)</summary>

```
Parsing .majordomo-config.groovy ...

Validating config fields ...
  ✅ All fields OK

Connecting to Jenkins: https://jenkins.srv.westpac.com.au ...

Checking registry reachability ...
  ✅ registry.pullDomain: <artifactory-pull-domain>
  ✅ registry.pushDomain: <artifactory-push-domain>

Verifying credentials in Jenkins ...
  ✅ registry.credentialsId: <jenkins-docker-username-password-credential-id>
  ✅ credentials.githubCopilotCredentialsId: <jenkins-github-copilot-secret-text-credential-id>
  ✅ credentials.artifactoryCredentialsId: <jenkins-artifactory-credential-id>
  ✅ credentials.bitbucketTokenCredentialsId: <jenkins-bitbucket-token-credential-id>
  ✅ credentials.bitbucketSshCredentialsId: <jenkins-bitbucket-ssh-credential-id>
  ✅ credentials.gwtTokenCredentialsId: <jenkins-gwt-token-credential-id>

Checking job: <folder>/<job-name> ...
  ✅ Job '<folder>/<job-name>' created.
```

</details>

---

## 🔗 Central Pipeline Setup

Use `setup-majordomo-central.py` when one central Jenkins job serves many onboarded repositories via `majordomo-central-config/`.

**Create a new central job:**

```bash
python .majordomo/scripts/setup-majordomo-central.py \
    --api-token <your-jenkins-api-token> \
    --folder <jenkins-folder> \
    --repo-url ssh://git@bitbucket.srv.westpac.com.au/a01a0f/majordomo.git \
    --create-job
```

**Update an existing central job:**

```bash
python .majordomo/scripts/setup-majordomo-central.py \
    --api-token <your-jenkins-api-token> \
    --folder <jenkins-folder> \
    --repo-url ssh://git@bitbucket.srv.westpac.com.au/a01a0f/majordomo.git \
    --update-job
```

**Validate central defaults and one repo config only:**

```bash
python .majordomo/scripts/setup-majordomo-central.py \
    --validate-repo <repo-slug> \
    --validate-only
```

The central job uses `.majordomo/pipelines/MajordomoReview.Central.CI.Jenkinsfile` and reads central defaults from `majordomo-central-config/_defaults.groovy`.

---

## 🏷️ Authentication and Username Resolution

Both setup scripts authenticate to Jenkins with `--username` and `--api-token`.

If `--username` is omitted, both scripts use `ARTIFACTORY_USER` and take only the local part before `@`. For example, `L212278@westpacgroup.com` resolves to `L212278`.

If `--api-token` is omitted, both scripts use `JENKINS_API_TOKEN`.

---

## ⚙️ Update Existing Jobs

Use `--update-job` to patch an existing Jenkins job without recreating it.

For `setup-majordomo.py`, omitting `--job-name` in update mode triggers an interactive selector for matching jobs. The script compares generated XML with the current job config and skips updates when there is no diff.

For `setup-majordomo-central.py`, update mode rewrites the configured central job definition using the current defaults and arguments.

Use `--dump-xml <path>` in either script to inspect generated Jenkins XML without posting changes.

---

## 📦 Manual Job Creation

Use manual creation only when setup automation is unavailable.

<details>
<summary><strong>📋 Manual job creation</strong> (click to expand)</summary>

Create a new Pipeline job in Jenkins and set the following:

| Field | Standard Pipeline | Central Pipeline |
|---|---|---|
| **Definition** | Pipeline script from SCM | Pipeline script from SCM |
| **SCM** | Git | Git |
| **Repository URL** | `ssh://git@bitbucket.srv.westpac.com.au/.../<your-app-repo>.git` | `ssh://git@bitbucket.srv.westpac.com.au/a01a0f/majordomo.git` |
| **Credentials** | Your Bitbucket SSH credential | Your Bitbucket SSH credential |
| **Script Path** | `.majordomo/pipelines/MajordomoReview.CI.Jenkinsfile` | `.majordomo/pipelines/MajordomoReview.Central.CI.Jenkinsfile` |

Under **Additional Behaviours** → **Add** → **Advanced sub-modules behaviours**:
- ☐ Recursively update submodules
- ✅ Use credentials from default remote of parent repository

Under **Lightweight checkout**:
- ☐ Uncheck **Lightweight checkout**

</details>

---

## ⚙️ Webhook Setup

Add the Generic Webhook Trigger plugin (a Jenkins plugin that maps webhook JSON payload fields to build parameters) and configure the variable extractions listed in the `parameters` block of `pipelines/MajordomoReview.CI.Jenkinsfile`. The JSONPath expressions (query paths used to extract values from JSON payloads) and event sources are documented there as comments alongside each `string` parameter definition.

Set all extractions to resolve to an empty string when the path is absent. Set the **Cause** field to `Triggered by Bitbucket: ${PR_EVENT_TYPE} in ${CHANGE_BRANCH} by ${ACTOR_NAME}`.

The Generic Webhook Trigger authenticates via a **token in the URL**. We store this token as a Jenkins secret text credential and reference it by credential ID in the **Token Credential** field, **not** the plain **Token** field.

Create a Jenkins **Secret text** credential (a stored secret value Jenkins can inject at runtime) with a UUID value (e.g. `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`) and a recognisable ID (e.g. `post-creator-hook-pr`). Set **Token Credential** in the Generic Webhook Trigger to that credential ID.

Set `gwtTokenCredentialsId` in `.majordomo-config.groovy` to this same credential ID. The **Pipeline Snapshot Guard** uses it to re-fire the webhook when it detects that the `.majordomo` submodule is **stale** — it triggers a fresh build (with updated pipeline code) and aborts the current one. Without this credential configured, the guard falls back to Jenkins `build()`, which creates a build with an upstream parent link in the UI instead of a clean webhook-triggered build.

> **Misconfiguration is silent.** The guard only checks for an HTTP 2xx response from the webhook endpoint. If `gwtTokenCredentialsId` holds the wrong token (one that routes to a different Jenkins job), the POST returns 200, the guard treats it as success, and the current build aborts — but no correct fresh build runs. Verify the credential ID matches the token configured in **this** job's Generic Webhook Trigger.

In Bitbucket, create a webhook pointing at:

```
https://<your-jenkins-host>/generic-webhook-trigger/invoke?token=<token-value>
```

Enable the `pr:opened`, `pr:from_ref_updated`, and `repo:refs_changed` events.
