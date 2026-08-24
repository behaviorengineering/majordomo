"""Unit tests for review-cache.py helper behavior."""

from __future__ import annotations

import argparse
import importlib.util
import json
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path
from types import ModuleType

import pytest


def _load_review_cache_module() -> ModuleType:
    module_path = (
        Path(__file__).resolve().parents[3] / "pipelines" / "scripts" / "review-cache.py"
    )
    module_spec = importlib.util.spec_from_file_location("review_cache_module", module_path)
    if module_spec is None or module_spec.loader is None:
        raise RuntimeError(f"Unable to load module from {module_path}")

    module = importlib.util.module_from_spec(module_spec)
    sys.modules[module_spec.name] = module
    module_spec.loader.exec_module(module)
    return module


def test_build_index_keeps_skill_scoped_entries() -> None:
    """Ensure index keys remain isolated by skill_name when present."""
    review_cache = _load_review_cache_module()
    now = datetime.now(UTC)

    shared_sha = "a" * 64
    record_a = review_cache._CacheRecord(
        file_path=Path("skill-a/analysis-a.json"),
        metadata={"cluster_sha": shared_sha, "skill_name": "skill-a"},
        created_at=now,
    )
    record_b = review_cache._CacheRecord(
        file_path=Path("skill-b/analysis-a.json"),
        metadata={"cluster_sha": shared_sha, "skill_name": "skill-b"},
        created_at=now,
    )

    index = review_cache._build_index([record_a, record_b])

    assert f"skill-a:{shared_sha}" in index
    assert f"skill-b:{shared_sha}" in index
    assert shared_sha not in index


def test_resolve_lookup_entry_prefers_skill_key_then_fallback() -> None:
    """Ensure lookup first checks skill-prefixed keys, then plain cluster key."""
    review_cache = _load_review_cache_module()
    cluster_sha = "b" * 64

    skill_entry = {
        "cluster_sha": cluster_sha,
        "skill_name": "pr-review-summary",
        "file": "pr-review-summary/analysis.json",
    }
    fallback_entry = {
        "cluster_sha": cluster_sha,
        "file": "generic/analysis.json",
    }

    raw_index = {
        f"pr-review-summary:{cluster_sha}": skill_entry,
        cluster_sha: fallback_entry,
    }

    resolved = review_cache._resolve_lookup_entry(
        raw_index,
        cluster_sha,
        "pr-review-summary",
    )
    assert resolved == skill_entry

    resolved_fallback = review_cache._resolve_lookup_entry(raw_index, cluster_sha, "")
    assert resolved_fallback == fallback_entry


def test_parse_cluster_files_argument_normalizes_and_sorts(tmp_path: Path) -> None:
    """Ensure merged cluster file inputs are normalized and sorted."""
    review_cache = _load_review_cache_module()

    files_list = tmp_path / "cluster-files.txt"
    files_list.write_text("src/z.py\nsrc/a.py\n", encoding="utf-8")

    parsed = review_cache._parse_cluster_files_argument(
        cluster_files=["src\\m.py", "  ", "src/b.py"],
        cluster_files_file=files_list,
    )

    assert parsed == ["src/a.py", "src/b.py", "src/m.py", "src/z.py"]


def test_collect_markdown_artifact_filters_path_traversal(tmp_path: Path) -> None:
    """Ensure artifact collection ignores traversal-style artifact names."""
    review_cache = _load_review_cache_module()

    analysis_file = tmp_path / "manifest.json"
    analysis_file.write_text('{"reviewable": []}\n', encoding="utf-8")

    reports_dir = tmp_path / "reports"
    reports_dir.mkdir(parents=True)
    (reports_dir / "summary.md").write_text("ok\n", encoding="utf-8")
    (reports_dir / "evil.md").write_text("nope\n", encoding="utf-8")

    markdown_files, _artifact_hash = review_cache._collect_markdown_artifact(
        analysis_file,
        reports_dir,
        ["summary.md", "../evil.md", "nested/evil.md"],
    )

    assert "summary.md" in markdown_files
    assert "../evil.md" not in markdown_files
    assert "nested/evil.md" not in markdown_files


def test_restore_command_rejects_entry_file_outside_cache_dir(tmp_path: Path) -> None:
    """Ensure restore blocks entry-file path traversal outside cache-dir."""
    review_cache = _load_review_cache_module()
    cache_dir = tmp_path / "cache"
    cache_dir.mkdir(parents=True)

    args = review_cache.argparse.Namespace(
        cache_dir=cache_dir,
        entry_file="../outside.json",
        output_dir=tmp_path / "output",
    )

    with pytest.raises(ValueError, match="inside cache-dir"):
        review_cache._restore_command(args)


def test_precheck_prunes_expired_and_keeps_valid_entries(
    tmp_path: Path,
    capsys,
) -> None:
    """Ensure precheck deletes expired records and retains valid index entries."""
    review_cache = _load_review_cache_module()
    cache_dir = tmp_path / "cache"
    cache_dir.mkdir(parents=True)

    cluster_files = ["src/main.py"]
    cluster_files_hash = review_cache._compute_cluster_files_hash(cluster_files)

    def _write_cache_entry(cluster_sha: str, created_at: str) -> Path:
        metadata = {
            "cluster_sha": cluster_sha,
            "skill_name": "pr-review-code",
            "fingerprint_version": "v1",
            "cluster_files": cluster_files,
            "cluster_files_hash": cluster_files_hash,
            "model_id": "gpt-5.3-codex",
            "instruction_bundle_hash": "instr-hash",
            "prompt_template_hash": "prompt-hash",
            "scoring_rubric_hash": "rubric-hash",
            "output_schema_version": "1",
            "analysis_payload_hash": "payload-hash",
            "created_at": created_at,
        }
        skill_dir = cache_dir / "pr-review-code"
        skill_dir.mkdir(parents=True, exist_ok=True)
        entry_path = skill_dir / f"analysis-{cluster_sha}.json"
        frontmatter = review_cache._format_frontmatter(metadata)
        entry_path.write_text(f"{frontmatter}\n{{\"ok\": true}}\n", encoding="utf-8")
        return entry_path

    valid_sha = "a" * 64
    expired_sha = "b" * 64
    valid_created_at = datetime.now(UTC).strftime(review_cache._TIMESTAMP_FMT)
    expired_created_at = (datetime.now(UTC) - timedelta(days=500)).strftime(
        review_cache._TIMESTAMP_FMT
    )

    valid_path = _write_cache_entry(valid_sha, valid_created_at)
    expired_path = _write_cache_entry(expired_sha, expired_created_at)

    args = argparse.Namespace(
        project_id="demo",
        cache_dir=cache_dir,
        project_retention_days=None,
        central_retention_days=None,
        global_retention_days=180,
        min_retention_days=30,
        index_out=None,
    )

    result = review_cache._precheck_command(args)
    captured = capsys.readouterr()
    payload = json.loads(captured.out)

    assert result == 0
    assert payload["expired_deleted"] == 1
    assert payload["valid_entries"] == 1
    assert payload["scanned_files"] == 1
    assert valid_path.exists()
    assert not expired_path.exists()