"""Technical deep review — focused second pass on files cited in tech-review.md.

Parses tech-review.md, extracts files cited in **Does:** fields, stages one input file
per cited source file (risks + full file content), dispatches one copilot call per file
in parallel, and aggregates outputs into tech-review-deep.md.

Usage:
    python tech-review-deep.py <pr-number> <tech-review-md> <workspace-root> <staging-base> <output-dir>

Arguments:
    pr-number         PR number being reviewed.
    tech-review-md    Absolute path to the finalised tech-review.md.
    workspace-root    Absolute path to the workspace root (app repo checkout).
    staging-base      Base staging directory (<pipeline-output>-staging-<pipeline>).
    output-dir        Skill output dir (<pipeline-output>/pr-review-technical-deep).

Environment:
    TECH_DEEP_CHUNK_LINES  Max lines per full-file chunk (default: 400).
    TECH_DEEP_CONCURRENCY  Max parallel copilot calls (default: 6).
    All env vars forwarded to copilot-dispatch.sh
    (GITHUB_TOKEN, COPILOT_PIPELINE, COPILOT_MODEL, etc.).

Behaviour:
    1. Parse tech-review.md — extract file paths from **Does:** fields.
    2. For each cited file:
       a. Read full file content from <workspace-root>/<file>.
       b. Chunk at TECH_DEEP_CHUNK_LINES if needed.
       c. Write staging input: risks block + full file content + diff marker.
       d. Write a minimal manifest.json pointing to that input file.
    3. Dispatch one copilot call per file (parallel, capped at TECH_DEEP_CONCURRENCY).
       Skill: pr-review-technical-deep. Mode: technical-deep (via --technical-deep).
    4. Aggregate per-file outputs into tech-review-deep.md in output-dir.
"""

from __future__ import annotations

import concurrent.futures
import inspect
import json
import os
import re
import subprocess
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

_SCRIPT_DIR = Path(__file__).parent
_DISPATCH = _SCRIPT_DIR / "copilot-dispatch.sh"

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

_LOG_FMT = "[{ts}] [{level}] {msg}"


def _ts() -> str:
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def log_info(msg: str) -> None:
    print(_LOG_FMT.format(ts=_ts(), level="INFO", msg=msg))


def log_warn(msg: str) -> None:
    print(_LOG_FMT.format(ts=_ts(), level="WARN", msg=msg))


def log_error(msg: str) -> None:
    print(_LOG_FMT.format(ts=_ts(), level="ERROR", msg=msg), file=sys.stderr)


def log_header(msg: str) -> None:
    log_info(f"========== {msg} ==========")


# ---------------------------------------------------------------------------
# Parsing: extract file paths and their associated risk blocks from tech-review.md
# ---------------------------------------------------------------------------

# Matches the file path in a **Does:** field, e.g.:
#   **Does:** `src/foo/bar.py` `MyClass.__init__` does something
_DOES_RE = re.compile(r"^\*\*Does:\*\*\s+`([^`]+\.py[^`]*)`", re.MULTILINE)

# An H3 heading marks the start of a risk entry
_H3_RE = re.compile(r"^### .+", re.MULTILINE)

# Horizontal rule separating entries
_HR_RE = re.compile(r"^---\s*$", re.MULTILINE)


def _file_slug(file_path: str) -> str:
    """Convert a repo-relative file path to a safe filename stem."""
    return re.sub(r"[^a-zA-Z0-9]", "-", file_path).strip("-")


def parse_risks_by_file(tech_review: str) -> dict[str, list[str]]:
    """Return a mapping of file path → list of risk block strings.

    Each risk block is the full H3 + body text for one entry. Blocks are
    split on horizontal rules. Only blocks containing a **Does:** line that
    names a .py (or similar source) file are included.

    Args:
        tech_review: Full text of tech-review.md.

    Returns:
        Dict mapping repo-relative file path to list of raw risk block strings.
    """
    # Split on horizontal rules to isolate individual risk entries.
    # Include the section header (## ⚠️ Correctness Risks) only up to the
    # first risk entry — entries are everything that starts with ###.
    sections = _HR_RE.split(tech_review)
    risks_by_file: dict[str, list[str]] = {}

    for section in sections:
        section = section.strip()
        if not _H3_RE.match(section):
            continue
        match = _DOES_RE.search(section)
        if not match:
            continue
        file_path = match.group(1).strip()
        risks_by_file.setdefault(file_path, []).append(section)

    return risks_by_file


