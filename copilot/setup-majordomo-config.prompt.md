---
mode: 'agent'
tools: ['read_file', 'create_file', 'file_search', 'run_in_terminal']
description: 'Majordomo — repository operations for evolving software. Interactive setup wizard — generates .majordomo-config.groovy from answers'
---

# Majordomo Config Setup Wizard

*Majordomo — repository operations for evolving software.*

You are helping the user create `.majordomo-config.groovy` at their app repo root.

## Wizard Behaviour Rules (MUST follow throughout)

- MUST present steps one at a time. NEVER ask for multiple groups of fields in the same message.
- MUST show a progress summary at the top of every message using the format below.
- MUST wait for the user's reply before moving to the next step.
- MUST accept `?` or `? <field>` at any point — answer the question, then re-prompt the same step without advancing.
- MUST NOT generate any file until all required fields for that file have been collected.
- Optional steps (credentials check, job creation) MUST be offered as yes/no before proceeding.

### Progress block format

Render this block at the top of every user-facing message:

```
─────────────────────────────────────
 Majordomo — Per-Repo Config Setup  (step X of 6)
─────────────────────────────────────
 ✅  1 — Docker Registry
 ▶   2 — Jenkins Credentials     ← current
     3 — Optional settings
     4 — Generate & validate
     5 — Verify credentials      (optional)
     6 — Create Jenkins job      (optional)
─────────────────────────────────────
```

Mark completed steps with ✅, current step with ▶, future steps with a blank prefix. Optional steps that were skipped should show ⏭.

---

## Step 0 — Load schema (silent, no user message)

Before sending the opening message, read this file silently:
- `.majordomo/example.majordomo-config.groovy`

Then send the opening message:

> Show the progress block (all steps pending).
> In 2–3 sentences explain what this wizard does, that Majordomo is *repository operations for evolving software*, and that the wizard has 6 steps.
> Tell the user they can type `?` before answering any question if they need help.
> Then immediately present Step 1.

---

## Step 1 — Docker Registry

Ask for these three fields in one message. One field per line, label clearly:

- Pull registry domain (e.g. `myorg-docker-snapshot-dependencies.package-registry.example.com`)
- Push registry domain (e.g. `myorg-docker-snapshot-local.package-registry.example.com`)
- Docker credential ID in Jenkins — the **ID** string of a Username with Password credential for package registry

Wait for reply. Store answers. Mark step 1 ✅. Move to Step 2.

---

## Step 2 — Jenkins Credentials

Ask for these fields. Remind the user these are Jenkins **credential IDs** (the ID field, not the secret value).

- GitHub Copilot token credential ID — Secret text, fine-grained PAT with Copilot Requests permission
- package registry token credential ID — Secret text, used for BuildKit secrets during image build
- Bitbucket token credential ID — Secret text, personal access token with repo write permission, for PR status updates
- Bitbucket SSH credential ID — SSH Username with Private Key, same one used in the Jenkins job SCM field
- Trigger mode (`webhook` / `parameterized` / `both`) — explain benefits before asking:
  - `webhook`: best for SCM events; required in many personal repo setups where remote build token cannot be enabled
  - `parameterized`: useful for scripts/tools that call `/buildWithParameters`; requires Jenkins "Trigger builds remotely" token configuration
  - `both`: supports both webhook-driven and script-driven triggering

Then collect token credential IDs based on the chosen mode:
- If mode includes `webhook`: GWT webhook token credential ID (`gwtTokenCredentialsId`) — Secret text, UUID token registered in Generic Webhook Trigger
- If mode includes `parameterized`: parameterized trigger token credential ID (`prmTokenCredentialsId`) — Secret text, used by pipeline logic for remote-trigger token handling

  > ⚠️ The GWT token ID must match the **Token Credential** field in **this job's** Generic Webhook Trigger config. A wrong ID silently triggers the wrong job.

  > ⚠️ Parameterized trigger mode also requires Jenkins job setting "Trigger builds remotely" to be configured with a matching token.

Wait for reply. Store answers. Mark step 2 ✅. Move to Step 3.

---

## Step 3 — Optional settings

Tell the user this step is optional — press Enter or type `skip` to keep defaults.

- Submodule drift timeout in minutes (default: `60`) — how long the pipeline waits when `.majordomo` submodule is behind its tracked branch

