"""Unit tests for push-to-cache.py helper behavior."""

from __future__ import annotations

import importlib.util
import subprocess
import sys
from pathlib import Path
from types import ModuleType


def _load_push_to_cache_module() -> ModuleType:
    module_path = (
        Path(__file__).resolve().parents[3] / "pipelines" / "scripts" / "push-to-cache.py"
    )
    module_spec = importlib.util.spec_from_file_location(
        "push_to_cache_module", module_path
    )
    if module_spec is None or module_spec.loader is None:
        raise RuntimeError(f"Unable to load module from {module_path}")

    module = importlib.util.module_from_spec(module_spec)
    sys.modules[module_spec.name] = module
    module_spec.loader.exec_module(module)
    return module


def test_clean_remote_strips_embedded_credentials() -> None:
    """Ensure clean remote removes user info from HTTPS URLs."""
    push_to_cache = _load_push_to_cache_module()

    cleaned = push_to_cache._clean_remote("https://user:token@example.com/scm/a/repo.git")

    assert cleaned == "https://example.com/scm/a/repo.git"


def test_is_stale_info_reject_detects_stale_push() -> None:
    """Ensure stale-info errors are recognized for retry behavior."""
    push_to_cache = _load_push_to_cache_module()
    failed_push = subprocess.CompletedProcess(
        args=["git", "push"],
        returncode=1,
        stdout="",
        stderr="[rejected] stale info",
    )

    assert push_to_cache._is_stale_info_reject(failed_push) is True


def test_run_git_push_retries_once_on_stale_info(tmp_path: Path) -> None:
    """Ensure stale-info rejection triggers fetch/rebase and a single retry."""
    push_to_cache = _load_push_to_cache_module()
    git_calls: list[list[str]] = []
    stale = subprocess.CompletedProcess(
        args=["git", "push"],
        returncode=1,
        stdout="stale info",
        stderr="",
    )
    success = subprocess.CompletedProcess(
        args=["git", "push"],
        returncode=0,
        stdout="ok",
        stderr="",
    )
    responses = [stale, success]
    retries: list[str] = []

    def _fake_run_git_command(
        _worktree: Path,
        _auth_header: str,
        args: list[str],
    ) -> subprocess.CompletedProcess[str]:
        git_calls.append(args)
        return responses.pop(0)

    def _fake_fetch_remote_branch(
        _worktree: Path,
        _remote_url: str,
        _auth_header: str,
        _branch: str,
    ) -> None:
        retries.append("fetch")

    def _fake_rebase_onto_tracking_branch(_worktree: Path, _branch: str) -> None:
        retries.append("rebase")

    push_to_cache._run_git_command = _fake_run_git_command
    push_to_cache._fetch_remote_branch = _fake_fetch_remote_branch
    push_to_cache._rebase_onto_tracking_branch = _fake_rebase_onto_tracking_branch

    push_to_cache._run_git_push(
        tmp_path,
        "https://example.com/scm/a/repo.git",
        "Authorization: Bearer token",
        "majordomo-pr-reviewer-cache/project-a",
    )

    assert len(git_calls) == 2
    assert retries == ["fetch", "rebase"]


def test_main_returns_pattern_violation_for_invalid_branch(
    monkeypatch,
    tmp_path: Path,
) -> None:
    """Ensure invalid branch names return the branch-policy exit code."""
    push_to_cache = _load_push_to_cache_module()

    monkeypatch.setattr(
        "sys.argv",
        [
            "push-to-cache.py",
            "--remote",
            "https://example.com/scm/a/repo.git",
            "--branch",
            "feature/not-allowed",
            "--worktree",
            str(tmp_path),
        ],
    )

    assert push_to_cache.main() == push_to_cache._EXIT_PATTERN_VIOLATION