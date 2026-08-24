"""Unit tests for review-to-junit.py conversion behavior."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from types import ModuleType


def _load_review_to_junit_module() -> ModuleType:
    module_path = (
        Path(__file__).resolve().parents[3] / "pipelines" / "scripts" / "review-to-junit.py"
    )
    module_spec = importlib.util.spec_from_file_location(
        "review_to_junit_module", module_path
    )
    if module_spec is None or module_spec.loader is None:
        raise RuntimeError(f"Unable to load module from {module_path}")

    module = importlib.util.module_from_spec(module_spec)
    sys.modules[module_spec.name] = module
    module_spec.loader.exec_module(module)
    return module


def test_sanitize_removes_illegal_xml_characters() -> None:
    """Ensure XML-illegal control characters are stripped from output text."""
    review_to_junit = _load_review_to_junit_module()

    sanitized = review_to_junit._sanitize("good\x00text\x1fdone")

    assert sanitized == "goodtextdone"


def test_build_testsuite_skips_aux_files_and_sa_duplicates(tmp_path: Path) -> None:
    """Ensure only actionable findings become test cases in generated suite."""
    review_to_junit = _load_review_to_junit_module()
    skill_dir = tmp_path / "skill"
    per_file_dir = skill_dir / "per-file"
    per_file_dir.mkdir(parents=True)

    (per_file_dir / "summary.md").write_text("# summary\n", encoding="utf-8")
    (per_file_dir / "foo_session.md").write_text("# session\n", encoding="utf-8")
    (per_file_dir / "service-a.md").write_text(
        "\n".join(
            [
                "# src/service_a.py",
                "- [WARN] naming issue (already flagged by static analysis)",
                "- [CRITICAL] null dereference risk",
            ]
        )
        + "\n",
        encoding="utf-8",
    )

    testsuite = review_to_junit._build_testsuite(skill_dir, "pr-review", "pr-review-code")

    assert testsuite.get("tests") == "1"
    assert testsuite.get("failures") == "1"
    testcase_nodes = list(testsuite.findall("testcase"))
    assert len(testcase_nodes) == 1