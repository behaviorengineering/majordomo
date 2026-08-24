"""Interactive manager for the Git submodule that contains this script.

Detects its own submodule root at runtime, locates the parent repository,
and provides an interactive menu to update and switch branches — committing
the parent's submodule pointer after each operation.

Usage:
    python submodule.py

Exit codes:
    0  Clean exit (user quit or operation succeeded)
    1  Fatal error (git command failed, not inside a submodule)
"""

from __future__ import annotations

import inspect
import subprocess
import sys
from pathlib import Path

if sys.platform == "win32":
    import msvcrt
else:
    import termios
    import tty

_EXIT_ERR = 1
_PIPELINES_BRANCH = "pipelines"
_WORKTREE_DIR = ".pipelines-worktree"
_DIVERGE_WARNING = inspect.cleandoc("""
    Warning: local HEAD ({local}) does not match origin/{branch} ({remote}).
    The branch may have diverged (merge commit instead of fast-forward).
    Reset hard to origin/{branch} to fix this?
""")


# ── Git helpers ────────────────────────────────────────────────────────────


def run_git(args: list[str], *, cwd: Path, check: bool = True) -> str:
    """Run a git command in *cwd* and return stripped stdout.

    Args:
        args: Git arguments (excluding the ``git`` binary itself).
        cwd: Working directory for the command.
        check: Raise ``CalledProcessError`` on non-zero exit when ``True``.

    Returns:
        Stripped stdout string.

    Raises:
        subprocess.CalledProcessError: If *check* is ``True`` and git exits non-zero.
    """
    result = subprocess.run(
        ["git", *args],
        cwd=cwd,
        capture_output=True,
        text=True,
        check=check,
    )
    return result.stdout.strip()


def find_submodule_root() -> Path:
    """Find the root of the Git submodule that contains this script.

    Returns:
        Absolute path to the submodule root directory.

    Raises:
        SystemExit: If the script is not inside a Git repository.
    """
    try:
        root = run_git(["rev-parse", "--show-toplevel"], cwd=Path(__file__).parent)
    except subprocess.CalledProcessError:
        print("Error: could not determine submodule root — not inside a git repo.")
        sys.exit(_EXIT_ERR)
    return Path(root)


def find_parent_repo_root(submodule_root: Path) -> Path | None:
    """Find the parent repository root that owns the submodule.

    Args:
        submodule_root: Absolute path to the submodule root directory.

    Returns:
        Absolute path to the parent repository root, or ``None`` when no
        parent repository exists or the directory is not a registered submodule
        (i.e. not listed as a gitlink in the parent's index).
    """
    try:
        root = run_git(["rev-parse", "--show-toplevel"], cwd=submodule_root.parent)
    except subprocess.CalledProcessError:
        return None
    parent = Path(root)
    if parent == submodule_root:
        return None
    # Confirm the directory is a submodule: either tracked in the current
    # branch's index (gitlink mode 160000) or registered in $GIT_DIR/modules/
    # (persists across branches when the submodule is only tracked on another
    # branch, e.g. parent is on 'updates' but submodule lives on 'pipelines').
    rel = str(submodule_root.relative_to(parent))
    index_entry = run_git(["ls-files", "--stage", rel], cwd=parent, check=False)
    if index_entry.startswith("160000"):
        return parent
    git_dir_raw = run_git(["rev-parse", "--git-dir"], cwd=parent, check=False)
    if git_dir_raw:
        git_dir_path = Path(git_dir_raw)
        git_dir = git_dir_path if git_dir_path.is_absolute() else parent / git_dir_raw
        if (git_dir / "modules" / rel).exists():
            return parent
    return None


def get_submodule_name(submodule_root: Path, parent_root: Path | None) -> str:
    """Return the submodule path relative to the parent repository root.

    Args:
        submodule_root: Absolute path to the submodule directory.
        parent_root: Absolute path to the parent repository root, or ``None``
            when not inside a true submodule.

    Returns:
        Relative path string (e.g. ``'.majordomo'``), or the directory
        name when there is no parent repository.
    """
    if parent_root is None:
        return submodule_root.name
    return str(submodule_root.relative_to(parent_root))


