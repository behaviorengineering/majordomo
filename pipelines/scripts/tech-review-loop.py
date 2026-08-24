"""Technical review generation loop.

Generates a PR technical review then scores it, iterating until the score meets the
pass threshold or the iteration cap is reached.

Usage:
    python tech-review-loop.py <pr-number> <batch-staging-dir> <skill-output-dir>

Arguments:
    pr-number           PR number being reviewed.
    batch-staging-dir   batch_000 dir for pr-review-technical
                        (written by git-diff-prep.py).
    skill-output-dir    Skill output dir: <pipeline-output>/pr-review-technical.

Same call signature as copilot-dispatch.sh — Groovy calls it the same way,
just substituting this script for copilot-dispatch.sh for pr-review-technical.

Environment:
    TECH_PASS_SCORE      Minimum score to accept (default: 11, max: 14).
    TECH_MAX_ITERATIONS  Maximum generation attempts (default: 3).
    All other env vars forwarded to copilot-dispatch.sh
    (GITHUB_TOKEN, COPILOT_PIPELINE, etc.).

Loop behaviour:
    1. copilot-dispatch.sh --technical  writes <pipeline-output>/tech-review.md
    2. copilot-dispatch.sh --tech-score writes <pipeline-output>/tech-score.md
    3. Parse SCORE: N from tech-score.md
    4. If score >= threshold: done (exit 0)
    5. Copy tech-score.md to <batch-staging-dir>/pr-review-technical/tech_feedback.md
       for the next iteration (read by the technical skill's §Feedback Integration)
    6. Repeat from step 1 (up to TECH_MAX_ITERATIONS total attempts)

Iteration artefacts are archived to <skill-output-dir>/logs/:
    tech_review_iter_N.md, tech_score_iter_N.md
"""

from __future__ import annotations

import inspect
import json
import os
import re
import shutil
import subprocess
import sys
from datetime import UTC, datetime
from pathlib import Path

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

_SCRIPT_DIR = Path(__file__).parent
_DISPATCH = _SCRIPT_DIR / "copilot-dispatch.sh"

# ---------------------------------------------------------------------------
# Logging helpers — mirror the bash common.sh style for consistent CI output
# ---------------------------------------------------------------------------

_LOG_FMT = "[{ts}] [{level}] {msg}"


def _ts() -> str:
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def log_info(msg: str) -> None:
    """Log an INFO message to stdout."""
    print(_LOG_FMT.format(ts=_ts(), level="INFO", msg=msg))


def log_error(msg: str) -> None:
    """Log an ERROR message to stderr."""
    print(_LOG_FMT.format(ts=_ts(), level="ERROR", msg=msg), file=sys.stderr)


def log_header(msg: str) -> None:
    """Log a section header to stdout."""
    log_info(f"========== {msg} ==========")


# ---------------------------------------------------------------------------
# Subprocess helpers
# ---------------------------------------------------------------------------


def _run_dispatch(args: list[str]) -> int:
    """Run copilot-dispatch.sh with the given arguments and return the exit code.

    Args:
        args: Arguments to pass after the script path.

    Returns:
        Exit code from the subprocess.
    """
    result = subprocess.run(
        ["bash", str(_DISPATCH), *args],
        check=False,
    )
    return result.returncode


# ---------------------------------------------------------------------------
# Score parsing
# ---------------------------------------------------------------------------

_SCORE_RE = re.compile(r"^SCORE:\s*(\d+)", re.MULTILINE)


def _parse_score(score_file: Path) -> int | None:
    """Extract the integer score from tech-score.md.

    Args:
        score_file: Path to the tech-score.md file written by copilot-dispatch.

    Returns:
        Parsed score as an integer, or None if the line is absent or malformed.
    """
    text = score_file.read_text(encoding="utf-8")
    match = _SCORE_RE.search(text)
    if match is None:
        return None
    return int(match.group(1))


# ---------------------------------------------------------------------------
# Main loop
# ---------------------------------------------------------------------------


