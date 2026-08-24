"""Unit tests for summary-loop.py loop semantics."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from types import ModuleType


def _load_summary_loop_module() -> ModuleType:
    module_path = (
        Path(__file__).resolve().parents[3] / "pipelines" / "scripts" / "summary-loop.py"
    )
    module_spec = importlib.util.spec_from_file_location("summary_loop_module", module_path)
    if module_spec is None or module_spec.loader is None:
        raise RuntimeError(f"Unable to load module from {module_path}")

    module = importlib.util.module_from_spec(module_spec)
    sys.modules[module_spec.name] = module
    module_spec.loader.exec_module(module)
    return module


def test_feedback_written_only_between_iterations(tmp_path: Path) -> None:
    """Ensure summary feedback is written after a failed non-final iteration."""
    summary_loop = _load_summary_loop_module()
    staging_dir = tmp_path / "batch_000"
    staging_dir.mkdir(parents=True)
    output_dir = tmp_path / "pipeline-output" / "pr-review-summary"
    output_dir.mkdir(parents=True)

    score_file = output_dir.parent / "score.md"
    summary_file = output_dir.parent / "summary.md"
    summary_file.write_text("summary\n", encoding="utf-8")

    scores = iter([9, 16])

    def _fake_run_dispatch(args: list[str]) -> int:
        if "--score" in args:
            score_file.write_text(f"SCORE: {next(scores)}\n", encoding="utf-8")
        return 0

    summary_loop._run_dispatch = _fake_run_dispatch

    exit_code = summary_loop.run_summary_loop(
        pr_number="123",
        staging_dir=staging_dir,
        output_dir=output_dir,
        pass_score=15,
        max_iter=3,
    )

    assert exit_code == 0
    assert (staging_dir / "score_feedback.md").exists()


def test_no_feedback_written_on_final_failed_iteration(tmp_path: Path) -> None:
    """Ensure summary feedback is not emitted after the final failed attempt."""
    summary_loop = _load_summary_loop_module()
    staging_dir = tmp_path / "batch_000"
    staging_dir.mkdir(parents=True)
    output_dir = tmp_path / "pipeline-output" / "pr-review-summary"
    output_dir.mkdir(parents=True)

    score_file = output_dir.parent / "score.md"
    summary_file = output_dir.parent / "summary.md"
    summary_file.write_text("summary\n", encoding="utf-8")

    def _fake_run_dispatch(args: list[str]) -> int:
        if "--score" in args:
            score_file.write_text("SCORE: 4\n", encoding="utf-8")
        return 0

    summary_loop._run_dispatch = _fake_run_dispatch

    exit_code = summary_loop.run_summary_loop(
        pr_number="123",
        staging_dir=staging_dir,
        output_dir=output_dir,
        pass_score=15,
        max_iter=1,
    )

    assert exit_code == 0
    assert not (staging_dir / "score_feedback.md").exists()


def test_run_summary_loop_fails_when_score_is_unparseable(tmp_path: Path) -> None:
    """Ensure malformed score output fails fast with exit code 1."""
    summary_loop = _load_summary_loop_module()
    staging_dir = tmp_path / "batch_000"
    staging_dir.mkdir(parents=True)
    output_dir = tmp_path / "pipeline-output" / "pr-review-summary"
    output_dir.mkdir(parents=True)

    score_file = output_dir.parent / "score.md"
    summary_file = output_dir.parent / "summary.md"
    summary_file.write_text("summary\n", encoding="utf-8")

    def _fake_run_dispatch(args: list[str]) -> int:
        if "--score" in args:
            score_file.write_text("NOT_A_SCORE\n", encoding="utf-8")
        return 0

    summary_loop._run_dispatch = _fake_run_dispatch

    exit_code = summary_loop.run_summary_loop(
        pr_number="123",
        staging_dir=staging_dir,
        output_dir=output_dir,
        pass_score=15,
        max_iter=2,
    )

    assert exit_code == 1