def _is_gitlink_in_index(parent_root: Path, submodule_name: str) -> bool:
    """Return True when the submodule is tracked as a gitlink on the current branch.

    On branches where the submodule was never committed (e.g. the parent is on
    'updates' but the gitlink only lives on 'pipelines'), git add will fail.
    This check prevents that error.

    Args:
        parent_root: Absolute path to the parent repository root.
        submodule_name: Relative path of the submodule within the parent repo.

    Returns:
        True when ``git ls-files --stage`` shows a 160000 entry for the path.
    """
    entry = run_git(
        ["ls-files", "--stage", submodule_name], cwd=parent_root, check=False
    )
    return entry.startswith("160000")


def get_current_branch(repo_root: Path) -> str:
    """Return the currently checked-out branch name.

    Args:
        repo_root: Repository directory to inspect.

    Returns:
        Branch name, or ``'(detached HEAD)'`` when not on a named branch.
    """
    try:
        return run_git(["symbolic-ref", "--short", "HEAD"], cwd=repo_root)
    except subprocess.CalledProcessError:
        return "(detached HEAD)"


def get_current_sha(repo_root: Path) -> str:
    """Return the short SHA of the current HEAD commit.

    Args:
        repo_root: Repository directory to inspect.

    Returns:
        Abbreviated SHA string (7 characters).
    """
    return run_git(["rev-parse", "--short", "HEAD"], cwd=repo_root)


def is_working_tree_dirty(repo_root: Path) -> bool:
    """Return True if the submodule working tree has any uncommitted changes.

    Args:
        repo_root: Repository directory to inspect.

    Returns:
        ``True`` when ``git status --porcelain`` produces any output.
    """
    return bool(run_git(["status", "--porcelain"], cwd=repo_root))


def _get_git_dir(repo_root: Path) -> Path:
    """Resolve the actual git directory, following gitlinks for submodules.

    In a submodule, ``<root>/.git`` is a file (gitlink), not a directory.
    ``git rev-parse --git-dir`` always returns the real path.

    Args:
        repo_root: Repository directory to inspect.

    Returns:
        Absolute path to the ``.git`` directory (or the gitfile target).
    """
    git_dir = run_git(["rev-parse", "--git-dir"], cwd=repo_root)
    p = Path(git_dir)
    return p if p.is_absolute() else repo_root / p


def reset_working_tree(repo_root: Path) -> None:
    """Hard-reset the working tree to HEAD, discarding all local modifications.

    Args:
        repo_root: Repository directory to reset.

    Raises:
        SystemExit: If ``git reset`` fails.
    """
    try:
        run_git(["reset", "--hard", "HEAD"], cwd=repo_root)
        run_git(["clean", "-fd"], cwd=repo_root)
    except subprocess.CalledProcessError as exc:
        print(f"Error: git reset failed — {exc}")
        sys.exit(_EXIT_ERR)


def _confirm_and_reset(submodule_root: Path) -> bool:
    """Prompt to discard local changes if the working tree is dirty.

    Args:
        submodule_root: Absolute path to the submodule directory.

    Returns:
        ``True`` if the tree is clean or the user confirmed the reset.
        ``False`` if the user declined — caller should abort the operation.
    """
    if not is_working_tree_dirty(submodule_root):
        return True
    msg = "Warning: submodule has local modifications (possibly from a force push)."
    print(msg)
    raw = input("Discard local changes and reset to HEAD? (y/N): ").strip().lower()
    if raw != "y":
        print("Cancelled — local changes preserved.")
        return False
    reset_working_tree(submodule_root)
    return True


def get_remote_tracking_sha(repo_root: Path, branch: str) -> str | None:
    """Return the full SHA of ``origin/<branch>``, or ``None`` if not found.

    Args:
        repo_root: Repository directory to inspect.
        branch: Branch name (without the ``origin/`` prefix).

    Returns:
        Full 40-character SHA string, or ``None`` if the tracking ref is absent.
    """
    result = run_git(
        ["rev-parse", "--verify", f"origin/{branch}"], cwd=repo_root, check=False
    )
    return result or None


def get_remote_branches(repo_root: Path) -> list[str]:
    """Fetch remote refs and return all remote branch names.

    Args:
        repo_root: Repository directory to inspect.

    Returns:
        Sorted, deduplicated list of branch names without the ``origin/`` prefix.

    Raises:
        SystemExit: If ``git fetch`` fails.
    """
    print("Fetching remote branches...")
    try:
        run_git(["fetch", "--prune"], cwd=repo_root)
    except subprocess.CalledProcessError as exc:
        print(f"Error: git fetch failed — {exc}")
        sys.exit(_EXIT_ERR)
    raw = run_git(["branch", "-r"], cwd=repo_root)
    branches: list[str] = []
    for line in raw.splitlines():
        stripped = line.strip()
        if not stripped or "HEAD" in stripped:
            continue
        branches.append(stripped.removeprefix("origin/"))
    return sorted(set(branches))


