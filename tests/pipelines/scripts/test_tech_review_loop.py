"""Unit tests for tech-review-loop.py loop semantics."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from types import ModuleType


def _load_tech_review_loop_module() -> ModuleType:
    module_path = (
        Path(__file__).resolve().parents[3] / "pipelines" / "scripts" / "tech-review-loop.py"
    )
    module_spec = importlib.util.spec_from_file_location("tech_review_loop_module", module_path)
    if module_spec is None or module_spec.loader is None:
        raise RuntimeError(f"Unable to load module from {module_path}")

    module = importlib.util.module_from_spec(module_spec)
    sys.modules[module_spec.name] = module
    module_spec.loader.exec_module(module)
    return module


def test_feedback_written_to_skill_scoped_path(tmp_path: Path) -> None:
    """Ensure intermediate feedback is written under pr-review-technical staging."""
    tech_loop = _load_tech_review_loop_module()
    staging_dir = tmp_path / "batch_000"
    output_dir = tmp_path / "pipeline-output" / "pr-review-technical"
    output_dir.mkdir(parents=True)

    score_file = output_dir.parent / "tech-score.md"
    tech_review_file = output_dir.parent / "tech-review.md"
    tech_review_file.write_text("review\n", encoding="utf-8")

    scores = iter([5, 12])

    def _fake_run_dispatch(args: list[str]) -> int:
        if "--tech-score" in args:
            current = next(scores)
            score_file.write_text(f"SCORE: {current}\n", encoding="utf-8")
        return 0

    tech_loop._run_dispatch = _fake_run_dispatch

    exit_code = tech_loop.run_tech_review_loop(
        pr_number="123",
        staging_dir=staging_dir,
        output_dir=output_dir,
        pass_score=10,
        max_iter=3,
    )

    assert exit_code == 0
    assert (staging_dir / "pr-review-technical" / "tech_feedback.md").exists()
    assert not (staging_dir / "tech_feedback.md").exists()


def test_feedback_not_written_on_final_iteration(tmp_path: Path) -> None:
    """Ensure no feedback file is emitted after the final failed iteration."""
    tech_loop = _load_tech_review_loop_module()
    staging_dir = tmp_path / "batch_000"
    output_dir = tmp_path / "pipeline-output" / "pr-review-technical"
    output_dir.mkdir(parents=True)

    score_file = output_dir.parent / "tech-score.md"
    tech_review_file = output_dir.parent / "tech-review.md"
    tech_review_file.write_text("review\n", encoding="utf-8")

    def _fake_run_dispatch(_args: list[str]) -> int:
        score_file.write_text("SCORE: 4\n", encoding="utf-8")
        return 0

    tech_loop._run_dispatch = _fake_run_dispatch

    exit_code = tech_loop.run_tech_review_loop(
        pr_number="123",
        staging_dir=staging_dir,
        output_dir=output_dir,
        pass_score=10,
        max_iter=1,
    )

    assert exit_code == 0
    assert not (staging_dir / "pr-review-technical" / "tech_feedback.md").exists()