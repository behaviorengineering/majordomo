"""Convert a Markdown file to a self-contained HTML page.

Usage:
    python md-to-html.py <input.md> <output.html>

Arguments:
    input.md     Path to the source Markdown file.
    output.html  Path to write the rendered HTML file.

The output is a fully self-contained HTML document with inline CSS — no
external assets — suitable for use as a Jenkins build artifact link.

Exit codes:
    0  Converted successfully.
    1  Fatal error.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import markdown

# ---------------------------------------------------------------------------
# HTML shell
# ---------------------------------------------------------------------------

_HTML_TEMPLATE = """\
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>__TITLE__</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; }
    body {
      font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
      font-size: 14px;
      line-height: 1.6;
      color: #24292f;
      background: #ffffff;
      max-width: 900px;
      margin: 40px auto;
      padding: 0 24px 60px;
    }
    h1, h2, h3, h4, h5, h6 {
      margin-top: 1.5em;
      margin-bottom: 0.5em;
      font-weight: 600;
      line-height: 1.25;
    }
    h1 { font-size: 2em; border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; }
    h2 { font-size: 1.5em; border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; }
    h3 { font-size: 1.25em; }
    p { margin: 0 0 1em; }
    a { color: #0969da; text-decoration: none; }
    a:hover { text-decoration: underline; }
    code {
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      font-size: 85%;
      background: #f6f8fa;
      padding: 0.2em 0.4em;
      border-radius: 6px;
    }
    pre {
      background: #f6f8fa;
      padding: 16px;
      overflow: auto;
      border-radius: 6px;
      line-height: 1.45;
    }
    pre code {
      background: transparent;
      padding: 0;
      font-size: 100%;
    }
    blockquote {
      margin: 0 0 1em;
      padding: 0 1em;
      color: #57606a;
      border-left: 4px solid #d0d7de;
    }
    table {
      border-collapse: collapse;
      width: 100%;
      margin-bottom: 1em;
    }
    th, td {
      border: 1px solid #d0d7de;
      padding: 6px 13px;
      text-align: left;
    }
    tr:nth-child(even) { background: #f6f8fa; }
    hr { border: none; border-top: 1px solid #d0d7de; margin: 1.5em 0; }
    ul, ol { padding-left: 2em; margin: 0 0 1em; }
    li { margin: 0.25em 0; }
  </style>
</head>
<body>
__BODY__
</body>
</html>
"""

# ---------------------------------------------------------------------------
# Markdown extensions to enable
# ---------------------------------------------------------------------------

_EXTENSIONS = [
    "fenced_code",
    "tables",
    "toc",
    "attr_list",
]


def _derive_title(md_path: Path, html_body: str) -> str:
    """Extract the first H1 from rendered HTML, falling back to the file stem.

    Args:
        md_path: Source Markdown path (used for fallback).
        html_body: Rendered HTML body string.

    Returns:
        Page title string.
    """
    match = re.search(r"<h1[^>]*>(.*?)</h1>", html_body, re.IGNORECASE | re.DOTALL)
    if match:
        # Strip any inner HTML tags from the heading text
        raw = re.sub(r"<[^>]+>", "", match.group(1))
        return raw.strip()
    return md_path.stem.replace("-", " ").replace("_", " ").title()


def convert(md_path: Path, html_path: Path) -> None:
    """Convert *md_path* to a self-contained HTML file at *html_path*.

    Args:
        md_path: Path to the source Markdown file.
        html_path: Destination path for the HTML output.

    Raises:
        FileNotFoundError: If *md_path* does not exist.
        OSError: If the output file cannot be written.
    """
    source = md_path.read_text(encoding="utf-8")
    body = markdown.markdown(source, extensions=_EXTENSIONS)
    title = _derive_title(md_path, body)
    page = _HTML_TEMPLATE.replace("__TITLE__", title).replace("__BODY__", body)
    html_path.parent.mkdir(parents=True, exist_ok=True)
    html_path.write_text(page, encoding="utf-8")


def main(argv: list[str]) -> int:
    """Entry point.

    Args:
        argv: Command-line arguments (excluding the script name).

    Returns:
        Exit code: 0 on success, 1 on error.
    """
    if len(argv) != 2:
        print(f"Usage: {Path(__file__).name} <input.md> <output.html>", file=sys.stderr)
        return 1

    md_path = Path(argv[0])
    html_path = Path(argv[1])

    if not md_path.exists():
        print(f"ERROR: input file not found: {md_path}", file=sys.stderr)
        return 1

    convert(md_path, html_path)
    print(f"Converted: {md_path} → {html_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