# ── Commands ───────────────────────────────────────────────────────────────


def _pull_with_recovery(submodule_root: Path, branch: str) -> str | None:
    """Pull ``origin/<branch>``, recovering from merge conflicts and force pushes.

    Args:
        submodule_root: Absolute path to the submodule directory.
        branch: Branch name to pull.

    Returns:
        Output string describing what happened, or ``None`` if the user cancelled
        recovery (caller should return early without committing).
    """
    try:
        return run_git(["pull", "origin", branch], cwd=submodule_root)
    except subprocess.CalledProcessError as exc:
        merge_head = _get_git_dir(submodule_root) / "MERGE_HEAD"
        if merge_head.exists():
            print("Warning: pull left repo in a conflicted merge state — aborting.")
            run_git(["merge", "--abort"], cwd=submodule_root, check=False)
        print(f"Error: git pull failed — {exc}")
        msg = f"Reset hard to 'origin/{branch}' (discards all local changes)? (y/N): "
        if input(msg).strip().lower() != "y":
            print("Cancelled — no changes made.")
            return None
        run_git(["fetch", "origin"], cwd=submodule_root)
        run_git(["reset", "--hard", f"origin/{branch}"], cwd=submodule_root)
        run_git(["clean", "-fd"], cwd=submodule_root)
        return f"Reset to origin/{branch}."


def _select_branch(branches: list[str], current: str) -> str | None:
    """Display a numbered branch list and return the user's selection.

    Args:
        branches: Sorted list of available branch names.
        current: Currently checked-out branch name (marked with ``*``).

    Returns:
        Selected branch name, or ``None`` if the user cancelled or the
        selected branch is already the current one.
    """
    print(f"\nCurrent branch: {current}")
    print("\nAvailable branches:")
    for idx, branch in enumerate(branches, start=1):
        marker = " *" if branch == current else ""
        print(f"  {idx:2}. {branch}{marker}")
    print()
    raw = input("Enter branch number (or 'q' to cancel): ").strip()
    if raw.lower() == "q":
        return None
    if not raw.isdigit():
        print("Invalid input — expected a number.")
        return None
    choice = int(raw)
    if choice < 1 or choice > len(branches):
        print(f"Invalid choice — enter a number between 1 and {len(branches)}.")
        return None
    selected = branches[choice - 1]
    if selected == current:
        print(f"Already on '{selected}' — nothing to do.")
        return None
    return selected


def cmd_update(
    submodule_root: Path, parent_root: Path | None, submodule_name: str
) -> bool:
    """Pull the latest commits on the current branch and update the parent pointer.

    Args:
        submodule_root: Absolute path to the submodule directory.
        parent_root: Absolute path to the parent repository root, or ``None``
            when not inside a submodule.
        submodule_name: Relative path of the submodule within the parent repo.

    Returns:
        True when the submodule HEAD actually moved forward.
    """
    sha_before = run_git(["rev-parse", "HEAD"], cwd=submodule_root)
    current = get_current_branch(submodule_root)
    if not _confirm_and_reset(submodule_root):
        return False
    print(f"Pulling latest on '{current}' in '{submodule_name}'...")
    out = _pull_with_recovery(submodule_root, current)
    if out is None:
        return False
    print(out)
    local_sha = run_git(["rev-parse", "HEAD"], cwd=submodule_root)
    remote_sha = get_remote_tracking_sha(submodule_root, current)
    if remote_sha and local_sha != remote_sha:
        local_short = local_sha[:7]
        remote_short = remote_sha[:7]
        msg = _DIVERGE_WARNING.format(
            local=local_short,
            branch=current,
            remote=remote_short,
        )
        print(msg)
        if input("Fix now? (y/N): ").strip().lower() == "y":
            run_git(["reset", "--hard", f"origin/{current}"], cwd=submodule_root)
            run_git(["clean", "-fd"], cwd=submodule_root)
            print(f"Reset to origin/{current}.")
            local_sha = run_git(["rev-parse", "HEAD"], cwd=submodule_root)
        else:
            print("Skipped — submodule left at diverged state.")
    changed = local_sha != sha_before
    if parent_root is not None:
        if not _is_gitlink_in_index(parent_root, submodule_name):
            print(
                "  ⚠️  Submodule not tracked on this branch — "
                "skipping parent pointer update."
            )
        else:
            run_git(["add", submodule_name], cwd=parent_root)
            commit_msg = f"Update {submodule_name} submodule to latest '{current}'"
            commit_out = run_git(
                ["commit", "-m", commit_msg], cwd=parent_root, check=False
            )
            print(
                commit_out
                or "Nothing to commit — submodule pointer already up to date."
            )
    return changed


