"""Unit tests for publish-pr-summary.py behavior."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from types import ModuleType

import pytest


def _load_publish_pr_summary_module() -> ModuleType:
    module_path = (
        __import__("pathlib").Path(__file__).resolve().parents[3]
        / "pipelines"
        / "scripts"
        / "publish-pr-summary.py"
    )
    module_spec = importlib.util.spec_from_file_location(
        "publish_pr_summary_module", module_path
    )
    if module_spec is None or module_spec.loader is None:
        raise RuntimeError(f"Unable to load module from {module_path}")

    module = importlib.util.module_from_spec(module_spec)
    sys.modules[module_spec.name] = module
    module_spec.loader.exec_module(module)
    return module


class _FakeClient:
    def __init__(self, description: str) -> None:
        self._description = description
        self.put_calls: list[str] = []
        self.comment_calls: list[str] = []

    def get_pr(self) -> dict:
        return {
            "description": self._description,
            "version": 1,
            "title": "t",
            "toRef": {"id": "refs/heads/master"},
            "reviewers": [],
        }

    def put_description(self, _pr_meta: dict, description: str) -> None:
        self.put_calls.append(description)

    def post_comment(self, text: str) -> None:
        self.comment_calls.append(text)


def test_has_body_content_skips_headings_and_comments() -> None:
    """Ensure publish skips markdown with no meaningful body lines."""
    publish = _load_publish_pr_summary_module()

    assert publish._has_body_content("# Heading\n\n<!-- note -->\n") is False
    assert publish._has_body_content("# Heading\n\nActual summary\n") is True


def test_run_auto_claims_empty_description() -> None:
    """Ensure auto mode claims empty PR descriptions with marker content."""
    publish = _load_publish_pr_summary_module()
    client = _FakeClient(description="")

    publish._run_auto(client, "Review body")

    assert len(client.put_calls) == 1
    assert publish.COPILOT_MARKER in client.put_calls[0]
    assert not client.comment_calls


def test_run_auto_updates_owned_description() -> None:
    """Ensure auto mode updates descriptions already owned by copilot marker."""
    publish = _load_publish_pr_summary_module()
    client = _FakeClient(description=f"{publish.COPILOT_MARKER}\nOld")

    publish._run_auto(client, "New body")

    assert len(client.put_calls) == 1
    assert "New body" in client.put_calls[0]
    assert not client.comment_calls


def test_run_auto_comments_when_user_owns_description(monkeypatch) -> None:
    """Ensure auto mode posts a link comment when PR description is user-owned."""
    publish = _load_publish_pr_summary_module()
    client = _FakeClient(description="User-owned description")
    monkeypatch.setenv("SUMMARY_ARTIFACT_URL", "https://example.test/summary")

    publish._run_auto(client, "Summary")

    assert not client.put_calls
    assert len(client.comment_calls) == 1
    assert "https://example.test/summary" in client.comment_calls[0]


def test_main_exits_for_invalid_mode(tmp_path: Path, monkeypatch) -> None:
    """Ensure invalid publish mode fails fast before any client operations."""
    publish = _load_publish_pr_summary_module()
    summary_file = tmp_path / "summary.md"
    summary_file.write_text("Actual content\n", encoding="utf-8")

    monkeypatch.setattr(
        "sys.argv",
        ["publish-pr-summary.py", "12", str(summary_file), "invalid-mode"],
    )

    with pytest.raises(SystemExit) as exc:
        publish.main()

    assert exc.value.code == 1


def test_main_exits_when_required_env_missing(tmp_path: Path, monkeypatch) -> None:
    """Ensure publish aborts when required BITBUCKET env vars are missing."""
    publish = _load_publish_pr_summary_module()
    summary_file = tmp_path / "summary.md"
    summary_file.write_text("Actual content\n", encoding="utf-8")

    monkeypatch.delenv("BITBUCKET_URL", raising=False)
    monkeypatch.setattr(
        "sys.argv",
        ["publish-pr-summary.py", "12", str(summary_file), "comment"],
    )

    with pytest.raises(SystemExit) as exc:
        publish.main()

    assert exc.value.code == 1


def test_build_review_links_preserves_expected_order(monkeypatch) -> None:
    """Ensure summary, technical, deep, then SA links are emitted in order."""
    publish = _load_publish_pr_summary_module()
    monkeypatch.setenv("SUMMARY_HTML_ARTIFACT_URL", "https://example.test/summary.html")
    monkeypatch.setenv("TECH_REVIEW_ARTIFACT_URL", "https://example.test/tech.html")
    monkeypatch.setenv("TECH_DEEP_ARTIFACT_URL", "https://example.test/deep.html")
    monkeypatch.setenv(
        "SA_ARTIFACT_URLS",
        json.dumps(
            [
                {"slug": "ruff", "url": "https://example.test/ruff.html"},
                {"slug": "mypy", "url": "https://example.test/mypy.html"},
            ]
        ),
    )

    links = publish._build_review_links()

    assert "PR Summary" in links[0]
    assert "Technical Review" in links[1]
    assert "Technical Deep Review" in links[2]
    assert "ruff" in links[3]
    assert "mypy" in links[4]


def test_build_review_links_uses_fallback_summary_url(monkeypatch) -> None:
    """Ensure fallback summary URL is used when summary HTML URL is absent."""
    publish = _load_publish_pr_summary_module()
    monkeypatch.delenv("SUMMARY_HTML_ARTIFACT_URL", raising=False)
    monkeypatch.delenv("TECH_REVIEW_ARTIFACT_URL", raising=False)
    monkeypatch.delenv("TECH_DEEP_ARTIFACT_URL", raising=False)
    monkeypatch.delenv("SA_ARTIFACT_URLS", raising=False)

    links = publish._build_review_links(summary_fallback_url="https://fallback.test/summary")

    assert len(links) == 1
    assert "fallback.test/summary" in links[0]