# Submodule Management Guide for App Repositories

*Majordomo — repository operations for evolving software.*

This guide is for engineers who maintain the `.majordomo` submodule in application repositories. It covers setup, update workflows, pinning strategy, and recovery steps for broken submodule state.

## 🧭 What You'll Learn

**Getting Started:**
- [First-time setup](#first-time-setup-adding-the-submodule) - Add the submodule to a new app repo
- [How to clone the repo including submodules](#how-to-clone-the-repo-including-submodules) - Clone with submodules included

**Maintenance:**
- [How `majordomo submodule` works](#how-majordomo-submodule-works) - Two-stage flow, menu choices, and smart push behaviour
- [Updating the submodule reference to the latest version](#updating-the-submodule-reference-to-the-latest-version) - Pull latest commits on the current branch
- [Switching the submodule to a different branch](#switching-the-submodule-to-a-different-branch) - Move the submodule to a different pipeline branch
- [Pinning the submodule to a specific commit](#pinning-the-submodule-to-a-specific-commit) - Lock the submodule to an exact SHA

**Troubleshooting:**
- [Fixing broken submodules](#fixing-broken-submodules) - Recover from detached HEAD, wrong remote, or missing `.git` dir

---

## 🔐 CI access to the submodule remote

Your CI identity (GitHub Actions deploy key, machine user, or service account) must be able to clone this repository when resolving `.majordomo`.

Recommended approach:
- Create an SSH key or fine-scoped token for CI.
- Grant it read access to the majordomo remote.
- Store it in the control-tower / app-repo secrets used by checkout.

---

## 📦 First-time setup (adding the submodule)

Only needed once when setting up a new app repo (legacy submodule consumers). New onboardings should prefer the [control-tower model](PLAN-control-tower-github-go.md) so app repos stay clean.

Install the `majordomo` binary from [GitHub Releases](https://github.com/behaviorengineering/majordomo/releases/latest) (see [02 — Setup](02-setup.md)), then either use `majordomo submodule` or add the submodule by hand:

```bash
cd <your-app-repo>
git submodule add https://github.com/behaviorengineering/majordomo.git .majordomo
git add .gitmodules .majordomo
git commit -m "Add .majordomo as submodule"
git push origin <your-branch>
```

Org config and workflows live in the control tower — see [PLAN — Control Tower](PLAN-control-tower-github-go.md).

---

## 📦 How to clone the repo including submodules

```bash
# Fresh clone: includes the .majordomo submodule automatically
git clone --recurse-submodules ssh://git@bitbucket.example.com/scm/.../<your-app-repo>.git
```

If you already cloned without `--recurse-submodules`, the `.majordomo` folder exists but is empty. Initialize the submodule:

```bash
git submodule update --init
```

---

## ⚙️ How `majordomo submodule` works

Run from the root of your app repo (with `majordomo` on `PATH`, built from this repo):

```bash
majordomo submodule
```

**Stage 1 — context check (off-branch only).** If your parent repo is not on the `pipelines` branch, the command shows a warning and asks how to proceed:

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

**Stage 2 — operations menu.** Type a choice and press Enter:

```
1. Update to latest (pull current branch)
2. Switch to a different branch
3. Pin to current commit
q. Quit
```

**Smart push behaviour.** The command only offers "Push to origin and exit?" after an operation that actually changed something. If the submodule was already up to date, the prompt is skipped.

---

## ⚙️ Updating the submodule reference to the latest version

Run the command and press **1**:

```bash
majordomo submodule
```

The command pulls the latest commits on the current branch. If the parent repo pointer changed, it commits the update and offers to push. If the submodule was already up to date, it exits silently.

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

Run the command and choose **2**:

```bash
majordomo submodule
```

The command fetches remote branches, presents a numbered list, checks out the selected branch, and commits the parent repo pointer change.

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

Run the command, choose **2** to switch to the target branch, then **3** to pin:

```bash
majordomo submodule
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

If submodule state becomes corrupted (detached HEAD (Git state where HEAD points directly to a commit instead of a branch), wrong remote, missing `.git` dir inside the submodule folder), `majordomo submodule` cannot be used from that broken checkout; fix the submodule folder first.

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
git submodule add --force ssh://git@bitbucket.example.com/scm/tooling/majordomo.git .majordomo

git add .gitmodules .majordomo
git commit -m "Fix broken submodule registrations"
git push
```

If your repository also uses other submodules, repeat the same two-step pattern (`git rm -r --cached <path>`, then `git submodule add --force <url> <path>`) for each additional submodule.

> **Note:** `git rm -r --cached` only removes the Git index entry (the staged tracking record); it does **not** delete the folder on disk.