def cmd_switch_branch(
    submodule_root: Path, parent_root: Path | None, submodule_name: str
) -> bool:
    """Interactively pick a remote branch and switch the submodule to it.

    Args:
        submodule_root: Absolute path to the submodule directory.
        parent_root: Absolute path to the parent repository root, or ``None``
            when not inside a submodule.
        submodule_name: Relative path of the submodule within the parent repo.

    Returns:
        True when the branch was actually switched.
    """
    branches = get_remote_branches(submodule_root)
    if not branches:
        print("No remote branches found.")
        return False
    current = get_current_branch(submodule_root)
    selected = _select_branch(branches, current)
    if selected is None:
        return False
    print(f"\nSwitching to '{selected}'...")
    if not _confirm_and_reset(submodule_root):
        return False
    merge_head = _get_git_dir(submodule_root) / "MERGE_HEAD"
    if merge_head.exists():
        print("Warning: aborting in-progress merge before switching branch.")
        run_git(["merge", "--abort"], cwd=submodule_root, check=False)
    try:
        run_git(["checkout", "-B", selected, f"origin/{selected}"], cwd=submodule_root)
    except subprocess.CalledProcessError as exc:
        print(f"Error: git checkout failed — {exc}")
        sys.exit(_EXIT_ERR)
    print(f"Resetting to 'origin/{selected}'...")
    run_git(["reset", "--hard", f"origin/{selected}"], cwd=submodule_root)
    run_git(["clean", "-fd"], cwd=submodule_root)
    if parent_root is not None:
        if not _is_gitlink_in_index(parent_root, submodule_name):
            print(
                "  ⚠️  Submodule not tracked on this branch — "
                "skipping parent pointer update."
            )
        else:
            run_git(
                ["submodule", "set-branch", "--branch", selected, submodule_name],
                cwd=parent_root,
            )
            run_git(["add", ".gitmodules", submodule_name], cwd=parent_root)
            commit_msg = f"Pin {submodule_name} submodule to branch '{selected}'"
            commit_out = run_git(
                ["commit", "-m", commit_msg], cwd=parent_root, check=False
            )
            print(commit_out or "Nothing to commit.")
    print(f"\nSubmodule '{submodule_name}' is now on '{selected}'.")
    return True


def cmd_pin_commit(
    submodule_root: Path, parent_root: Path | None, submodule_name: str
) -> bool:
    """Pin the submodule to its current HEAD SHA and update the parent pointer.

    Args:
        submodule_root: Absolute path to the submodule directory.
        parent_root: Absolute path to the parent repository root, or ``None``
            when not inside a submodule.
        submodule_name: Relative path of the submodule within the parent repo.

    Returns:
        True when the parent pointer was updated.
    """
    sha = get_current_sha(submodule_root)
    branch = get_current_branch(submodule_root)
    print(f"Pinning '{submodule_name}' to {sha} (branch: {branch})...")
    if parent_root is not None:
        if not _is_gitlink_in_index(parent_root, submodule_name):
            print(
                "  ⚠️  Submodule not tracked on this branch — "
                "skipping parent pointer update."
            )
        else:
            run_git(["add", submodule_name], cwd=parent_root)
            commit_msg = f"Pin {submodule_name} submodule to commit {sha}"
            commit_out = run_git(
                ["commit", "-m", commit_msg], cwd=parent_root, check=False
            )
            print(
                commit_out
                or "Nothing to commit — submodule pointer already up to date."
            )
            return True
    return False


