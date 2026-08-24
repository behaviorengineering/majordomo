"""Convert Copilot PR review markdown reports to JUnit XML for Jenkins.

Walks the review output directory produced by copilot-review.groovy:

    <review-output-dir>/
      <pipeline-name>/
        <skill-name>/
          <slug>.md          ← per-file reports  (parsed)
          summary.md         ← skipped
          index.md           ← skipped
          session.md         ← skipped
          review-manifest.json  ← skipped (not .md)

One JUnit XML file is written per pipeline/skill pair:

    <junit-output-dir>/copilot-review-<pipeline>-<skill>.xml

Finding classification → JUnit mapping:
  [CRITICAL] / [WARN] → <failure>   (makes build unstable in Jenkins)
  [INFO]              → <skipped>   (tracked; does not affect build status)
  No findings in file → passing <testcase> with no child element
"""

from __future__ import annotations

import re
import sys
import xml.etree.ElementTree as ET
from datetime import UTC, datetime
from pathlib import Path

_FINDING_RE = re.compile(r"^\s*-\s+\[(CRITICAL|WARN|INFO)\]\s+(.+)$")
_FILE_HEADER_RE = re.compile(r"^#\s+(.+)$")
_META_RE = re.compile(r"^\*\*(.+?):\*\*\s*(.+)$")
_SKIP_FILENAMES: frozenset[str] = frozenset({"summary.md", "index.md", "session.md"})
_SKIP_SUFFIXES: tuple[str, ...] = ("_session.md",)
_NAME_MAX = 120
# Findings that the agent annotated as SA duplicates — the linter already caught them.
# These are filtered out rather than reported, since they add noise without new signal.
_SA_DUPLICATE_RE = re.compile(r"\(already flagged by static analysis\)", re.IGNORECASE)
# XML 1.0 illegal characters: everything below 0x20 except tab (0x09), LF (0x0A), CR (0x0D),
# plus the surrogates range and 0xFFFE/0xFFFF.
_XML_ILLEGAL_RE = re.compile(
    r"[\x00-\x08\x0b\x0c\x0e-\x1f\ud800-\udfff\ufffe\uffff]"
)


def _sanitize(text: str) -> str:
    """Remove characters that are illegal in XML 1.0 element content."""
    return _XML_ILLEGAL_RE.sub("", text)


def _parse_report(
    path: Path,
) -> tuple[str, list[tuple[str, str]], list[tuple[str, str]]]:
    """Return (reviewed_file_path, metadata_pairs, findings) for one slug report."""
    reviewed_file = path.stem
    meta: list[tuple[str, str]] = []
    findings: list[tuple[str, str]] = []

    for line in path.read_text(encoding="utf-8").splitlines():
        if m := _FILE_HEADER_RE.match(line):
            reviewed_file = m.group(1).strip()
        elif m := _META_RE.match(line):
            meta.append((m.group(1).strip(), m.group(2).strip()))
        elif m := _FINDING_RE.match(line):
            findings.append((m.group(1), m.group(2).strip()))

    return reviewed_file, meta, findings


def _build_testsuite(
    skill_dir: Path,
    pipeline_name: str,
    skill_name: str,
) -> ET.Element:
    """Build a JUnit testsuite element from all per-file report *.md files."""
    per_file_dir = skill_dir / "per-file"
    report_files = sorted(
        p for p in per_file_dir.glob("*.md")
        if p.name not in _SKIP_FILENAMES and not p.name.endswith(_SKIP_SUFFIXES)
    ) if per_file_dir.is_dir() else []

    timestamp = datetime.now(UTC).isoformat()
    total = failures = skipped = 0

    testsuite = ET.Element(
        "testsuite",
        {
            "name": f"Copilot PR Review — {pipeline_name}/{skill_name}",
            "tests": "0",
            "failures": "0",
            "errors": "0",
            "skipped": "0",
            "timestamp": timestamp,
        },
    )

    for report_path in report_files:
        reviewed_file, meta, findings = _parse_report(report_path)
        classname = (
            f"copilot_review.{pipeline_name}.{skill_name}."
            + reviewed_file.replace("\\", "/").replace("/", ".")
        )
        meta_block = "\n".join(f"{k}: {v}" for k, v in meta)
        preamble = f"File: {reviewed_file}\n{meta_block}\n".rstrip("\n") + "\n"

        if not findings:
            tc = ET.SubElement(
                testsuite,
                "testcase",
                {"classname": classname, "name": reviewed_file, "time": "0"},
            )
            stdout = ET.SubElement(tc, "system-out")
            stdout.text = _sanitize(preamble + "\nNo issues found.")
            total += 1
            continue

        for level, text in findings:
            if _SA_DUPLICATE_RE.search(text):
                continue  # already reported by the linter — skip to avoid duplicate noise
            name = text if len(text) <= _NAME_MAX else text[:_NAME_MAX - 3] + "..."
            testcase = ET.SubElement(
                testsuite,
                "testcase",
                {"classname": classname, "name": f"[{level}] {name}", "time": "0"},
            )
            stdout = ET.SubElement(testcase, "system-out")
            stdout.text = _sanitize(preamble + f"\n[{level}] {text}")
            if level == "CRITICAL":
                failure = ET.SubElement(
                    testcase, "failure", {"message": _sanitize(text), "type": level}
                )
                failure.text = _sanitize(preamble + f"\n[{level}] {text}")
                failures += 1
            else:  # WARN and INFO → skipped: visible in report, does not affect build status
                skipped_el = ET.SubElement(testcase, "skipped", {"message": _sanitize(text)})
                skipped_el.text = _sanitize(preamble + f"\n[{level}] {text}")
                skipped += 1
            total += 1

    testsuite.set("tests", str(total))
    testsuite.set("failures", str(failures))
    testsuite.set("skipped", str(skipped))

    return testsuite


def convert(review_output_dir: Path, junit_output_dir: Path) -> None:
    """Walk review_output_dir and write one JUnit XML per pipeline/skill pair."""
    if not review_output_dir.is_dir():
        return  # nothing produced yet (e.g. all pipelines skipped — nothing to review)

    junit_output_dir.mkdir(parents=True, exist_ok=True)

    for pipeline_dir in sorted(review_output_dir.iterdir()):
        if not pipeline_dir.is_dir():
            continue
        pipeline_name = pipeline_dir.name

        for skill_dir in sorted(pipeline_dir.iterdir()):
            if not skill_dir.is_dir():
                continue
            skill_name = skill_dir.name

            testsuite = _build_testsuite(skill_dir, pipeline_name, skill_name)

            testsuites = ET.Element("testsuites")
            testsuites.append(testsuite)
            tree = ET.ElementTree(testsuites)
            ET.indent(tree, space="  ")

            out_path = junit_output_dir / (
                f"copilot-review-{pipeline_name}-{skill_name}.xml"
            )
            tree.write(out_path, encoding="utf-8", xml_declaration=True)


_EXPECTED_ARGS = 3

if __name__ == "__main__":
    if len(sys.argv) != _EXPECTED_ARGS:
        print(
            f"Usage: {sys.argv[0]} <review-output-dir> <junit-output-dir>",
            file=sys.stderr,
        )
        sys.exit(1)
    convert(Path(sys.argv[1]), Path(sys.argv[2]))