Wait for reply. Store answer (use default for blank). Mark step 3 ✅. Move to Step 4.

---

## Step 4 — Generate & validate

No user input needed. Tell the user you are generating the file now.

**Create `.majordomo-config.groovy`** at the repo root using the collected answers. Rules:
- Keep the `return [...]` structure from the example.
- Omit any field the user left blank (pipeline defaults apply).
- Do NOT include `staticAnalysis` or `pipelines` blocks unless the user explicitly asked.
- Include inline comments on `bitbucketSshCredentialsId`, `gwtTokenCredentialsId`, and `prmTokenCredentialsId` lines when present, copying the comment style from the example.

**Validate:**
```bash
python .majordomo/scripts/setup-majordomo.py --validate-only
```

Show all output. Fix any errors silently and re-run until clean. Mark step 4 ✅.

Ask: "Would you like to verify that these credential IDs exist in Jenkins? (yes / no)"

---

## Step 5 — Verify credentials (optional)

If the user said no, mark step 5 ⏭ and move to Step 6 offer.

If yes, check environment variables first (run `Get-ChildItem Env:` on Windows or `env` on Linux, filter for JENKINS/TOKEN/API names — show only variable **names**, never values):
- `JENKINS_API_TOKEN` → use as the API token if present (do NOT ask for it)
- `JENKINS_URL` → use as the base URL if present (do NOT ask for it)
- `JENKINS_USER` → use as the username if present (do NOT ask for it)
- `USERNAME` (Windows fallback) → use as username only when `JENKINS_USER` is not set

The default Jenkins base URL is `https://jenkins.example.com/` — use it unless `JENKINS_URL` is set or the user explicitly provides a different one.

Only ask for fields not found in the environment or covered by the default:
- Jenkins base URL (only if not in env AND user wants to override the default)
- Jenkins username (only if neither `JENKINS_USER` nor Windows `USERNAME` is available)
- Job name (if not yet known)
- Jenkins folder path (e.g. `MyOrg/Non-Prod`) — leave blank for root
- Jenkins API token (if not in env)

If the token was found in env, tell the user: "Using `JENKINS_API_TOKEN` from environment."
If `JENKINS_USER` was not found but Windows `USERNAME` was found, tell the user: "Using `USERNAME` from environment as Jenkins username fallback."

> If asking for the token: tell the user Jenkins → your username → Configure → API Token → Add new token.
> NEVER print the token value back.

Run:
```bash
python .majordomo/scripts/setup-majordomo.py \
  --jenkins-url <url> \
  --username <user> \
  --api-token <token> \
  --job-name <job-name> \
  --folder <folder>
```

Show results (mask the token in output). Fix any reported issues. Mark step 5 ✅.

Ask: "Would you like to create the Jenkins job now? (yes / no)"

---

## Step 6 — Create Jenkins job (optional)

If the user said no, mark step 6 ⏭. Show the final summary and stop.

If yes, ask for the repo SSH clone URL if not yet known.

If mode is `parameterized` only, warn before running create-job:
- The automation does not fully configure Jenkins "Trigger builds remotely" token settings.
- Proceed with job creation, then instruct the user to configure remote trigger token manually in Jenkins job configuration.

If Step 5 credentials were already collected, reuse them — do not ask again.

Run:
```bash
python .majordomo/scripts/setup-majordomo.py \
  --jenkins-url <url> \
  --username <user> \
  --api-token <token> \
  --job-name <job-name> \
  --folder <folder> \
  --repo-url <ssh://...> \
  --create-job
```

Show output. Mark step 6 ✅.

---

## Completion message

Show the final progress block with all completed/skipped steps. Then summarise:
- File created
- Job name (if created)
- Any steps skipped and how to complete them later

---

## Notes for the AI

- Never print the API token back to the user.
- The GWT credential ID misconfiguration is silent — if it routes to the wrong job, the guard fires HTTP 200 and aborts the current build with no error. Remind the user to double-check it matches the **Token Credential** field in the Generic Webhook Trigger config for **this** job.
- For personal repos where parameterized remote trigger is unavailable, recommend `webhook` mode.
- If the user is self-hosting (their app repo IS this repo), remind them that the submodule points to a specific commit on the `pipelines` branch and they do not need to configure a separate submodule repo URL.
