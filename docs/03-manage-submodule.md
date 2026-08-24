# Submodule Management Guide for App Repositories

*Majordomo — repository operations for evolving software.*

This guide is for engineers who maintain the `.majordomo` submodule in application repositories. It covers setup, update workflows, pinning strategy, and recovery steps for broken submodule state.

## 🧭 What You'll Learn

**Getting Started:**
- [First-time setup](#first-time-setup-adding-the-submodule) - Add the submodule to a new app repo
- [How to clone the repo including submodules](#how-to-clone-the-repo-including-submodules) - Clone with submodules included

**Maintenance:**
- [How the script works](#how-the-script-works) - Two-stage flow, keypress navigation, and smart push behaviour
- [Updating the submodule reference to the latest version](#updating-the-submodule-reference-to-the-latest-version) - Pull latest commits on the current branch
- [Switching the submodule to a different branch](#switching-the-submodule-to-a-different-branch) - Move the submodule to a different pipeline branch
- [Pinning the submodule to a specific commit](#pinning-the-submodule-to-a-specific-commit) - Lock the submodule to an exact SHA

**Troubleshooting:**
- [Fixing broken submodules](#fixing-broken-submodules) - Recover from detached HEAD, wrong remote, or missing `.git` dir

---

## 🔐 Jenkins Access to Bitbucket

To use this pipeline from Jenkins, Jenkins must be able to authenticate to Bitbucket and pull the submodule.

Recommended approach:
Complete this access setup before running first-time submodule commands.
- Create an SSH key pair for Jenkins (or your CI identity).
- Add the public key to Bitbucket.
- Add the private key to Jenkins credentials and use it for Git checkout.

If your team already has a Bitbucket service account, you can use that account's SSH key instead.

---

## 📦 First-time setup (adding the submodule)

Only needed once when setting up a new app repo.

```bash
cd <your-app-repo>
git submodule add ssh://git@bitbucket.example.com/example-project/majordomo.git .majordomo
git add .gitmodules .majordomo
git commit -m "Add .majordomo pipeline as submodule"
git push origin <your-branch>
```

Then create your config file from the template:

```bash
cp .majordomo/example.majordomo-config.groovy .majordomo-config.groovy
# Edit .majordomo-config.groovy with your registry and credential values
git add .majordomo-config.groovy
git commit -m "Add Jenkins pipeline config"
git push origin <your-branch>
```

---

## 📦 How to clone the repo including submodules

```bash
# Fresh clone: includes the .majordomo submodule automatically
git clone --recurse-submodules ssh://git@bitbucket.example.com/.../<your-app-repo>.git
```

If you already cloned without `--recurse-submodules`, the `.majordomo` folder exists but is empty. Initialize the submodule:

```bash
git submodule update --init
```

---

## ⚙️ How the script works

Run the script from the root of your app repo:

```bash
python .majordomo/scripts/submodule.py
```

**Stage 1 — context check (off-branch only).** If your parent repo is not on the `pipelines` branch, the script shows a warning and asks how to proceed:

```
⚠️  OFF-BRANCH WARNING  ⚠️
Parent repo is on 'updates', not 'pipelines'.
Any direct commits will land on 'updates'.

1. 🔒 Safe  — update 'pipelines' via isolated worktree
2. ⚡ Direct — I know what I'm doing (operate on 'updates')
q. Quit
```

| Option | What it does |
|--------|-------------|
| **1 — Safe** | Creates a temporary Git worktree for `pipelines`, commits the pointer update there, pushes `pipelines` to origin, and cleans up. Your working branch is never touched. |
| **2 — Direct** | Operates on your current branch. Use when you intentionally want the submodule pointer updated on a non-`pipelines` branch. |

If the parent repo is already on `pipelines`, stage 1 is skipped entirely.

**Stage 2 — operations menu.** A single-keypress menu — no Enter needed:

```
1. Update to latest (pull current branch)
2. Switch to a different branch
3. Pin to current commit
q. Quit
```

**Smart push behaviour.** The script only offers "Push to origin and exit?" after an operation that actually changed something. If the submodule was already up to date, the prompt is skipped.

---

## ⚙️ Updating the submodule reference to the latest version

Run the script and press **1**:

```bash
python .majordomo/scripts/submodule.py
```

The script pulls the latest commits on the current branch. If the parent repo pointer changed, it commits the update and offers to push. If the submodule was already up to date, it exits silently.

<details>
<summary><strong>Manual git commands</strong> (click to expand)</summary>

```bash
cd .majordomo
git pull origin master
cd ..
git add .majordomo
git commit -m "Update .majordomo pipeline submodule"
git push
```

</details>

---

## ⚙️ Switching the submodule to a different branch

Run the script and press **2**:

```bash
python .majordomo/scripts/submodule.py
```

The script fetches remote branches, presents a numbered list, checks out the selected branch, and commits the parent repo pointer change.

<details>
<summary><strong>Manual git commands</strong> (click to expand)</summary>

```bash
cd .majordomo
git fetch
git checkout <branch-name>
cd ..
git add .majordomo
git commit -m "Pin .majordomo submodule to <branch-name> branch"
git push
```

</details>

---

## ⚙️ Pinning the submodule to a specific commit

Run the script, press **2** to switch to the target branch, then press **3** to pin:

```bash
python .majordomo/scripts/submodule.py
```

The parent repo records the exact commit SHA.

<details>
<summary><strong>Manual git commands</strong> (click to expand)</summary>

```bash
cd .majordomo
git fetch
git checkout <commit-sha>
cd ..
git add .majordomo
git commit -m "Pin .majordomo submodule to <commit-sha>"
git push
```

</details>

---

## 🔧 Fixing broken submodules

If submodule state becomes corrupted (detached HEAD (Git state where HEAD points directly to a commit instead of a branch), wrong remote, missing `.git` dir inside the submodule folder), the `submodule.py` script cannot be used; it lives inside `.majordomo` which is the broken submodule itself.

**Common causes:**
- SSH key not set up before running initial `git submodule add`: remote becomes unreachable
- Cloning app repo without `--recurse-submodules`: submodule folder exists but is empty or detached
- Force-pushing to parent repo without updating submodule pointer: parent and submodule SHA get out of sync
- Manually editing `.gitmodules` with incorrect remote URL or path

Run these commands manually from the **root of your app repo**. Replace submodule paths with the submodules present in your target repository.

```bash
# Remove cached index entries for the broken submodules
git rm -r --cached .majordomo

# Re-register them (--force overwrites any stale .gitmodules entries)
git submodule add --force ssh://git@bitbucket.example.com/example-project/majordomo.git .majordomo

git add .gitmodules .majordomo
git commit -m "Fix broken submodule registrations"
git push
```

If your repository also uses other submodules, repeat the same two-step pattern (`git rm -r --cached <path>`, then `git submodule add --force <url> <path>`) for each additional submodule.

> **Note:** `git rm -r --cached` only removes the Git index entry (the staged tracking record); it does **not** delete the folder on disk.

