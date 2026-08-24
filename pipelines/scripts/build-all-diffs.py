"""Concatenate per-file diffs from a manifest into a single all-diffs.txt.

Each diff is preceded by a ``=== FILE: <path> ===`` header so the agent can
identify which file it belongs to without reading individual input files.

Usage:
    python build-all-diffs.py <manifest-json> <output-file> [--cap <n>]

Arguments:
    manifest-json   Path to the manifest.json written by copilot-dispatch.sh.
                    Must contain a ``reviewable`` array with ``file`` and
                    ``input_file`` fields.
    output-file     Path to write the concatenated diffs to.
    --cap <n>       Truncate each file's diff to at most <n> lines and append
                    a marker for omitted content.  Omit for no cap.

Exit codes:
    0  Output written successfully.
    1  Fatal error (bad arguments, missing manifest, unreadable input file).
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any, TextIO


def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Concatenate per-file diffs into all-diffs.txt."
    )
    parser.add_argument("manifest", type=Path, help="Path to manifest.json.")
    parser.add_argument("output", type=Path, help="Path to write all-diffs.txt.")
    parser.add_argument(
        "--cap",
        type=int,
        default=None,
        metavar="N",
        help="Truncate each diff to at most N lines.",
    )
    return parser.parse_args()


def _load_reviewable(manifest: Path) -> list[dict[str, str]]:
    """Load the reviewable list from a manifest file."""
    try:
        data = json.loads(manifest.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        print(f"ERROR: cannot read manifest {manifest}: {exc}", file=sys.stderr)
        sys.exit(1)
    reviewable = data.get("reviewable")
    if not isinstance(reviewable, list):
        print(f"ERROR: manifest {manifest} has no 'reviewable' array.", file=sys.stderr)
        sys.exit(1)
    return reviewable


def _serialize_agent_context(raw_context: Any) -> str | None:
    """Return compact JSON for a per-file agent_context object, if present."""
    if not isinstance(raw_context, dict) or not raw_context:
        return None
    try:
        return json.dumps(raw_context, sort_keys=True)
    except TypeError:
        return None


def _write_entry(
    out_file: TextIO,
    file: str,
    input_file: Path,
    cap: int | None,
    agent_context: Any,
) -> None:
    """Write one diff entry — header, content, optional truncation marker, blank line."""
    out_file.write(f"=== FILE: {file} ===\n")
    context_json = _serialize_agent_context(agent_context)
    if context_json:
        out_file.write(f"=== AGENT CONTEXT: {context_json} ===\n")

    try:
        lines = input_file.read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError:
        lines = []

    if cap is None or len(lines) <= cap:
        out_file.write("\n".join(lines))
        if lines:
            out_file.write("\n")
    else:
        out_file.write("\n".join(lines[:cap]))
        out_file.write("\n")
        omitted = len(lines) - cap
        out_file.write(f"[... {omitted} lines omitted — diff cap is {cap} lines]\n")

    out_file.write("\n")


def main() -> None:
    """Entry point."""
    args = _parse_args()
    reviewable = _load_reviewable(args.manifest)

    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8") as out:
        for entry in reviewable:
            file = entry.get("file", "")
            input_file = Path(entry.get("input_file", ""))
            _write_entry(out, file, input_file, args.cap, entry.get("agent_context"))


if __name__ == "__main__":
    main()