# ---------------------------------------------------------------------------
# Staging: write one input file per cited source file
# ---------------------------------------------------------------------------

def _chunk_lines(lines: list[str], chunk_size: int) -> list[list[str]]:
    return [lines[i : i + chunk_size] for i in range(0, len(lines), chunk_size)]


def _load_technical_agent_contexts(staging_base: Path) -> dict[str, dict[str, Any]]:
    """Load per-file agent_context entries staged for pr-review-technical."""
    manifest = staging_base / "pr-review-technical" / "batch_000" / "manifest.json"
    if not manifest.exists():
        return {}

    try:
        data = json.loads(manifest.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}

    reviewable = data.get("reviewable")
    if not isinstance(reviewable, list):
        return {}

    by_file: dict[str, dict[str, Any]] = {}
    for entry in reviewable:
        if not isinstance(entry, dict):
            continue
        file_path = entry.get("file")
        context = entry.get("agent_context")
        if isinstance(file_path, str) and isinstance(context, dict) and context:
            by_file[file_path] = context
    return by_file


def stage_file(
    file_path: str,
    risks: list[str],
    workspace_root: Path,
    staging_dir: Path,
    chunk_lines: int,
    pr_number: str,
    skill_dir: Path,
    agent_context: dict[str, Any] | None = None,
) -> list[Path]:
    """Write staging input file(s) for one cited source file.

    Returns a list of staging manifest directories (one per chunk).

    Args:
        file_path: Repo-relative path to the source file.
        risks: List of raw risk block strings for this file.
        workspace_root: Root of the app repo checkout.
        staging_dir: Base staging directory for this deep pass.
        chunk_lines: Max lines per full-file chunk.
        pr_number: PR number (used in manifest).
        skill_dir: Absolute path to pr-review-technical-deep skill directory.

    Returns:
        List of batch staging directories, one per chunk.
    """
    full_path = workspace_root / file_path
    if not full_path.exists():
        log_warn(f"[tech-review-deep] Cited file not found in workspace: {file_path} — skipping")
        return []

    file_lines = full_path.read_text(encoding="utf-8", errors="replace").splitlines()
    chunks = _chunk_lines(file_lines, chunk_lines)
    slug = _file_slug(file_path)
    risks_block = "\n\n---\n\n".join(risks)

    batch_dirs: list[Path] = []

    for idx, chunk in enumerate(chunks, start=1):
        total = len(chunks)
        chunk_label = (
            f"=== FULL FILE: {file_path} ==="
            if total == 1
            else f"=== FULL FILE CHUNK {idx} of {total}: {file_path} ==="
        )
        content = "\n".join([
            "=== RISKS FROM TECH-REVIEW ===",
            "",
            risks_block,
            "",
            chunk_label,
            "",
            *chunk,
        ])

        chunk_suffix = "" if total == 1 else f"-chunk{idx:03d}"
        input_filename = f"{slug}{chunk_suffix}.txt"

        batch_dir = staging_dir / f"batch_{slug}{chunk_suffix}"
        batch_dir.mkdir(parents=True, exist_ok=True)

        input_file = batch_dir / input_filename
        input_file.write_text(content, encoding="utf-8")

        # Minimal manifest for the deep skill — one reviewable entry per chunk.
        manifest = {
            "base_branch": "",
            "refspec": "",
            "skill_dir": "pr-review-technical-deep",
            "review_agents": {"pr-review-technical-deep": [file_path]},
            "reviewable": [
                {
                    "file": file_path,
                    "slug": slug,
                    "mode": "full_and_diff" if total == 1 else "diff_chunk",
                    "chunk": idx if total > 1 else None,
                    "total_chunks": total if total > 1 else None,
                    "input_file": input_filename,
                    "agent": "pr-review-technical-deep",
                }
            ],
            "excluded": [],
        }
        if agent_context:
            manifest["reviewable"][0]["agent_context"] = agent_context
        (batch_dir / "manifest.json").write_text(
            json.dumps(manifest, indent=2), encoding="utf-8"
        )

        # Copy SKILL.md and templates into batch dir so dispatch can find them.
        import shutil
        shutil.copy2(skill_dir / "SKILL.md", batch_dir / "SKILL.md")
        templates_src = skill_dir / "templates"
        if templates_src.exists():
            shutil.copytree(templates_src, batch_dir / "templates", dirs_exist_ok=True)

        # Write Sydney timestamp.
        import subprocess as _sp
        ts = _sp.check_output(
            ["date", "+%Y-%m-%dT%H:%M:%S%:z"],
            env={**os.environ, "TZ": "Australia/Sydney"},
            text=True,
        ).strip()
        (batch_dir / "review_timestamp.txt").write_text(ts, encoding="utf-8")

        batch_dirs.append(batch_dir)

    return batch_dirs