def run_tech_review_loop(
    pr_number: str,
    staging_dir: Path,
    output_dir: Path,
    pass_score: int,
    max_iter: int,
) -> int:
    """Run the generate → score → iterate loop.

    Args:
        pr_number: PR number being reviewed.
        staging_dir: batch_000 dir for pr-review-technical.
        output_dir: Skill output dir (<pipeline-output>/pr-review-technical).
        pass_score: Minimum score to accept.
        max_iter: Maximum number of attempts.

    Returns:
        Exit code: 0 on success, 1 on unrecoverable error.
    """
    pipeline_output_dir = output_dir.parent
    score_file = pipeline_output_dir / "tech-score.md"
    tech_review_file = pipeline_output_dir / "tech-review.md"
    # Feedback is read by the technical skill from the skill's staging subdirectory
    # (the same directory that contains manifest.json after staging setup).
    feedback_file = staging_dir / "pr-review-technical" / "tech_feedback.md"
    logs_dir = output_dir / "logs"
    logs_dir.mkdir(parents=True, exist_ok=True)

    threshold_str = f"pass>={pass_score}, max={max_iter} iterations"
    log_header(f"Tech-review loop: PR #{pr_number} ({threshold_str})")

    score = 0
    iteration = 1

    while iteration <= max_iter:
        log_info(f"[tech-review-loop] Iteration {iteration}/{max_iter}")

        os.environ["TECH_ITER"] = str(iteration)

        # Step 1: Generate technical review
        rc = _run_dispatch([pr_number, str(staging_dir), str(output_dir), "--technical"])
        if rc != 0:
            log_error(f"[tech-review-loop] copilot-dispatch --technical failed (exit {rc})")
            return 1

        # Step 2: Score the technical review
        rc = _run_dispatch([pr_number, str(staging_dir), str(output_dir), "--tech-score"])
        if rc != 0:
            log_error(f"[tech-review-loop] copilot-dispatch --tech-score failed (exit {rc})")
            return 1

        # Step 3: Parse score
        if not score_file.exists():
            log_error("[tech-review-loop] tech-score.md not found after score pass.")
            return 1

        parsed = _parse_score(score_file)
        if parsed is None:
            log_error("[tech-review-loop] Could not parse SCORE from tech-score.md.")
            return 1
        score = parsed

        log_info(f"[tech-review-loop] Iteration {iteration} score: {score}/14")

        # Archive iteration artefacts
        if tech_review_file.exists():
            shutil.copy2(tech_review_file, logs_dir / f"tech_review_iter_{iteration}.md")
        if score_file.exists():
            shutil.copy2(score_file, logs_dir / f"tech_score_iter_{iteration}.md")

        # Step 4: Accept if threshold met
        if score >= pass_score:
            log_info(f"[tech-review-loop] Score {score} >= {pass_score}. Accepting.")
            break

        # Step 5: Feed score back for next iteration (skip on final attempt)
        if iteration < max_iter:
            log_info(f"[tech-review-loop] Score {score} < {pass_score}. Feeding back.")
            feedback_file.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(score_file, feedback_file)
        else:
            log_info(f"[tech-review-loop] Max iterations reached. Best: {score}/14.")

        iteration += 1

    log_info(f"[tech-review-loop] Done. Final score: {score}/14")

    record = {
        "timestamp": datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "pr": pr_number,
        "skill": "pr-review-technical",
        "mode": "tech-review-loop",
        "iterations": iteration,
        "final_score": score,
        "pass_threshold": pass_score,
        "passed": score >= pass_score,
    }
    metrics_file = output_dir / "metrics.jsonl"
    with metrics_file.open("a", encoding="utf-8") as fh:
        fh.write(json.dumps(record) + "\n")

    return 0


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def main() -> None:
    """Parse arguments and run the tech review loop."""
    if len(sys.argv) != 4:
        script_name = Path(sys.argv[0]).name
        msg = inspect.cleandoc(
            f"""
            Usage: {script_name} <pr-number> <batch-staging-dir> <skill-output-dir>
            """
        )
        print(msg, file=sys.stderr)
        sys.exit(1)

    pr_number = sys.argv[1]
    staging_dir = Path(sys.argv[2]).resolve()
    output_dir = Path(sys.argv[3]).resolve()

    pass_score = int(os.environ.get("TECH_PASS_SCORE", "11"))
    max_iter = int(os.environ.get("TECH_MAX_ITERATIONS", "3"))

    sys.exit(run_tech_review_loop(pr_number, staging_dir, output_dir, pass_score, max_iter))


if __name__ == "__main__":
    main()