def cmd_update_via_worktree(
    submodule_root: Path,
    parent_root: Path,
    submodule_name: str,
) -> bool:
    """Update the submodule pointer on the pipelines branch via an isolated worktree.

    Creates a temporary worktree for the pipelines branch, updates the submodule
    pointer to the current HEAD SHA, commits, and pushes — without disturbing
    the caller's active branch.

    Args:
        submodule_root: Absolute path to the submodule directory.
        parent_root: Absolute path to the parent repository root.
        submodule_name: Relative path of the submodule within the parent repo.

    Returns:
        True when the submodule pointer was actually committed and pushed.

    Raises:
        SystemExit: If fetch, worktree creation, or push fails.
    """
    sha = run_git(["rev-parse", "HEAD"], cwd=submodule_root)
    branch = get_current_branch(submodule_root)
    short_sha = sha[:7]

    print(f"Fetching remote to verify '{_PIPELINES_BRANCH}' branch exists...")
    try:
        run_git(["fetch", "origin"], cwd=parent_root)
    except subprocess.CalledProcessError as exc:
        print(f"Error: git fetch failed — {exc}")
        sys.exit(_EXIT_ERR)

    remote_ref = run_git(
        ["rev-parse", "--verify", f"origin/{_PIPELINES_BRANCH}"],
        cwd=parent_root,
        check=False,
    )
    if not remote_ref:
        pipelines = _PIPELINES_BRANCH
        msg = inspect.cleandoc(f"""
            Error: '{pipelines}' branch does not exist on remote.
            Create it first, then re-run this option.
        """)
        print(msg)
        return False

    worktree_path = parent_root / _WORKTREE_DIR
    if worktree_path.exists():
        print(f"Removing stale worktree at '{_WORKTREE_DIR}'...")
        run_git(
            ["worktree", "remove", "--force", str(worktree_path)],
            cwd=parent_root,
            check=False,
        )

    print(f"Creating isolated worktree for '{_PIPELINES_BRANCH}'...")
    try:
        run_git(
            [
                "worktree",
                "add",
                "--detach",
                str(worktree_path),
                f"origin/{_PIPELINES_BRANCH}",
            ],
            cwd=parent_root,
        )
    except subprocess.CalledProcessError as exc:
        print(f"Error: git worktree add failed — {exc}")
        sys.exit(_EXIT_ERR)

    committed = False
    try:
        run_git(
            ["update-index", "--cacheinfo", f"160000,{sha},{submodule_name}"],
            cwd=worktree_path,
        )
        commit_msg = f"Update {submodule_name} to {short_sha} (branch: {branch})"
        commit_out = run_git(
            ["commit", "-m", commit_msg], cwd=worktree_path, check=False
        )
        print(commit_out or "Nothing to commit — submodule pointer already up to date.")
        if commit_out:
            committed = True
            print(f"Pushing '{_PIPELINES_BRANCH}' to origin...")
            push_out = run_git(
                ["push", "origin", f"HEAD:{_PIPELINES_BRANCH}"], cwd=worktree_path
            )
            print(push_out or f"Pushed '{_PIPELINES_BRANCH}' to origin.")
    finally:
        print("Cleaning up worktree...")
        run_git(
            ["worktree", "remove", "--force", str(worktree_path)],
            cwd=parent_root,
            check=False,
        )
    return committed


def push_to_origin(parent_root: Path) -> None:
    """Push the current parent branch to origin and exit.

    Args:
        parent_root: Absolute path to the parent repository root.

    Raises:
        SystemExit: If ``git push`` fails, or after a successful push.
    """
    branch = get_current_branch(parent_root)
    print(f"Pushing '{branch}' to origin...")
    try:
        out = run_git(["push", "origin", branch], cwd=parent_root)
    except subprocess.CalledProcessError as exc:
        print(f"Error: git push failed — {exc}")
        sys.exit(_EXIT_ERR)
    print(out or f"Pushed '{branch}' to origin.")


# ── Interactive menu ───────────────────────────────────────────────────────


def _read_key(prompt: str) -> str:
    """Read a single keypress without requiring Enter.

    Args:
        prompt: Text to display before waiting for input.

    Returns:
        Single character (lowercased) pressed by the user.
    """
    sys.stdout.write(prompt)
    sys.stdout.flush()
    if sys.platform == "win32":
        ch = msvcrt.getwch()
        if ch in ("\x00", "\xe0"):  # drain function/arrow key escape sequences
            msvcrt.getwch()
            ch = ""
    else:
        fd = sys.stdin.fileno()
        old_settings = termios.tcgetattr(fd)
        try:
            tty.setraw(fd)
            ch = sys.stdin.read(1)
        finally:
            termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)
        if ch == "\x03":  # Ctrl+C
            sys.stdout.write("\n")
            raise KeyboardInterrupt
    sys.stdout.write(ch + "\n")
    sys.stdout.flush()
    return ch.lower()


