"""Constrained cache-branch push helper.

This script is the only allowed push path for cache branches. It validates
branch naming constraints, injects an HTTPS token from the environment, and
pushes using an explicit refspec.
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit

_CACHE_BRANCH_RE = re.compile(r"^majordomo-pr-reviewer-cache/[a-z0-9][a-z0-9-]*$")
_EXIT_PATTERN_VIOLATION = 42


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Push cache branch with constraints")
    parser.add_argument("--remote", required=True)
    parser.add_argument("--branch", required=True)
    parser.add_argument("--worktree", type=Path, required=True)
    return parser


def _require_env(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise ValueError(f"Missing required environment variable: {name}")
    return value


def _redact_remote(remote_url: str) -> str:
    parsed = urlsplit(remote_url)
    netloc = parsed.netloc
    if "@" not in netloc:
        return remote_url

    _, host_part = netloc.rsplit("@", 1)
    return urlunsplit(
        (
            parsed.scheme,
            f"***:***@{host_part}",
            parsed.path,
            parsed.query,
            parsed.fragment,
        )
    )


def _clean_remote(remote_url: str) -> str:
    """Strip any embedded credentials from the remote URL."""
    parsed = urlsplit(remote_url)
    if parsed.scheme != "https":
        raise ValueError("Remote URL must be https")
    host_part = parsed.netloc.split("@")[-1]
    return urlunsplit(
        (parsed.scheme, host_part, parsed.path, parsed.query, parsed.fragment)
    )


def _bearer_header(token: str) -> str:
    """Build a git http.extraHeader value for Bearer token auth."""
    return f"Authorization: Bearer {token}"


def _run_git_command(
    worktree: Path, auth_header: str, args: list[str]
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [
            "git",
            "-C",
            str(worktree),
            "-c",
            f"http.extraHeader={auth_header}",
            *args,
        ],
        check=False,
        capture_output=True,
        text=True,
    )


def _print_git_output(completed: subprocess.CompletedProcess[str]) -> None:
    if completed.stdout.strip():
        print(completed.stdout.strip())
    if completed.stderr.strip():
        print(completed.stderr.strip(), file=sys.stderr)


def _is_stale_info_reject(completed: subprocess.CompletedProcess[str]) -> bool:
    output = f"{completed.stdout}\n{completed.stderr}".lower()
    return completed.returncode != 0 and "stale info" in output


def _fetch_remote_branch(
    worktree: Path, remote_url: str, auth_header: str, branch: str
) -> None:
    remote_ref = f"refs/heads/{branch}"
    tracking_ref = f"refs/remotes/origin/{branch}"
    completed = _run_git_command(
        worktree,
        auth_header,
        ["fetch", "--depth=1", remote_url, f"+{remote_ref}:{tracking_ref}"],
    )
    _print_git_output(completed)
    if completed.returncode != 0:
        raise RuntimeError(f"git fetch failed with exit code {completed.returncode}")


def _rebase_onto_tracking_branch(worktree: Path, branch: str) -> None:
    tracking_ref = f"refs/remotes/origin/{branch}"
    completed = subprocess.run(
        ["git", "-C", str(worktree), "rebase", tracking_ref],
        check=False,
        capture_output=True,
        text=True,
    )
    _print_git_output(completed)
    if completed.returncode == 0:
        return

    subprocess.run(
        ["git", "-C", str(worktree), "rebase", "--abort"],
        check=False,
        capture_output=True,
        text=True,
    )
    raise RuntimeError(f"git rebase failed with exit code {completed.returncode}")


def _run_git_push(
    worktree: Path, remote_url: str, auth_header: str, branch: str
) -> None:
    refspec = f"HEAD:refs/heads/{branch}"
    print(f"cache push remote: {_redact_remote(remote_url)}")
    print(f"cache push branch: {branch}")
    print(f"cache push refspec: {refspec}")

    # Push to 'origin' instead of raw remote_url to allow git to find and match
    # the local tracking ref for --force-with-lease (since tracking refs are
    # associated with remote names, not raw URLs).
    completed = _run_git_command(
        worktree,
        auth_header,
        ["push", "origin", refspec, "--force-with-lease"],
    )
    _print_git_output(completed)
    if completed.returncode == 0:
        return

    if not _is_stale_info_reject(completed):
        raise RuntimeError(f"git push failed with exit code {completed.returncode}")

    print(
        "cache push rejected as stale info; reconciling and retrying once",
        file=sys.stderr,
    )
    _fetch_remote_branch(worktree, remote_url, auth_header, branch)
    _rebase_onto_tracking_branch(worktree, branch)

    retry = _run_git_command(
        worktree,
        auth_header,
        ["push", "origin", refspec, "--force-with-lease"],
    )
    _print_git_output(retry)
    if retry.returncode != 0:
        raise RuntimeError(f"git push retry failed with exit code {retry.returncode}")


def main() -> int:
    parser = _build_parser()
    args = parser.parse_args()

    try:
        if not _CACHE_BRANCH_RE.fullmatch(args.branch):
            print("branch pattern validation failed", file=sys.stderr)
            return _EXIT_PATTERN_VIOLATION
        if not args.worktree.exists():
            raise ValueError(f"worktree does not exist: {args.worktree}")

        token = _require_env("BITBUCKET_TOKEN")
        clean_remote = _clean_remote(args.remote)
        auth_header = _bearer_header(token)
        _run_git_push(args.worktree, clean_remote, auth_header, args.branch)
        return 0
    except (OSError, RuntimeError, ValueError) as err:
        print(str(err), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
