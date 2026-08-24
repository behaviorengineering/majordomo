"""Unit tests for git-diff-prep.py helper behavior."""

from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
from pathlib import Path
from types import ModuleType

import pytest


def _load_git_diff_prep_module() -> ModuleType:
    module_path = (
        Path(__file__).resolve().parents[3] / "pipelines" / "scripts" / "git-diff-prep.py"
    )
    module_spec = importlib.util.spec_from_file_location("git_diff_prep_module", module_path)
    if module_spec is None or module_spec.loader is None:
        raise RuntimeError(f"Unable to load module from {module_path}")

    module = importlib.util.module_from_spec(module_spec)
    sys.modules[module_spec.name] = module
    module_spec.loader.exec_module(module)
    return module


def test_build_staging_filename_keeps_byte_limit() -> None:
    """Ensure generated staging filenames never exceed configured byte limit."""
    prep = _load_git_diff_prep_module()
    long_slug = "x" * 600

    file_name = prep.build_staging_filename(long_slug)

    assert file_name.endswith(".txt")
    assert len(file_name.encode("utf-8")) <= prep.MAX_STAGE_FILENAME_BYTES
    assert "-" in file_name


def test_is_excluded_matches_expected_patterns() -> None:
    """Ensure lockfiles, minified files, and build outputs are excluded."""
    prep = _load_git_diff_prep_module()

    assert prep.is_excluded("uv.lock") is True
    assert prep.is_excluded("frontend/app.min.js") is True
    assert prep.is_excluded("dist/package.js") is True
    assert prep.is_excluded("src/app.py") is False


def test_load_agent_context_config_supports_legacy_flat_form(tmp_path: Path) -> None:
    """Ensure flat legacy context files are normalized into global/scoped keys."""
    prep = _load_git_diff_prep_module()
    config_file = tmp_path / "agent-context.json"
    config_file.write_text(
        json.dumps(
            {
                "techStack": ["python"],
                "customRules": ["prefer explicit error handling"],
            }
        ),
        encoding="utf-8",
    )

    parsed = prep.load_agent_context_config(config_file)

    assert parsed["scoped"] == {}
    assert parsed["global"]["techStack"] == ["python"]
    assert parsed["global"]["customRules"] == ["prefer explicit error handling"]


def test_resolve_routing_personas_missing_file_exits(tmp_path: Path) -> None:
    """Ensure missing persona files fail fast with SystemExit."""
    prep = _load_git_diff_prep_module()

    with pytest.raises(SystemExit):
        prep._resolve_routing_personas(
            {"pr-review-code": "agents/missing.persona.md"},
            tmp_path,
        )


def test_context_for_file_invalid_scoped_context_exits(tmp_path: Path) -> None:
    """Ensure non-dict scoped context entries fail fast."""
    prep = _load_git_diff_prep_module()
    agent_context = {
        "global": {"customRules": ["rule-a"]},
        "scoped": {"src/**": "invalid"},
    }

    with pytest.raises(SystemExit):
        prep._context_for_file("src/app.py", agent_context, tmp_path)


def test_parse_name_status_uses_destination_for_rename_and_copy() -> None:
    """Ensure rename/copy statuses keep the destination path for routing."""
    prep = _load_git_diff_prep_module()
    raw = "R100\x00src/old.py\x00src/new.py\x00C075\x00a.txt\x00b.txt\x00"

    parsed = prep.parse_name_status(raw)

    assert parsed == [("R", "src/new.py"), ("C", "b.txt")]


def test_parse_name_status_handles_regular_status_entries() -> None:
    """Ensure non-rename statuses parse into (status, path) pairs."""
    prep = _load_git_diff_prep_module()
    raw = "A\x00src/add.py\x00M\x00src/mod.py\x00D\x00src/del.py\x00"

    parsed = prep.parse_name_status(raw)

    assert parsed == [
        ("A", "src/add.py"),
        ("M", "src/mod.py"),
        ("D", "src/del.py"),
    ]


def test_get_submodule_exclusions_parses_cached_status(monkeypatch) -> None:
    """Ensure cached submodule status lines become anchored exclusion patterns."""
    prep = _load_git_diff_prep_module()

    fake_completed = subprocess.CompletedProcess(
        args=["git", "submodule", "status", "--cached"],
        returncode=0,
        stdout=" abc123 .majordomo (heads/main)\n-fff111 vendor/lib\n",
        stderr="",
    )

    def _fake_run(*_args, **_kwargs):
        return fake_completed

    monkeypatch.setattr(prep.subprocess, "run", _fake_run)

    patterns = prep.get_submodule_exclusions()

    assert any(pattern.search(".majordomo/pipelines/scripts/git-diff-prep.py") for pattern in patterns)
    assert any(pattern.search("vendor/lib/src/main.py") for pattern in patterns)


def test_load_routing_falls_back_to_defaults_for_invalid_json(tmp_path: Path) -> None:
    """Ensure malformed routing JSON falls back to built-in defaults."""
    prep = _load_git_diff_prep_module()
    routing_file = tmp_path / "routing.json"
    routing_file.write_text("{not-valid-json", encoding="utf-8")

    rules, persona_paths = prep.load_routing(routing_file)

    assert rules == prep.DEFAULT_ROUTING
    assert persona_paths == {}


def test_load_routing_exits_for_invalid_rule_shape(tmp_path: Path) -> None:
    """Ensure invalid routing entry shapes fail fast with SystemExit."""
    prep = _load_git_diff_prep_module()
    routing_file = tmp_path / "routing.json"
    routing_file.write_text(
        json.dumps({"pr-review-code": {"globs": "**/*.py"}}),
        encoding="utf-8",
    )

    with pytest.raises(SystemExit):
        prep.load_routing(routing_file)