def _build_ops_menu(submodule_name: str, current_branch: str) -> str:
    """Build the operations menu string (update / switch / pin).

    Args:
        submodule_name: Relative path of the submodule within the parent repo.
        current_branch: Currently checked-out branch name.

    Returns:
        Formatted menu string ready to print.
    """
    header = inspect.cleandoc(f"""
        Submodule Manager
        -----------------
        Submodule : {submodule_name}
        Branch    : {current_branch}
    """)
    items = [
        "1. Update to latest (pull current branch)",
        "2. Switch to a different branch",
        "3. Pin to current commit",
        "q. Quit",
    ]
    return f"{header}\n\n" + "\n".join(items)


def _ops_menu_loop(
    submodule_root: Path,
    parent_root: Path | None,
    submodule_name: str,
) -> None:
    """Run the update / switch / pin menu loop.

    Args:
        submodule_root: Absolute path to the submodule directory.
        parent_root: Absolute path to the parent repository root, or ``None``.
        submodule_name: Relative path of the submodule within the parent repo.
    """
    while True:
        current_branch = get_current_branch(submodule_root)
        print(f"\n{_build_ops_menu(submodule_name, current_branch)}")
        choice = _read_key("Choice: ")
        if choice == "q":
            break
        if choice == "1":
            changed = cmd_update(submodule_root, parent_root, submodule_name)
        elif choice == "2":
            changed = cmd_switch_branch(submodule_root, parent_root, submodule_name)
        elif choice == "3":
            changed = cmd_pin_commit(submodule_root, parent_root, submodule_name)
        else:
            print("Invalid choice.")
            continue
        if changed and parent_root is not None:
            raw_push = input("\nPush to origin and exit? (y/N): ").strip().lower()
            if raw_push == "y":
                push_to_origin(parent_root)
                break


def _prompt_off_branch_context(
    submodule_root: Path,
    parent_root: Path,
    submodule_name: str,
    current_parent_branch: str,
) -> None:
    """Prompt the user to choose safe or direct mode when not on pipelines.

    Displays a prominent warning then asks whether to use the isolated
    worktree (safe) or operate directly on the current parent branch.

    Args:
        submodule_root: Absolute path to the submodule directory.
        parent_root: Absolute path to the parent repository root.
        submodule_name: Relative path of the submodule within the parent repo.
        current_parent_branch: Currently checked-out parent branch name.
    """
    pipelines = _PIPELINES_BRANCH
    branch_name = current_parent_branch
    warning = inspect.cleandoc(f"""
        \u26a0\ufe0f  OFF-BRANCH WARNING  \u26a0\ufe0f
        Parent repo is on '{branch_name}', not '{pipelines}'.
        Any direct commits will land on '{branch_name}'.
    """)
    print(f"\n{warning}\n")
    context_items = [
        f"1. \U0001f512 Safe  — update '{pipelines}' via isolated worktree",
        f"2. \u26a1 Direct — I know what I'm doing (operate on '{branch_name}')",
        "q. Quit",
    ]
    print("\n".join(context_items))
    while True:
        choice = _read_key("\nContext: ")
        if choice == "q":
            sys.exit(0)
        if choice == "1":
            cmd_update_via_worktree(
                submodule_root, parent_root, submodule_name
            )
            return
        if choice == "2":
            _ops_menu_loop(submodule_root, parent_root, submodule_name)
            return
        print("Invalid choice — enter 1, 2, or q.")


# ── Entry point ────────────────────────────────────────────────────────────


def main() -> None:
    """Resolve submodule context and launch the interactive manager."""
    submodule_root = find_submodule_root()
    parent_root = find_parent_repo_root(submodule_root)
    submodule_name = get_submodule_name(submodule_root, parent_root)
    if parent_root is not None:
        current_parent_branch = get_current_branch(parent_root)
        if current_parent_branch != _PIPELINES_BRANCH:
            _prompt_off_branch_context(
                submodule_root,
                parent_root,
                submodule_name,
                current_parent_branch,
            )
            return
    _ops_menu_loop(submodule_root, parent_root, submodule_name)


if __name__ == "__main__":
    main()
