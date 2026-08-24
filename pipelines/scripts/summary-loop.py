"""Summary generation loop.

Generates a PR summary then scores it, iterating until the score meets the pass
threshold or the iteration cap is reached.

Usage:
    python summary-loop.py <pr-number> <batch-staging-dir> <skill-output-dir>

Arguments:
    pr-number           PR number being reviewed.
    batch-staging-dir   batch_000 dir for pr-review-summary
                        (written by git-diff-prep.py).
    skill-output-dir    Skill output dir: <pipeline-output>/pr-review-summary.

Same call signature as copilot-dispatch.sh — Groovy calls it the same way,
just substituting this script for copilot-dispatch.sh for pr-review-summary.

Environment:
    SUMMARY_PASS_SCORE      Minimum score to accept (default: 15, max: 23).
    SUMMARY_MAX_ITERATIONS  Maximum generation attempts (default: 5).
    All other env vars forwarded to copilot-dispatch.sh
    (GITHUB_TOKEN, COPILOT_PIPELINE, etc.).

Loop behaviour:
    1. copilot-dispatch.sh --summary  writes <pipeline-output>/summary.md
    2. copilot-dispatch.sh --score    writes <pipeline-output>/score.md
    3. Parse SCORE: N from score.md
    4. If score >= threshold: done (exit 0)
    5. Copy score.md to <batch-staging-dir>/score_feedback.md for next iteration
    6. Repeat from step 1 (up to SUMMARY_MAX_ITERATIONS total attempts)

Iteration artefacts are archived to <skill-output-dir>/logs/:
    summary_iter_N.md, score_iter_N.md
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
    """Extract the integer score from score.md.

    Args:
        score_file: Path to the score.md file written by copilot-dispatch.

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


def run_summary_loop(
    pr_number: str,
    staging_dir: Path,
    output_dir: Path,
    pass_score: int,
    max_iter: int,
) -> int:
    """Run the generate → score → iterate loop.

    Args:
        pr_number: PR number being reviewed.
        staging_dir: batch_000 dir for pr-review-summary.
        output_dir: Skill output dir (<pipeline-output>/pr-review-summary).
        pass_score: Minimum score to accept.
        max_iter: Maximum number of attempts.

    Returns:
        Exit code: 0 on success, 1 on unrecoverable error.
    """
    pipeline_output_dir = output_dir.parent
    score_file = pipeline_output_dir / "score.md"
    logs_dir = output_dir / "logs"
    logs_dir.mkdir(parents=True, exist_ok=True)

    threshold_str = f"pass>={pass_score}, max={max_iter} iterations"
    log_header(f"Summary loop: PR #{pr_number} ({threshold_str})")

    score = 0
    iteration = 1

    while iteration <= max_iter:
        log_info(f"[summary-loop] Iteration {iteration}/{max_iter}")

        os.environ["SUMMARY_ITER"] = str(iteration)

        # Step 1: Generate summary
        rc = _run_dispatch([pr_number, str(staging_dir), str(output_dir), "--summary"])
        if rc != 0:
            log_error(f"[summary-loop] copilot-dispatch --summary failed (exit {rc})")
            return 1

        # Step 2: Score the summary
        rc = _run_dispatch([pr_number, str(staging_dir), str(output_dir), "--score"])
        if rc != 0:
            log_error(f"[summary-loop] copilot-dispatch --score failed (exit {rc})")
            return 1

        # Step 3: Parse score
        if not score_file.exists():
            log_error("[summary-loop] score.md not found after score pass.")
            return 1

        parsed = _parse_score(score_file)
        if parsed is None:
            log_error("[summary-loop] Could not parse SCORE from score.md.")
            return 1
        score = parsed

        log_info(f"[summary-loop] Iteration {iteration} score: {score}/19")

        # Archive iteration artefacts
        summary_src = pipeline_output_dir / "summary.md"
        if summary_src.exists():
            shutil.copy2(summary_src, logs_dir / f"summary_iter_{iteration}.md")
        if score_file.exists():
            shutil.copy2(score_file, logs_dir / f"score_iter_{iteration}.md")

        # Step 4: Accept if threshold met
        if score >= pass_score:
            log_info(f"[summary-loop] Score {score} >= {pass_score}. Accepting.")
            break

        # Step 5: Feed score back for next iteration (skip on final attempt)
        if iteration < max_iter:
            log_info(f"[summary-loop] Score {score} < {pass_score}. Feeding back.")
            shutil.copy2(score_file, staging_dir / "score_feedback.md")
        else:
            log_info(f"[summary-loop] Max iterations reached. Best: {score}/19.")

        iteration += 1

    log_info(f"[summary-loop] Done. Final score: {score}/19")

    record = {
        "timestamp": datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "pr": pr_number,
        "skill": "pr-review-summary",
        "mode": "summary-loop",
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
    """Parse arguments and run the summary loop."""
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

    pass_score = int(os.environ.get("SUMMARY_PASS_SCORE", "15"))
    max_iter = int(os.environ.get("SUMMARY_MAX_ITERATIONS", "5"))

    sys.exit(run_summary_loop(pr_number, staging_dir, output_dir, pass_score, max_iter))


if __name__ == "__main__":
    main()
