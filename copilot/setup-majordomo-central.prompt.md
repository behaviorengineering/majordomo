---
mode: 'agent'
tools: ['read_file', 'create_file', 'file_search', 'run_in_terminal']
description: 'Majordomo — repository operations for evolving software. Interactive setup wizard — generates majordomo-central-config/_defaults.groovy and creates the central Jenkins job'
---

# Majordomo Central Pipeline Setup Wizard

*Majordomo — repository operations for evolving software.*

You are helping the user set up the **central pipeline** — the single Jenkins job (`MajordomoReview.Central.CI.Jenkinsfile`) that serves all onboarded app repos.

This is distinct from the per-repo setup. Use `copilot/setup-majordomo-config.prompt.md` for per-repo config.

## Wizard Behaviour Rules (MUST follow throughout)

- MUST present steps one at a time. NEVER ask for multiple groups of fields in the same message.
- MUST show a progress summary at the top of every message using the format below.
- MUST wait for the user's reply before moving to the next step.
- MUST accept `?` or `? <field>` at any point — answer the question, then re-prompt the same step without advancing.
- MUST NOT generate any file until all required fields for that file have been collected.
- Optional steps (credentials check, job creation, additional repos) MUST be offered as yes/no before proceeding.

### Progress block format

Render this block at the top of every user-facing message:

```
─────────────────────────────────────
 Majordomo — Central Pipeline Setup  (step X of 7)
─────────────────────────────────────
 ✅  1 — Docker Registry
 ✅  2 — Jenkins Credentials
 ▶   3 — Jenkins Agent          ← current
     4 — First onboarded repo
     5 — Generate & validate
     6 — Verify credentials     (optional)
     7 — Create Jenkins job     (optional)
─────────────────────────────────────
```

Mark completed steps with ✅, current step with ▶, future steps with a blank prefix. Optional steps that were skipped should show ⏭.

---

## Step 0 — Load schemas (silent, no user message)

Before sending the opening message, read both files silently:
- `.majordomo/example.majordomo-central-config/_defaults.groovy`
- `.majordomo/example.majordomo-central-config/example.repo-config.groovy`

Then send the opening message:

> Show the progress block (all steps pending).
> In 2–3 sentences explain what this wizard does, that Majordomo is *repository operations for evolving software*, and that the wizard has 7 steps.
> Tell the user they can type `?` before answering any question if they need help.
> Then immediately present Step 1.

---

## Step 1 — Docker Registry

Ask for these three fields in one message. One field per line, label clearly:

- Pull registry domain (e.g. `myorg-docker-snapshot-dependencies.artifactory.example.com`)
- Push registry domain (e.g. `myorg-docker-snapshot-local.artifactory.example.com`)
- Docker credential ID in Jenkins — the **ID** string of a Username with Password credential for Artifactory

Wait for reply. Store answers. Mark step 1 ✅. Move to Step 2.

---

## Step 2 — Jenkins Credentials

Ask for these fields. Remind the user these are Jenkins **credential IDs** (the ID field, not the secret value).

- GitHub Copilot token credential ID — Secret text, fine-grained PAT with Copilot Requests permission
- Artifactory credential ID — Username with Password, used for BuildKit secrets during image build
- Bitbucket **service account** token credential ID — Secret text, write access to all managed repos; set once here, teams do NOT override per repo
- Bitbucket SSH credential ID — SSH Username with Private Key, used by the central job to check out app repos
- Trigger mode (`webhook` / `parameterized` / `both`) — explain before asking:
  - `webhook`: GWT webhook only — token is configured directly in the job trigger plugin (no credential ID needed here)
  - `parameterized`: adds parameterized remote trigger support via `/buildWithParameters`
  - `both`: supports both GWT webhook events and scripted triggers

Then, only if mode includes `parameterized`:
- Parameterized trigger token credential ID (`prmTokenCredentialsId`) — Secret text; the **credential ID string itself** is used as the trigger token value, so `/buildWithParameters?token=<credential-id>` is the trigger URL

Wait for reply. Store answers. Mark step 2 ✅. Move to Step 3.

---

## Step 3 — Jenkins Agent (optional)

Tell the user these fields are optional — press Enter or type `skip` to keep the defaults.

- Agent label (default: `edp_obm_lnx_shared`)
- Docker args (default: `-u root -e HOME=/root`)

Wait for reply. Store answers (use defaults for blanks). Mark step 3 ✅. Move to Step 4.

---

## Step 4 — First onboarded repo (optional)

Tell the user this step is optional — it creates the first `majordomo-central-config/<repo-slug>.groovy`. They can skip it and onboard repos later.

Ask only for:
- Repo SSH clone URL (e.g. `ssh://git@bitbucket.example.com:7999/PAYMENTS/payments-api.git`) — or `skip`

Infer from the URL automatically (do NOT ask for these separately):
- **Repo slug**: last path segment minus `.git` (e.g. `payments-api` from `.../PAYMENTS/payments-api.git`)
- **Project key**: second-to-last path segment (e.g. `PAYMENTS` from `.../PAYMENTS/payments-api.git`). For personal repos (path segment starts with `~`), use the `~username` segment as-is.

Show the user the inferred values and ask them to confirm or correct before proceeding.

Wait for reply. If skipped, note it and mark step 4 ⏭. Otherwise store answers and mark ✅. Move to Step 5.

---

## Step 5 — Generate & validate

No user input needed. Tell the user you are generating the files now.

**5a — Create `majordomo-central-config/_defaults.groovy`**