# ---------------------------------------------------------------------------
# Dispatch: one copilot call per batch dir
# ---------------------------------------------------------------------------

def dispatch_batch(
    pr_number: str,
    batch_dir: Path,
    output_dir: Path,
) -> tuple[Path, int]:
    """Run copilot-dispatch.sh --technical-deep for one batch directory.

    Args:
        pr_number: PR number.
        batch_dir: Staging batch directory containing manifest.json + input file.
        output_dir: Skill output directory.

    Returns:
        Tuple of (batch_dir, exit_code).
    """
    result = subprocess.run(
        ["bash", str(_DISPATCH), pr_number, str(batch_dir), str(output_dir), "--technical-deep"],
        check=False,
    )
    return batch_dir, result.returncode


# ---------------------------------------------------------------------------
# Aggregation: collect per-file outputs into tech-review-deep.md
# ---------------------------------------------------------------------------

def aggregate_outputs(
    risks_by_file: dict[str, list[str]],
    output_dir: Path,
    pr_number: str,
) -> Path:
    """Concatenate per-file deep review outputs into tech-review-deep.md.

    Args:
        risks_by_file: File paths in the order they were cited.
        output_dir: Skill output directory containing per-file <slug>.md reports.
        pr_number: PR number for the header.

    Returns:
        Path to the written tech-review-deep.md.
    """
    lines: list[str] = [
        f"# PR #{pr_number} — Technical Deep Review",
        "",
        f"_Generated: {datetime.now(UTC).strftime('%Y-%m-%dT%H:%M:%SZ')}_",
        "",
        "---",
        "",
    ]

    for file_path in risks_by_file:
        slug = _file_slug(file_path)
        report_path = output_dir / f"{slug}.md"
        if report_path.exists():
            lines.append(report_path.read_text(encoding="utf-8").strip())
        else:
            lines.append(f"## {file_path}\n\n_No deep review output produced for this file._")
        lines.append("\n---\n")

    output_file = output_dir / "tech-review-deep.md"
    output_file.write_text("\n".join(lines), encoding="utf-8")
    log_info(f"[tech-review-deep] Wrote {output_file}")
    return output_file


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def run(
    pr_number: str,
    tech_review_path: Path,
    workspace_root: Path,
    staging_base: Path,
    output_dir: Path,
    chunk_lines: int,
    concurrency: int,
) -> int:
    """Run the full deep review pass.

    Args:
        pr_number: PR number.
        tech_review_path: Path to the finalised tech-review.md.
        workspace_root: App repo workspace root.
        staging_base: Base staging directory.
        output_dir: Output directory for this skill.
        chunk_lines: Max lines per full-file chunk.
        concurrency: Max parallel copilot calls.

    Returns:
        Exit code: 0 on success, 1 on error.
    """
    log_header(f"Tech-review deep pass: PR #{pr_number}")

    if not tech_review_path.exists():
        log_error(f"tech-review.md not found: {tech_review_path}")
        return 1

    tech_review_text = tech_review_path.read_text(encoding="utf-8")
    risks_by_file = parse_risks_by_file(tech_review_text)

    if not risks_by_file:
        log_info("[tech-review-deep] No file citations found in tech-review.md — nothing to do.")
        output_dir.mkdir(parents=True, exist_ok=True)
        (output_dir / "tech-review-deep.md").write_text(
            f"# PR #{pr_number} — Technical Deep Review\n\n"
            "_No correctness risks were cited in the technical review._\n",
            encoding="utf-8",
        )
        return 0

    log_info(f"[tech-review-deep] Cited files: {list(risks_by_file.keys())}")

    # Locate skill directory via same resolution as copilot-dispatch.sh.
    skills_base = _SCRIPT_DIR.parent.parent / "agents" / "skills"
    skill_dir = skills_base / "pr-review-technical-deep"
    if not skill_dir.exists():
        log_error(f"Skill directory not found: {skill_dir}")
        return 1

    staging_dir = staging_base / "pr-review-technical-deep"
    staging_dir.mkdir(parents=True, exist_ok=True)
    output_dir.mkdir(parents=True, exist_ok=True)
    context_by_file = _load_technical_agent_contexts(staging_base)

    # Stage all files.
    all_batch_dirs: list[Path] = []
    for file_path, risks in risks_by_file.items():
        batch_dirs = stage_file(
            file_path,
            risks,
            workspace_root,
            staging_dir,
            chunk_lines,
            pr_number,
            skill_dir,
            context_by_file.get(file_path),
        )
        all_batch_dirs.extend(batch_dirs)

    if not all_batch_dirs:
        log_warn("[tech-review-deep] No stageable files found (all cited files missing from workspace).")
        return 0

    log_info(f"[tech-review-deep] Dispatching {len(all_batch_dirs)} batch(es) with concurrency={concurrency}")

    # Dispatch in parallel.
    failures: list[Path] = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as pool:
        futures = {
            pool.submit(dispatch_batch, pr_number, bd, output_dir): bd
            for bd in all_batch_dirs
        }
        for future in concurrent.futures.as_completed(futures):
            batch_dir, rc = future.result()
            if rc != 0:
                log_error(f"[tech-review-deep] Batch failed (exit {rc}): {batch_dir}")
                failures.append(batch_dir)
            else:
                log_info(f"[tech-review-deep] Batch done: {batch_dir.name}")

    if failures:
        log_error(f"[tech-review-deep] {len(failures)} batch(es) failed — partial output written.")

    aggregate_outputs(risks_by_file, output_dir, pr_number)
    return 1 if failures else 0


def main() -> None:
    """Parse arguments and run the deep review pass."""
    if len(sys.argv) != 6:
        script_name = Path(sys.argv[0]).name
        msg = inspect.cleandoc(
            f"""
            Usage: {script_name} <pr-number> <tech-review-md> <workspace-root> <staging-base> <output-dir>
            """
        )
        print(msg, file=sys.stderr)
        sys.exit(1)

    pr_number = sys.argv[1]
    tech_review_path = Path(sys.argv[2]).resolve()
    workspace_root = Path(sys.argv[3]).resolve()
    staging_base = Path(sys.argv[4]).resolve()
    output_dir = Path(sys.argv[5]).resolve()

    chunk_lines = int(os.environ.get("TECH_DEEP_CHUNK_LINES", "400"))
    concurrency = int(os.environ.get("TECH_DEEP_CONCURRENCY", "6"))

    sys.exit(run(pr_number, tech_review_path, workspace_root, staging_base, output_dir, chunk_lines, concurrency))


if __name__ == "__main__":
    main()