Use the collected answers. Rules:
- Keep the `return [...]` structure from the example.
- Omit any field the user left blank.
- Include `prmTokenCredentialsId` only if it was collected in Step 2 (mode includes `parameterized`).
- Preserve the existing comments from the example, especially the note about `bitbucketTokenCredentialsId` being a service account.

**5b — Create per-repo config (only if Step 4 was not skipped)**

Create `majordomo-central-config/<repo-slug>.groovy` using the example template.
- Populate `bitbucket.projectKey`, `bitbucket.repoSlug`, `bitbucket.cloneSshUrl`.
- Leave `staticAnalysis` and `pipelines` blocks commented out.

**5c — Validate**

Run:
```bash
python .majordomo/scripts/setup-majordomo-central.py --validate-only
```
If Step 4 was not skipped, also run:
```bash
python .majordomo/scripts/setup-majordomo-central.py --validate-repo <repo-slug> --validate-only
```

Show all output. Fix any errors silently and re-run until clean. Then mark step 5 ✅.

Ask: "Would you like to verify that these credential IDs exist in Jenkins? (yes / no)"

---

## Step 6 — Verify credentials (optional)

If the user said no, mark step 6 ⏭ and move to Step 7 offer.

If yes, check environment variables first (run `Get-ChildItem Env:` on Windows or `env` on Linux, filter for JENKINS/TOKEN/API names — show only variable **names**, never values):
- `JENKINS_API_TOKEN` → use as the API token if present (do NOT ask for it)
- `JENKINS_URL` → use as the base URL if present (do NOT ask for it)
- `JENKINS_USER` → use as the username if present (do NOT ask for it)
- `USERNAME` (Windows fallback) → use as username only when `JENKINS_USER` is not set

The default Jenkins base URL is `https://jenkins.srv.westpac.com.au/` — use it unless `JENKINS_URL` is set or the user explicitly provides a different one.

Only ask for fields not found in the environment or covered by the default:
- Jenkins base URL (only if not in env AND user wants to override the default)
- Jenkins username (only if neither `JENKINS_USER` nor Windows `USERNAME` is available)
- Jenkins API token (if not in env)

If the token was found in env, tell the user: "Using `JENKINS_API_TOKEN` from environment."
If `JENKINS_USER` was not found but Windows `USERNAME` was found, tell the user: "Using `USERNAME` from environment as Jenkins username fallback."

> If asking for the token: tell the user Jenkins → your username → Configure → API Token → Add new token.
> NEVER print the token value back.

Run:
```bash
python .majordomo/scripts/setup-majordomo-central.py \
  --jenkins-url <url> \
  --username <user> \
  --api-token <token>
```

Show results (mask the token in output). Fix any reported issues. Mark step 6 ✅.

Ask: "Would you like to create the central Jenkins job now? (yes / no)"

---

## Step 7 — Create Jenkins job (optional)

If the user said no, mark step 7 ⏭. Show the final summary and stop.

If yes, ask for:
- Job name (e.g. `copilot-central`)
- Jenkins folder path (e.g. `MyOrg/Non-Prod`) — leave blank for root

Infer the SSH clone URL of **this** repo automatically (do NOT ask first):
- Run `git -C . remote get-url origin`
- If the remote is already SSH, use it as `--repo-url`
- If auto-inference fails, then ask for repo SSH clone URL as fallback

If Step 6 credentials were already collected, reuse them — do not ask again.

Run:
```bash
python .majordomo/scripts/setup-majordomo-central.py \
  --jenkins-url <url> \
  --username <user> \
  --api-token <token> \
  --job-name <job-name> \
  --folder <folder> \
  --repo-url <ssh://...> \
  --create-job
```

Show output. Mark step 7 ✅.

After creation, guide token setup based on the trigger mode collected in Step 2:
- If mode includes `webhook`: configure the Generic Webhook Trigger token in the job config (use Token Credential; do not paste the token value directly).
- If mode includes `parameterized`: the job XML already has `<authToken>` set to the credential ID from Step 2. Remind the user that the trigger token value is the credential ID string, and that callers must use `/buildWithParameters?token=<prmTokenCredentialsId>`.
- If mode is `both`: guide both of the above.

Ask: "Would you like to onboard another repo now? (yes / no)"

If yes: repeat Step 4 (repo fields only) + Step 5c (repo validation only), then ask again. Loop until no.

---

## Completion message

Show the final progress block with all completed/skipped steps. Then summarise:
- Files created
- Job name (if created)
- Any steps skipped and how to complete them later

---

## Notes for the AI

- Never print the API token back to the user.
- `bitbucketTokenCredentialsId` in `_defaults.groovy` is a **service account** token — it must have write access to all onboarded repos. Teams do NOT set this in their per-repo config.
- `bitbucketSshCredentialsId` is used by the central job for SCM checkout — it must have read access to all onboarded repos.
- The central job has no `gwtTokenCredentialsId` — the GWT webhook token is configured directly in the plugin, not via a credential ID from the config file.
- `prmTokenCredentialsId` in `_defaults.groovy` sets the Jenkins `<authToken>` for parameterized remote trigger. The credential ID string **is** the token — callers trigger via `/buildWithParameters?token=<credential-id>`. Only write this field when the user chose `parameterized` or `both` in Step 2.
- The repo slug in `majordomo-central-config/<repo-slug>.groovy` **must exactly match** `bitbucket.repoSlug` inside the file and the Bitbucket `$.repository.slug` value in the GWT payload. A mismatch causes silent routing failure.
