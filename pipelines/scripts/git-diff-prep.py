"""Prepare a git diff staging directory for Copilot review.

Computes which files changed in a PR, applies exclusion filters, fetches full
file content and diffs from git, then writes a manifest and input files to a
staging directory.  The staging directory is the sole output — no Copilot or
network calls are made here.

Usage:
    python git-diff-prep.py <base-branch> <staging-dir> [--routing <path>] [--agent-context <path>]

Arguments:
    base-branch   Git branch to diff against (e.g. master)
    staging-dir   Directory to write staging files into
    --routing     Optional path to a routing JSON file produced by the pipeline
                  from .majordomo-config.groovy. When absent, built-in
                  tier-keyword defaults are used.
    --agent-context Optional path to an agent context JSON file produced by the
                  pipeline from .majordomo-config.groovy.

Exit codes:
    0  Staging complete — at least one reviewable task written to manifest.json
    1  Fatal error (bad arguments, git failure)
    2  Nothing to review (no changed files, or all changes excluded) — caller should skip review

Output structure:
    <staging-dir>/manifest.json   — ordered list of review tasks
    <staging-dir>/<slug>.txt      — content fed to copilot for that task

manifest.json schema:
    {
        "base_branch": str,
        "refspec": str,
        "review_agents": {
            "pr-review-code": [str, ...],  # files routed to the code skill
            "pr-review-conf": [str, ...],  # files routed to the config skill
            "pr-review-docs": [str, ...]   # files routed to the docs skill
        },
        "reviewable": [
            {
                "file": str,           # relative repo path
                "slug": str,           # safe filename stem
                "mode": str,           # "full_and_diff" | "diff_only" | "diff_chunk"
                "chunk": int | null,   # 1-based, null unless mode == "diff_chunk"
                "total_chunks": int | null,
                "input_file": str      # basename of the .txt file in staging-dir
                "agent": str           # which agent this task is routed to
            },
            ...
        ],
        "excluded": [str, ...]
    }
"""

from __future__ import annotations

import fnmatch
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

# dep_clusters and doc_clusters live alongside this script — insert the scripts
# directory so both modules are importable when running directly.
sys.path.insert(0, str(Path(__file__).parent))
from dep_clusters import (
    cluster_aware_batches,
    cluster_files,
    reverse_deps,
)
from doc_clusters import (
    build_corpus_index,
    cluster_docs,
    reverse_links,
)
from doc_clusters import (
    cluster_aware_batches as doc_cluster_aware_batches,
)

# ---------------------------------------------------------------------------
# Limits
# ---------------------------------------------------------------------------

MAX_COMBINED_LINES: int = 500  # full file + diff — exceeded → diff-only
MAX_DIFF_LINES: int = 300  # diff-only — exceeded → chunked
GIT_TIMEOUT: int = 30  # seconds before a git subprocess is killed
MAX_STAGE_FILENAME_BYTES: int = 240  # Keep margin below typical 255-byte NAME_MAX

# Task modes written into manifest task dicts
_MODE_FULL_AND_DIFF: str = "full_and_diff"
_MODE_DIFF_ONLY: str = "diff_only"
_MODE_DIFF_CHUNK: str = "diff_chunk"

# Cross-skill batches (summary / technical / blast-radius) always use batch_000
_CROSS_SKILL_BATCH_DIR: str = "batch_000"
_CROSS_SKILL_BATCH_NUM: str = "000"

# ---------------------------------------------------------------------------
# Routing — tier-based agent classification
# ---------------------------------------------------------------------------

# Default routing — explicit code extensions only.
# Files not matching any pattern are excluded (not routed to any agent).
# To review docs/config files, configure explicit routing in .majordomo-config.groovy:
#
#   routing: [
#       'pr-review-docs': ['**/*.md'],
#       'pr-review-conf': ['**/*.yml', 'docs/**'],
#       'pr-review-code': ['src/**', 'lib/**'],
#   ]
DEFAULT_ROUTING: list[tuple[str, list[str]]] = [
    (
        "pr-review-docs",
        [
            "**/*.md",
            "**/*.rst",
        ],
    ),
    (
        "pr-review-conf",
        [
            "**/*.yml",
            "**/*.yaml",
            "**/*.toml",
            "**/*.json",
            "**/*.ini",
            "**/*.cfg",
            "**/*.env",
            "**/*.xml",
        ],
    ),
    (
        "pr-review-code",
        [
            # Python
            "**/*.py",
            # JavaScript / TypeScript
            "**/*.js",
            "**/*.jsx",
            "**/*.ts",
            "**/*.tsx",
            "**/*.mjs",
            "**/*.cjs",
            # JVM
            "**/*.java",
            "**/*.kt",
            "**/*.kts",
            "**/*.groovy",
            "**/*.scala",
            # C / C++ / C#
            "**/*.c",
            "**/*.h",
            "**/*.cpp",
            "**/*.cc",
            "**/*.cxx",
            "**/*.hpp",
            "**/*.cs",
            # Go / Rust / Swift / Kotlin
            "**/*.go",
            "**/*.rs",
            "**/*.swift",
            # Ruby / PHP / Perl
            "**/*.rb",
            "**/*.php",
            "**/*.pl",
            "**/*.pm",
            # Shell
            "**/*.sh",
            "**/*.bash",
            # Jenkins
            "**/*.Jenkinsfile",
            "**/Jenkinsfile",
            # PowerShell
            "**/*.ps1",
            "**/*.psm1",
            "**/*.psd1",
            # Templates / views
            "**/*.html",
            "**/*.jinja",
            "**/*.jinja2",
            "**/*.j2",
            # Styles
            "**/*.css",
            "**/*.scss",
            "**/*.sass",
            "**/*.less",
            # Terraform / HCL
            "**/*.tf",
            "**/*.hcl",
            # SQL
            "**/*.sql",
            # Dockerfile
            "**/Dockerfile",
            "**/Dockerfile.*",
            # Make
            "**/Makefile",
            "**/makefile",
            "**/*.mk",
            # Notebooks
            "**/*.ipynb",
            # API / schema
            "**/*.proto",
            "**/*.graphql",
            "**/*.gql",
            # Other languages
            "**/*.dart",
            "**/*.ex",
            "**/*.exs",
            "**/*.erl",
            "**/*.hrl",
            "**/*.hs",
            "**/*.lua",
            "**/*.r",
            "**/*.R",
            "**/*.zig",
            "**/*.nim",
        ],
    ),
]


def load_routing(
    routing_path: Path | None,
) -> tuple[list[tuple[str, list[str]]], dict[str, str]]:
    """Load routing rules from a JSON file, falling back to DEFAULT_ROUTING.

    Supports two value formats per skill entry:

    Simple (globs only)::

        {
            "pr-review-docs": ["**/*.md", "**/*.rst"],
            "pr-review-code": ["**"]
        }

    Extended (globs + optional persona)::

        {
            "pr-review-docs": {
                "globs":   ["**/*.md", "**/*.rst"],
                "persona": ".majordomo/personas/doc-reviewer.md"
            },
            "pr-review-code": ["**"]
        }

    The ``persona`` value is a repo-relative path.  It is stored as-is here
    and resolved from disk later in ``_resolve_routing_personas``.

    Order in the file determines routing priority — first match wins.

    Args:
        routing_path: Path to the routing JSON file, or None to use defaults.

    Returns:
        Tuple of:
          - Ordered list of (agent, [glob, ...]) tuples.
          - Dict of {agent: persona_path} for entries that declared a persona.
    """
    if routing_path is None or not routing_path.exists():
        return DEFAULT_ROUTING, {}
    try:
        raw: dict[str, Any] = json.loads(routing_path.read_text(encoding="utf-8"))
        rules: list[tuple[str, list[str]]] = []
        persona_paths: dict[str, str] = {}
        for agent, value in raw.items():
            if isinstance(value, list):
                rules.append((agent, value))
            elif isinstance(value, dict):
                globs = value.get("globs")
                if not isinstance(globs, list):
                    log("ERROR", f"routing['{agent}'].globs must be a list")
                    sys.exit(1)
                rules.append((agent, globs))
                persona = value.get("persona")
                if persona:
                    if not isinstance(persona, str):
                        log("ERROR", f"routing['{agent}'].persona must be a string path")
                        sys.exit(1)
                    persona_paths[agent] = persona
            else:
                log("ERROR", f"routing['{agent}'] must be a list of globs or a map with 'globs'")
                sys.exit(1)
        log("INFO", f"Loaded routing config: {routing_path}")
        return rules, persona_paths
    except SystemExit:
        raise
    except Exception as exc:
        log("WARN", f"Failed to load routing config ({exc}) — using defaults")
        return DEFAULT_ROUTING, {}


def _resolve_routing_personas(
    persona_paths: dict[str, str],
    repo_root: Path,
) -> dict[str, str]:
    """Resolve routing persona file paths to their text content.

    Args:
        persona_paths: Dict of {agent: repo-relative path} from load_routing.
        repo_root: Repository root directory.

    Returns:
        Dict of {agent: persona_text}.  Missing or empty files fail fast.
    """
    resolved: dict[str, str] = {}
    for agent, rel_path in persona_paths.items():
        rel_path = rel_path.strip()
        if not rel_path:
            log("ERROR", f"routing['{agent}'].persona has an empty path")
            sys.exit(1)
        full_path = (repo_root / rel_path).resolve()
        if not full_path.exists() or not full_path.is_file():
            log("ERROR", f"routing['{agent}'].persona file not found: {rel_path}")
            sys.exit(1)
        try:
            text = full_path.read_text(encoding="utf-8").strip()
        except OSError as exc:
            log("ERROR", f"routing['{agent}'].persona failed reading {rel_path}: {exc}")
            sys.exit(1)
        if not text:
            log("ERROR", f"routing['{agent}'].persona file is empty: {rel_path}")
            sys.exit(1)
        resolved[agent] = text
        log("INFO", f"  Loaded persona for {agent}: {rel_path}")
    return resolved


def classify_file(file: str, routing: list[tuple[str, list[str]]]) -> str | None:
    """Return the agent name for a file path based on routing rules.

    First matching glob pattern wins. Returns None if no rule matches
    (caller should treat the file as excluded).

    Args:
        file: Relative repo path.
        routing: Ordered list of (agent, [glob, ...]) tuples.

    Returns:
        Agent name string, or None if no rule matched.
    """
    for agent, patterns in routing:
        if any(fnmatch.fnmatch(file, pat) for pat in patterns):
            return agent
    return None  # no rule matched — not a reviewable file type


def load_agent_context_config(agent_context_path: Path | None) -> dict[str, Any]:
    """Load agent context configuration from JSON.

    Supports both legacy flat context and scoped context forms.

    Legacy form:
        {
            "techStack": ["python"],
            "reviewFocus": ["security"],
            "customRules": ["..."]
        }

    Scoped form:
        {
            "global": {...},
            "scoped": {
                "services/api/**": {...},
                "docs/**": {...}
            }
        }

    Args:
        agent_context_path: Path to optional context JSON file.

    Returns:
        Normalized context config with keys: global, scoped.
    """
    if agent_context_path is None or not agent_context_path.exists():
        return {"global": {}, "scoped": {}}

    try:
        raw: dict[str, Any] = json.loads(agent_context_path.read_text(encoding="utf-8"))
    except Exception as exc:
        log("ERROR", f"Failed to load agent context config ({exc})")
        sys.exit(1)

    if "global" in raw or "scoped" in raw:
        global_ctx = raw.get("global") or {}
        scoped_ctx = raw.get("scoped") or {}
        if not isinstance(global_ctx, dict) or not isinstance(scoped_ctx, dict):
            log("ERROR", "agentContext must define object values for 'global' and 'scoped'")
            sys.exit(1)
        log("INFO", f"Loaded scoped agent context: {agent_context_path}")
        return {"global": global_ctx, "scoped": scoped_ctx}

    # Backward-compatible flat context config.
    log("INFO", f"Loaded legacy flat agent context: {agent_context_path}")
    return {"global": raw, "scoped": {}}


def _resolve_rules(
    raw_rules: list[Any],
    repo_root: Path,
    source_label: str,
) -> list[str]:
    """Resolve inline and file-backed custom rules into plain text entries.

    Supported rule item formats:
        - "inline rule text"
        - {"file": "relative/path/to/rules.md"}

    File-backed rules are loaded relative to repo_root. Missing files fail fast.

    Args:
        raw_rules: Raw customRules list from context config.
        repo_root: Repository root directory.
        source_label: Label used for error reporting.

    Returns:
        Fully resolved list of rule strings.
    """
    resolved: list[str] = []
    for idx, item in enumerate(raw_rules, start=1):
        if isinstance(item, str):
            resolved.append(item)
            continue

        if isinstance(item, dict) and isinstance(item.get("file"), str):
            rel_path = str(item["file"]).strip()
            if not rel_path:
                log("ERROR", f"{source_label} customRules[{idx}] has empty file path")
                sys.exit(1)
            full_path = (repo_root / rel_path).resolve()
            if not full_path.exists() or not full_path.is_file():
                log(
                    "ERROR",
                    f"{source_label} customRules[{idx}] file not found: {rel_path}",
                )
                sys.exit(1)
            try:
                file_text = full_path.read_text(encoding="utf-8").strip()
            except OSError as exc:
                log(
                    "ERROR",
                    f"{source_label} customRules[{idx}] failed reading {rel_path}: {exc}",
                )
                sys.exit(1)
            if not file_text:
                log("ERROR", f"{source_label} customRules[{idx}] file is empty: {rel_path}")
                sys.exit(1)
            resolved.append(file_text)
            continue

        log(
            "ERROR",
            f"{source_label} customRules[{idx}] must be a string or {{\"file\": \"...\"}}",
        )
        sys.exit(1)

    return resolved


def _resolve_context_rules(context: dict[str, Any], repo_root: Path, label: str) -> dict[str, Any]:
    """Resolve customRules entries in a context map.

    Args:
        context: Context dict with optional customRules list.
        repo_root: Repository root directory.
        label: Label used for error reporting.

    Returns:
        Context dict with customRules resolved to list[str].
    """
    resolved = dict(context)
    raw_rules = resolved.get("customRules", [])
    if raw_rules is None:
        resolved["customRules"] = []
        return resolved
    if not isinstance(raw_rules, list):
        log("ERROR", f"{label} customRules must be a list")
        sys.exit(1)
    resolved["customRules"] = _resolve_rules(raw_rules, repo_root, label)
    return resolved


def _context_for_file(
    file_path: str,
    agent_context: dict[str, Any],
    repo_root: Path,
) -> dict[str, Any]:
    """Build the effective agent context for one file path.

    Merge strategy:
        1. Start with resolved global context.
        2. Find first matching scoped glob (if any).
        3. Apply scoped keys over global keys.
        4. Concatenate customRules: global + scoped.

    Args:
        file_path: Relative repo path for the file.
        agent_context: Normalized context config with global/scoped keys.
        repo_root: Repository root directory.

    Returns:
        Effective per-file context dict.
    """
    global_raw = agent_context.get("global", {})
    if not isinstance(global_raw, dict):
        log("ERROR", "agentContext.global must be an object")
        sys.exit(1)
    global_ctx = _resolve_context_rules(global_raw, repo_root, "agentContext.global")

    scoped_raw = agent_context.get("scoped", {})
    if not isinstance(scoped_raw, dict):
        log("ERROR", "agentContext.scoped must be an object")
        sys.exit(1)

    matched_glob: str | None = None
    matched_ctx_raw: dict[str, Any] = {}
    for glob, scoped_ctx in scoped_raw.items():
        if fnmatch.fnmatch(file_path, glob):
            if not isinstance(scoped_ctx, dict):
                log("ERROR", f"agentContext.scoped['{glob}'] must be an object")
                sys.exit(1)
            matched_glob = glob
            matched_ctx_raw = scoped_ctx
            break

    if matched_glob is None:
        return global_ctx

    scoped_ctx = _resolve_context_rules(
        matched_ctx_raw,
        repo_root,
        f"agentContext.scoped['{matched_glob}']",
    )
    merged = dict(global_ctx)
    for key, value in scoped_ctx.items():
        if key == "customRules":
            continue
        merged[key] = value
    merged["customRules"] = [
        *global_ctx.get("customRules", []),
        *scoped_ctx.get("customRules", []),
    ]
    return merged


# ---------------------------------------------------------------------------
# Exclusions
# ---------------------------------------------------------------------------

EXCLUDE_PATTERNS: list[re.Pattern[str]] = [
    re.compile(p)
    for p in [
        # Dependency lockfiles
        r".*\.lock$",
        r".*-lock\.json$",
        # Minified assets
        r".*\.min\.js$",
        r".*\.min\.css$",
        # Compiled / cache
        r".*\.pyc$",
        r"__pycache__/",
        # Binary / media
        r".*\.svg$",
        r".*\.png$",
        r".*\.jpg$",
        r".*\.jpeg$",
        r".*\.gif$",
        r".*\.ico$",
        r".*\.woff2?$",
        r".*\.ttf$",
        r".*\.eot$",
        r".*\.otf$",
        r".*\.pdf$",
        # Build output dirs
        r"^dist/",
        r"^build/",
        # Submodule paths are excluded dynamically — see get_submodule_exclusions()
    ]
]

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def get_submodule_exclusions() -> list[re.Pattern[str]]:
    """Return exclusion patterns for all registered git submodule paths.

    Runs ``git submodule status --cached`` to enumerate submodule paths.
    Each path produces a ``^<path>/`` anchor pattern so all files under it
    are excluded from review. Falls back to an empty list if git is unavailable
    or the repo has no submodules.

    Returns:
        List of compiled regex patterns, one per submodule path.
    """
    result = subprocess.run(
        ["git", "submodule", "status", "--cached"],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0 or not result.stdout.strip():
        return []
    patterns = []
    for line in result.stdout.splitlines():
        # Format: [+- ]<sha> <path> [(<describe>)]
        parts = line.strip().lstrip("+-U").split()
        if len(parts) >= 2:
            path = parts[1].rstrip("/")
            patterns.append(re.compile(rf"^{re.escape(path)}/"))
    return patterns


def log(level: str, message: str) -> None:
    """Print a timestamped log line to stdout.

    Args:
        level: Log level label (INFO, WARN, ERROR).
        message: Message text.
    """
    ts = datetime.now(tz=UTC).strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{ts}] [{level}] {message}", flush=True)


class GitError(RuntimeError):
    """Raised when a git subprocess exits with a non-zero return code."""


def git(*args: str) -> str:
    """Run a git command and return stdout.

    Args:
        *args: git subcommand and arguments.

    Returns:
        Decoded stdout of the git command.

    Raises:
        GitError: If the command exits with a non-zero return code.
        subprocess.TimeoutExpired: If the command exceeds GIT_TIMEOUT seconds.
    """
    result = subprocess.run(
        ["git", *args],
        capture_output=True,
        encoding="utf-8",
        errors="replace",
        timeout=GIT_TIMEOUT,
    )
    if result.returncode != 0:
        raise GitError(f"git {' '.join(args)} failed: {result.stderr.strip()}")
    return result.stdout


def file_slug(file: str) -> str:
    """Convert a relative file path to a safe filename stem.

    Args:
        file: Relative file path (e.g. src/foo/bar.py).

    Returns:
        Slug suitable for use as a filename stem (e.g. src-foo-bar-py).
    """
    return re.sub(r"[^\w\-]", "-", file)


def parse_name_status(raw: str) -> list[tuple[str, str]]:
    """Parse git diff -z --name-status output into (status_letter, path) pairs.

    Handles regular statuses (A/M/D/T/U) and rename/copy statuses (R/C).
    For renames and copies the destination (new) path is used.

    Args:
        raw: Raw NUL-delimited git diff --name-status output.

    Returns:
        List of (status_letter, path) pairs. Status letter is A/M/D/R/C/T/U.
    """
    tokens = [t for t in raw.split("\x00") if t]
    result: list[tuple[str, str]] = []
    idx = 0
    while idx < len(tokens):
        status = tokens[idx]
        letter = status[0]  # A, M, D, R, C, T, U
        if letter in ("R", "C"):
            # Rename/copy: status NUL old_path NUL new_path
            if idx + 2 < len(tokens):
                result.append((letter, tokens[idx + 2]))
                idx += 3
            else:
                idx += 1
        elif idx + 1 < len(tokens):
            result.append((letter, tokens[idx + 1]))
            idx += 2
        else:
            idx += 1
    return result


def _truncate_utf8(text: str, max_bytes: int) -> str:
    """Truncate text to at most *max_bytes* in UTF-8, preserving validity."""
    if max_bytes <= 0:
        return ""
    encoded = text.encode("utf-8")
    if len(encoded) <= max_bytes:
        return text
    return encoded[:max_bytes].decode("utf-8", errors="ignore")


def build_staging_filename(slug: str, *, suffix: str = "") -> str:
    """Build a staging filename that fits within filesystem name limits.

    Uses a deterministic hash suffix when truncation is required.

    Args:
        slug: Base filename stem.
        suffix: Optional suffix before .txt (e.g. "-chunk001").

    Returns:
        Safe filename that ends in .txt.
    """
    ext = ".txt"
    base_name = f"{slug}{suffix}{ext}"
    if len(base_name.encode("utf-8")) <= MAX_STAGE_FILENAME_BYTES:
        return base_name

    digest = hashlib.sha256(base_name.encode("utf-8")).hexdigest()[:12]
    hash_part = f"-{digest}"
    reserved_bytes = len((hash_part + suffix + ext).encode("utf-8"))
    slug_budget = max(MAX_STAGE_FILENAME_BYTES - reserved_bytes, 1)
    truncated_slug = _truncate_utf8(slug, slug_budget).rstrip("-._") or "file"

    candidate = f"{truncated_slug}{hash_part}{suffix}{ext}"
    assert len(candidate.encode("utf-8")) <= MAX_STAGE_FILENAME_BYTES, (
        f"invariant violated: {len(candidate.encode())} > {MAX_STAGE_FILENAME_BYTES}"
    )
    return candidate


def is_excluded(file: str) -> bool:
    """Check whether a file matches any exclusion pattern.

    Args:
        file: Relative file path.

    Returns:
        True if the file should be skipped.
    """
    return any(p.search(file) for p in EXCLUDE_PATTERNS)


def collect_sa_findings(file: str, sa_dir: Path) -> str:
    """Collect static analysis findings for a specific file from .sa/ output files.

    Scans each <sa_dir>/<tool>.txt file for lines that reference the given file path.
    Matches on the file basename and any line containing the repo-relative path or
    the /workspace/-prefixed path that SA tools emit.

    Args:
        file: Repo-relative file path (e.g. src/foo/bar.py).
        sa_dir: Path to the .sa/ directory in the workspace root.

    Returns:
        Formatted SA section string, or empty string if no findings or no .sa/ dir.
    """
    if not sa_dir.is_dir():
        return ""

    # Match lines containing the repo-relative path or /workspace/-prefixed path.
    # Most tools emit: path/to/file.py:line:col: message
    # shellcheck gcc format emits: /workspace/path/to/file.sh:line:col: message
    # Basename fallback intentionally omitted — too broad (e.g. auth.py matches
    # any line containing "auth" including unrelated paths).
    match_candidates = [
        file,                       # ruff: src/foo.py:42: ...
        f"/workspace/{file}",       # shellcheck: /workspace/src/foo.sh:42: ...
    ]

    findings: list[str] = []
    for sa_file in sorted(sa_dir.glob("*.txt")):
        tool = sa_file.stem
        try:
            lines = sa_file.read_text(encoding="utf-8", errors="replace").splitlines()
        except OSError:
            continue
        for line in lines:
            if any(candidate in line for candidate in match_candidates):
                findings.append(f"{tool}: {line}")

    if not findings:
        return ""

    body = "\n".join(findings)
    return f"\n=== STATIC ANALYSIS ===\n{body}\n=== END STATIC ANALYSIS ===\n"


def chunk_lines(text: str, size: int) -> list[str]:
    """Split text into chunks of at most *size* lines each.

    Args:
        text: Multi-line string to split.
        size: Maximum number of lines per chunk.

    Returns:
        List of text chunks, each containing at most *size* lines.
    """
    lines = text.splitlines(keepends=True)
    return ["".join(lines[i : i + size]) for i in range(0, len(lines), size)]


# ---------------------------------------------------------------------------
# Staging
# ---------------------------------------------------------------------------

_ADDED_FILE_GUIDANCE = (
    "=== REVIEW GUIDANCE ===\n"
    "This file was ADDED in this PR (no prior version exists in the repository).\n"
    "First assess: does this look like a bulk or automated import (generated code,\n"
    "synced documentation, vendored content, migration output)?\n"
    "  \u2192 If yes: provide a brief high-level overview of what it adds. Skip line-by-line review.\n"
    "  \u2192 If no:  perform a full detailed review as normal.\n"
    "=== END REVIEW GUIDANCE ===\n\n"
)


def stage_file(
    file: str,
    refspec: str,
    staging_dir: Path,
    repo_root: Path,
    agent: str,
    sa_dir: Path | None = None,
    *,
    status: str = "M",
) -> list[dict[str, object]]:
    """Fetch diff/content for one file and write staging input file(s).

    Args:
        file: Relative file path.
        refspec: Git refspec for the diff (e.g. "origin/master...HEAD").
        staging_dir: Directory to write input files into.
        repo_root: Absolute path to the repository root.
        agent: Agent name this file is routed to.
        sa_dir: Optional path to the .sa/ static analysis directory.
        status: Git diff status letter for this file (A/M/D/R/C/T).

    Returns:
        List of manifest task dicts for this file (one per chunk, or one total).
    """
    if not (repo_root / file).exists():
        log("WARN", f"Skipping deleted/missing file: {file}")
        return []

    slug = file_slug(file)
    file_diff = git("diff", refspec, "--", file)
    diff_lines = file_diff.count("\n")

    file_show = subprocess.run(
        ["git", "show", f"HEAD:{file}"],
        capture_output=True,
        encoding="utf-8",
        errors="replace",
        timeout=GIT_TIMEOUT,
    )
    file_content = file_show.stdout if file_show.returncode == 0 else ""
    content_lines = file_content.count("\n")
    combined_lines = content_lines + diff_lines

    sa_section = collect_sa_findings(file, sa_dir) if sa_dir else ""

    tasks: list[dict[str, object]] = []

    guidance = _ADDED_FILE_GUIDANCE if status == "A" else ""

    if combined_lines <= MAX_COMBINED_LINES:
        log("INFO", f"  {file}: full_and_diff ({combined_lines} lines)")
        input_text = (
            guidance
            + f"=== CURRENT FILE ({file}) ===\n{file_content}\n=== DIFF ===\n{file_diff}"
            + sa_section
        )
        input_file = build_staging_filename(slug)
        (staging_dir / input_file).write_text(input_text, encoding="utf-8")
        tasks.append(
            {
                "file": file,
                "slug": slug,
                "mode": "full_and_diff",
                "chunk": None,
                "total_chunks": None,
                "input_file": input_file,
                "agent": agent,
                "status": status,
            }
        )

    elif diff_lines <= MAX_DIFF_LINES:
        msg = (
            f"  {file}: diff_only (file {content_lines} lines, diff {diff_lines} lines)"
        )
        log("INFO", msg)
        input_file = build_staging_filename(slug)
        (staging_dir / input_file).write_text(guidance + file_diff + sa_section, encoding="utf-8")
        tasks.append(
            {
                "file": file,
                "slug": slug,
                "mode": "diff_only",
                "chunk": None,
                "total_chunks": None,
                "input_file": input_file,
                "agent": agent,
                "status": status,
            }
        )

    else:
        chunks = chunk_lines(file_diff, MAX_DIFF_LINES)
        total = len(chunks)
        msg = (
            f"  {file}: diff_chunk — {total} chunks"
            f" (file {content_lines} lines, diff {diff_lines} lines)"
        )
        log("INFO", msg)
        for idx, chunk_text in enumerate(chunks, start=1):
            input_file = build_staging_filename(slug, suffix=f"-chunk{idx:03d}")
            # SA section appended to the last chunk only — avoids repeating findings
            # across chunks of the same file while ensuring the LLM always sees them.
            chunk_sa = sa_section if idx == total else ""
            # Guidance prepended to the first chunk only so the LLM sees it once.
            chunk_guidance = guidance if idx == 1 else ""
            (staging_dir / input_file).write_text(chunk_guidance + chunk_text + chunk_sa, encoding="utf-8")
            tasks.append(
                {
                    "file": file,
                    "slug": slug,
                    "mode": "diff_chunk",
                    "chunk": idx,
                    "total_chunks": total,
                    "input_file": input_file,
                    "agent": agent,
                    "status": status,
                }
            )

    return tasks


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


# ---------------------------------------------------------------------------
# Main helpers
# ---------------------------------------------------------------------------


def _setup_git(
    base_branch: str,
    staging_dir: Path,
) -> tuple[str, Path, list[str], dict[str, str], list[tuple[str, str]]]:
    """Configure git, compute the refspec, and return changed file data.

    Marks the workspace as git-safe (required in Jenkins containers), logs
    environment info, and returns the list of files changed in the PR along
    with their status codes.

    Args:
        base_branch: Base branch name to diff against.
        staging_dir: Staging directory path (used for logging only).

    Returns:
        Tuple of (refspec, repo_root, all_files, file_status, status_pairs).

    Raises:
        SystemExit: With code 1 on git failure, code 2 if no changes detected.
    """
    EXCLUDE_PATTERNS.extend(get_submodule_exclusions())

    subprocess.run(
        ["git", "config", "--global", "--add", "safe.directory", str(Path.cwd())],
        check=True,
    )

    refspec = f"origin/{base_branch}...HEAD"

    log("INFO", "=" * 50)
    log("INFO", "git-diff-prep")
    log("INFO", "=" * 50)
    log("INFO", f"Base branch:  {base_branch}")
    log("INFO", f"Refspec:      {refspec}")
    log("INFO", f"Staging dir:  {staging_dir}")

    # Log whether the clone is shallow — informational only.
    # Fetch operations are intentionally omitted here: this script runs inside a Docker
    # container that has no SSH agent, so any git fetch would fail.
    # The Jenkinsfile 'Checkout PR Branch' stage handles branch switching on the host agent
    # before Docker starts, where SSH credentials are available.
    shallow_result = subprocess.run(
        ["git", "rev-parse", "--is-shallow-repository"],
        capture_output=True,
        text=True,
    )
    is_shallow = shallow_result.stdout.strip() == "true"
    log("INFO", f"Shallow clone: {is_shallow}")

    # Verify a common ancestor exists before attempting the three-dot diff.
    # A missing merge base means the feature branch and the base branch have
    # disconnected histories — most commonly caused by pushing a branch from
    # a different origin repo where the base branch was never pushed across.
    merge_base_result = subprocess.run(
        ["git", "merge-base", f"origin/{base_branch}", "HEAD"],
        capture_output=True,
        text=True,
    )
    if merge_base_result.returncode != 0:
        log(
            "ERROR",
            f"No common ancestor found between 'origin/{base_branch}' and HEAD. "
            f"The feature branch and '{base_branch}' have disconnected git histories. "
            f"Ensure '{base_branch}' in the target repo was pushed from the same origin "
            f"as the feature branch so a merge base exists.",
        )
        sys.exit(1)
    log("INFO", f"Merge base: {merge_base_result.stdout.strip()}")

    try:
        repo_root = Path(git("rev-parse", "--show-toplevel").strip())
        changed_raw = git("diff", "-z", "--name-status", refspec)
    except GitError as exc:
        log("ERROR", str(exc))
        sys.exit(1)

    status_pairs = parse_name_status(changed_raw)
    all_files = [path for _, path in status_pairs]
    file_status: dict[str, str] = {path: s for s, path in status_pairs}

    log("INFO", f"Raw diff -z --name-status output ({len(all_files)} files):")
    if all_files:
        for _s, _f in status_pairs:
            log("INFO", f"  [{_s}] {_f}")
    else:
        log("INFO", "  (empty)")

    if not all_files:
        log("WARN", f"No changes detected against origin/{base_branch}")
        sys.exit(2)

    return refspec, repo_root, all_files, file_status, status_pairs


def _classify_files(
    all_files: list[str],
    routing: list[tuple[str, list[str]]],
) -> tuple[list[str], list[str]]:
    """Partition changed files into reviewable and excluded lists.

    Args:
        all_files: All changed file paths from git diff.
        routing: Ordered list of (agent, [glob, ...]) tuples.

    Returns:
        Tuple of (reviewable, excluded) file lists.
    """
    reviewable: list[str] = []
    pattern_excluded: list[str] = []
    unrouted: list[str] = []
    for file in all_files:
        if is_excluded(file):
            pattern_excluded.append(file)
        elif classify_file(file, routing) is not None:
            reviewable.append(file)
        else:
            unrouted.append(file)
    excluded = pattern_excluded + unrouted

    log("INFO", f"Changed files: {len(all_files)}")
    log("INFO", f"Reviewable:    {len(reviewable)}")
    log(
        "INFO",
        f"Excluded:      {len(excluded)} ({len(unrouted)} unrouted, {len(pattern_excluded)} pattern-excluded)",
    )
    return reviewable, excluded


def _detect_sa_dir() -> Path | None:
    """Auto-detect the .sa/ static analysis directory.

    Returns:
        Path to .sa/ if it exists, else None.
    """
    sa_dir = Path(".sa")
    if sa_dir.is_dir():
        sa_files = list(sa_dir.glob("*.txt"))
        log("INFO", f"Static analysis: {len(sa_files)} tool output(s) found in .sa/")
        return sa_dir
    log("INFO", "Static analysis: .sa/ not present — skipping SA embedding")
    return None


def _stage_reviewable_files(
    reviewable: list[str],
    excluded: list[str],
    routing: list[tuple[str, list[str]]],
    refspec: str,
    staging_dir: Path,
    repo_root: Path,
    sa_dir: Path | None,
    base_branch: str,
    file_status: dict[str, str],
    agent_context: dict[str, Any],
    personas: dict[str, str],
) -> tuple[list[dict[str, object]], dict[str, list[str]], list[str]]:
    """Stage each reviewable file and write the top-level manifest.json.

    Args:
        reviewable: Files routed to a review agent.
        excluded: Already-excluded file paths.
        routing: Ordered list of (agent, [glob, ...]) tuples.
        refspec: Git refspec for the diff.
        staging_dir: Directory to write staging files into.
        repo_root: Absolute path to the repository root.
        sa_dir: Path to .sa/ directory, or None.
        base_branch: Base branch name.
        file_status: Mapping of file path to git status code (A/M/D/R...).
        agent_context: Normalized context config with global/scoped keys.
        personas: Dict of {agent: resolved persona text} from routing config.

    Returns:
        Tuple of (tasks, review_agents, excluded). excluded may be extended
        with files that were deleted or lost their routing.
    """
    # Work on a local copy so callers are not surprised by mutation.
    excluded = list(excluded)
    tasks: list[dict[str, object]] = []
    review_agents: dict[str, list[str]] = {}
    skipped: list[str] = []
    log("INFO", "Included:")
    for file in reviewable:
        if not (repo_root / file).exists():
            skipped.append(file)
            continue
        agent = classify_file(file, routing)
        if agent is None:
            # Shouldn't reach here (filtered above), but guard defensively
            excluded.append(file)
            continue
        review_agents.setdefault(agent, []).append(file)
        try:
            file_context = _context_for_file(file, agent_context, repo_root)
            staged_tasks = stage_file(
                file, refspec, staging_dir, repo_root, agent, sa_dir,
                status=file_status.get(file, "M"),
            )
            for staged_task in staged_tasks:
                staged_task["agent_context"] = file_context
                if agent in personas:
                    staged_task["persona"] = personas[agent]
            tasks.extend(staged_tasks)
        except GitError as exc:
            log("WARN", f"  git error staging {file}: {exc}")

    if skipped:
        log("INFO", f"Skipped (deleted):  {len(skipped)}")
        for f in skipped:
            log("WARN", f"  {f}: deleted — skipped")

    log("INFO", "Agent routing:")
    for agent, files in review_agents.items():
        log("INFO", f"  {agent}: {len(files)} file(s)")

    manifest = {
        "base_branch": base_branch,
        "refspec": refspec,
        "review_agents": review_agents,
        "reviewable": tasks,
        "excluded": excluded,
    }
    (staging_dir / "manifest.json").write_text(
        json.dumps(manifest, indent=2), encoding="utf-8"
    )
    log("INFO", f"Staged {len(tasks)} review task(s) for {len(reviewable)} file(s)")
    log("INFO", f"Manifest: {staging_dir / 'manifest.json'}")
    return tasks, review_agents, excluded


def _stage_skill_batches(
    tasks: list[dict[str, object]],
    review_agents: dict[str, list[str]],
    excluded: list[str],
    staging_dir: Path,
    repo_root: Path,
    base_branch: str,
    refspec: str,
    batch_size: int,
) -> tuple[list[dict[str, object]], list[str]]:
    """Write per-skill and per-batch manifests for Jenkins wave orchestration.

    Each batch dir is self-contained: its manifest.json plus all referenced
    input .txt files, so copilot-dispatch.sh only needs the batch dir path.

    Batching strategy — cluster-aware greedy packing:
      - Code files (dep_clusters.py): Parse imports → union-find → clusters.
      - Markdown files (doc_clusters.py): Parse links → union-find → clusters.
      Greedy bin-pack clusters into batches so related files land together.

    Args:
        tasks: All staged review tasks.
        review_agents: Mapping of agent name to list of file paths.
        excluded: Excluded file paths for manifest embedding.
        staging_dir: Root staging directory.
        repo_root: Absolute path to the repository root.
        base_branch: Base branch name.
        refspec: Git refspec for the diff.
        batch_size: Maximum tasks per batch.

    Returns:
        Tuple of (batch_entries, code_skill_names).
    """
    by_skill: dict[str, list[dict[str, object]]] = {}
    for task in tasks:
        by_skill.setdefault(task["agent"], []).append(task)  # type: ignore[arg-type]

    doc_changed_files = [
        str(t["file"]) for t in tasks if str(t["file"]).endswith(".md")
    ]
    corpus_index_data: list[dict[str, object]] = []
    if doc_changed_files:
        corpus_index_data = build_corpus_index(repo_root)
        log("INFO", f"Corpus index: {len(corpus_index_data)} .md file(s) indexed")

    batch_entries: list[dict[str, object]] = []
    for skill, skill_tasks in by_skill.items():
        skill_staging = staging_dir / skill
        skill_staging.mkdir(parents=True, exist_ok=True)

        skill_manifest = {
            "base_branch": base_branch,
            "refspec": refspec,
            "skill_dir": skill,
            "review_agents": {skill: review_agents.get(skill, [])},
            "reviewable": skill_tasks,
            "excluded": excluded,
        }
        (skill_staging / "manifest.json").write_text(
            json.dumps(skill_manifest, indent=2), encoding="utf-8"
        )

        skill_md_files = [
            str(t["file"]) for t in skill_tasks if str(t["file"]).endswith(".md")
        ]
        skill_has_md = bool(skill_md_files)
        skill_doc_clusters: list[list[str]] = []
        skill_reverse_links: dict[str, list[str]] = {}
        if skill_has_md:
            batches = doc_cluster_aware_batches(skill_tasks, batch_size, repo_root)
            skill_doc_clusters = [
                c for c in cluster_docs(skill_md_files, repo_root) if len(c) > 1
            ]
            skill_reverse_links = reverse_links(skill_md_files, repo_root)
        else:
            batches = cluster_aware_batches(skill_tasks, batch_size, repo_root)
        total_batches = len(batches)
        for batch_idx, batch_slice in enumerate(batches):
            batch_num = batch_idx + 1
            batch_dir = skill_staging / f"batch_{batch_num:03d}"
            batch_dir.mkdir(parents=True, exist_ok=True)

            batch_manifest: dict[str, object] = {
                "base_branch": base_branch,
                "refspec": refspec,
                "skill_dir": skill,
                "review_agents": {skill: review_agents.get(skill, [])},
                "reviewable": batch_slice,
                "excluded": excluded,
            }
            if skill_has_md:
                batch_manifest["doc_clusters"] = skill_doc_clusters
                batch_manifest["reverse_links"] = skill_reverse_links
            (batch_dir / "manifest.json").write_text(
                json.dumps(batch_manifest, indent=2), encoding="utf-8"
            )

            for task in batch_slice:
                src = staging_dir / task["input_file"]
                if src.exists():
                    shutil.copy2(src, batch_dir / task["input_file"])

            if skill_has_md and corpus_index_data:
                (batch_dir / "corpus-index.json").write_text(
                    json.dumps(corpus_index_data, indent=2), encoding="utf-8"
                )

            dirs_in_batch = sorted(
                {str(Path(t["file"]).parent) for t in batch_slice}
            )
            dirs_str = ", ".join(dirs_in_batch)
            log(
                "INFO",
                f"  Batch {batch_num:03d}: {len(batch_slice)} task(s)"
                f" from {len(dirs_in_batch)} dir(s): {dirs_str}",
            )
            batch_entries.append(
                {
                    "skill": skill,
                    "batch_num": f"{batch_num:03d}",
                    "task_count": len(batch_slice),
                    "staging_dir": str(batch_dir),
                }
            )

        log(
            "INFO",
            f"Skill {skill}: {len(skill_tasks)} task(s)"
            f" \u2192 {total_batches} batch(es) (directory-aware)",
        )

    return batch_entries, list(by_skill.keys())


def _stage_cross_skill_batches(  # noqa: PLR0912, PLR0915
    all_files: list[str],
    sa_dir: Path | None,
    staging_dir: Path,
    repo_root: Path,
    base_branch: str,
    refspec: str,
    file_status: dict[str, str],
    status_pairs: list[tuple[str, str]],
    agent_context: dict[str, Any],
    code_skill_names: list[str],
    summary_config: dict[str, Any] | None = None,
) -> tuple[list[dict[str, object]], list[str]]:
    """Stage summary, blast-radius, and technical batches (wave-1 skills).

    All three skills operate over the full set of changed files rather than
    the per-agent routing subsets used by the code/docs/conf skills.

    Args:
        all_files: All changed file paths from git diff.
        sa_dir: Path to .sa/ directory, or None.
        staging_dir: Root staging directory.
        repo_root: Absolute path to the repository root.
        base_branch: Base branch name.
        refspec: Git refspec for the diff.
        file_status: Mapping of file path to git status code.
        status_pairs: List of (status, path) tuples from parse_name_status.
        agent_context: Normalized context config with global/scoped keys.
        code_skill_names: Skill names from the per-agent routing stage.
        summary_config: Optional per-section enable/instructions overrides.

    Returns:
        Tuple of (extra_batch_entries, extra_skill_names) to prepend to the
        batch plan, ordered so the most-wave-1 skill comes first.
    """
    summary_skill = "pr-review-summary"
    technical_skill = "pr-review-technical"
    blast_radius_skill = "pr-review-blast-radius"

    # ------------------------------------------------------------------
    # Summary batch (batch_000) — diff-only, all non-excluded files
    # ------------------------------------------------------------------
    summary_staging = staging_dir / summary_skill / _CROSS_SKILL_BATCH_DIR
    summary_staging.mkdir(parents=True, exist_ok=True)

    summary_tasks: list[dict[str, object]] = []
    summary_candidates = [f for f in all_files if not is_excluded(f)]
    for file in summary_candidates:
        if not (repo_root / file).exists():
            continue
        slug = file_slug(file)
        try:
            file_diff = git("diff", refspec, "--", file)
        except GitError as exc:
            log("WARN", f"  summary: git error for {file}: {exc}")
            continue
        if not file_diff.strip():
            continue
        input_file = build_staging_filename(slug)
        (summary_staging / input_file).write_text(file_diff, encoding="utf-8")
        summary_tasks.append(
            {
                "file": file,
                "slug": slug,
                "mode": _MODE_DIFF_ONLY,
                "chunk": None,
                "total_chunks": None,
                "input_file": input_file,
                "agent": summary_skill,
                "status": file_status.get(file, "M"),
                "agent_context": _context_for_file(file, agent_context, repo_root),
            }
        )

    summary_files = [str(t["file"]) for t in summary_tasks]
    summary_clusters = cluster_files(summary_files, repo_root)
    summary_dep_clusters = [c for c in summary_clusters if len(c) > 1]
    log("INFO", f"Summary dep clusters: {len(summary_dep_clusters)} multi-file cluster(s)")

    summary_reverse_deps = reverse_deps(summary_files, repo_root)
    log(
        "INFO",
        f"Summary reverse deps: {len(summary_reverse_deps)} changed file(s) have external importers",
    )

    summary_sa: dict[str, str] = {}
    if sa_dir is not None:
        for sa_file in sorted(sa_dir.glob("*.txt")):
            try:
                summary_sa[sa_file.stem] = sa_file.read_text(
                    encoding="utf-8", errors="replace"
                )
            except OSError:
                pass
        if summary_sa:
            log("INFO", f"Summary SA: {len(summary_sa)} tool output(s) embedded in manifest")

    status_breakdown: dict[str, int] = {}
    for s, _ in status_pairs:
        status_breakdown[s] = status_breakdown.get(s, 0) + 1

    summary_manifest: dict[str, object] = {
        "base_branch": base_branch,
        "refspec": refspec,
        "skill_dir": summary_skill,
        "review_agents": {summary_skill: [t["file"] for t in summary_tasks]},
        "reviewable": summary_tasks,
        "excluded": [],
        "dep_clusters": summary_dep_clusters,
        "reverse_deps": summary_reverse_deps,
        "static_analysis": summary_sa,
        "status_breakdown": status_breakdown,
        "summary_config": summary_config or {},
    }
    (summary_staging / "manifest.json").write_text(
        json.dumps(summary_manifest, indent=2), encoding="utf-8"
    )
    log("INFO", f"Summary batch: {len(summary_tasks)} file(s) staged in {summary_staging}")

    # ------------------------------------------------------------------
    # Blast-radius batch — only when reverse_deps is non-empty
    # ------------------------------------------------------------------
    blast_radius_batch_entries: list[dict[str, object]] = []
    if summary_reverse_deps:
        blast_radius_staging = staging_dir / blast_radius_skill / _CROSS_SKILL_BATCH_DIR
        blast_radius_staging.mkdir(parents=True, exist_ok=True)
        for task in summary_tasks:
            src = summary_staging / str(task["input_file"])
            dst = blast_radius_staging / str(task["input_file"])
            if src.exists() and not dst.exists():
                shutil.copy2(src, dst)
        blast_radius_manifest: dict[str, object] = {
            "base_branch": base_branch,
            "refspec": refspec,
            "skill_dir": blast_radius_skill,
            "review_agents": {blast_radius_skill: [t["file"] for t in summary_tasks]},
            "reviewable": summary_tasks,
            "excluded": [],
            "dep_clusters": summary_dep_clusters,
            "reverse_deps": summary_reverse_deps,
            "static_analysis": summary_sa,
        }
        (blast_radius_staging / "manifest.json").write_text(
            json.dumps(blast_radius_manifest, indent=2), encoding="utf-8"
        )
        log(
            "INFO",
            f"Blast radius batch: {len(summary_tasks)} file(s) staged in {blast_radius_staging}",
        )
        blast_radius_batch_entries.append(
            {
                "skill": blast_radius_skill,
                "batch_num": _CROSS_SKILL_BATCH_NUM,
                "task_count": len(summary_tasks),
                "staging_dir": str(blast_radius_staging),
            }
        )
    else:
        log("INFO", "Blast radius batch: skipped — no reverse dependencies found")

    # ------------------------------------------------------------------
    # Technical batch (batch_000) — same scope as summary
    # ------------------------------------------------------------------
    technical_staging = staging_dir / technical_skill / _CROSS_SKILL_BATCH_DIR
    technical_staging.mkdir(parents=True, exist_ok=True)
    for task in summary_tasks:
        src = summary_staging / task["input_file"]
        dst = technical_staging / task["input_file"]
        if src.exists() and not dst.exists():
            shutil.copy2(src, dst)
    technical_manifest: dict[str, object] = {
        "base_branch": base_branch,
        "refspec": refspec,
        "skill_dir": technical_skill,
        "review_agents": {technical_skill: [t["file"] for t in summary_tasks]},
        "reviewable": summary_tasks,
        "excluded": [],
        "dep_clusters": summary_dep_clusters,
        "reverse_deps": summary_reverse_deps,
    }
    (technical_staging / "manifest.json").write_text(
        json.dumps(technical_manifest, indent=2), encoding="utf-8"
    )
    log(
        "INFO",
        f"Technical review batch: {len(summary_tasks)} file(s) staged in {technical_staging}",
    )

    # ------------------------------------------------------------------
    # Assemble extra entries + skill names in wave-1-first order
    # ------------------------------------------------------------------
    extra_entries: list[dict[str, object]] = []
    extra_skills: list[str] = []

    for entry in reversed(blast_radius_batch_entries):
        extra_entries.insert(0, entry)
    if blast_radius_batch_entries:
        extra_skills.insert(0, blast_radius_skill)

    technical_entry: dict[str, object] = {
        "skill": technical_skill,
        "batch_num": _CROSS_SKILL_BATCH_NUM,
        "task_count": len(summary_tasks),
        "staging_dir": str(technical_staging),
    }
    extra_entries.insert(0, technical_entry)
    if technical_skill not in extra_skills:
        extra_skills.insert(0, technical_skill)

    summary_entry: dict[str, object] = {
        "skill": summary_skill,
        "batch_num": _CROSS_SKILL_BATCH_NUM,
        "task_count": len(summary_tasks),
        "staging_dir": str(summary_staging),
    }
    extra_entries.insert(0, summary_entry)
    if summary_skill not in code_skill_names:
        extra_skills.insert(0, summary_skill)

    return extra_entries, extra_skills


def _write_batch_plan(
    batch_entries: list[dict[str, object]],
    batch_plan_skills: list[str],
    staging_dir: Path,
) -> None:
    """Assemble and write batch-plan.json to the staging directory.

    Args:
        batch_entries: Ordered list of batch entry dicts.
        batch_plan_skills: Ordered list of skill names for the plan.
        staging_dir: Root staging directory.
    """
    batch_plan: dict[str, object] = {
        "batches": batch_entries,
        "skills": batch_plan_skills,
        "total_batches": len(batch_entries),
    }
    (staging_dir / "batch-plan.json").write_text(
        json.dumps(batch_plan, indent=2), encoding="utf-8"
    )
    log(
        "INFO",
        f"Batch plan: {len(batch_entries)} batch(es) \u2192 {staging_dir / 'batch-plan.json'}",
    )


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main() -> None:
    """Prepare the staging directory and write manifest.json.

    Raises:
        SystemExit: On argument errors or git failures.
    """
    # Parse args: <base-branch> <staging-dir> [--routing <path>] [--agent-context <path>] [--summary-config <path>]
    args = sys.argv[1:]
    routing_path: Path | None = None
    agent_context_path: Path | None = None
    summary_config_path: Path | None = None
    if "--routing" in args:
        idx = args.index("--routing")
        if idx + 1 >= len(args):
            print(
                f"Usage: {sys.argv[0]} <base-branch> <staging-dir> [--routing <path>] [--agent-context <path>]"
            )
            sys.exit(1)
        routing_path = Path(args[idx + 1])
        args = args[:idx] + args[idx + 2 :]
    if "--agent-context" in args:
        idx = args.index("--agent-context")
        if idx + 1 >= len(args):
            print(
                f"Usage: {sys.argv[0]} <base-branch> <staging-dir> [--routing <path>] [--agent-context <path>] [--summary-config <path>]"
            )
            sys.exit(1)
        agent_context_path = Path(args[idx + 1])
        args = args[:idx] + args[idx + 2 :]
    if "--summary-config" in args:
        idx = args.index("--summary-config")
        if idx + 1 >= len(args):
            print(
                f"Usage: {sys.argv[0]} <base-branch> <staging-dir> [--routing <path>] [--agent-context <path>] [--summary-config <path>]"
            )
            sys.exit(1)
        summary_config_path = Path(args[idx + 1])
        args = args[:idx] + args[idx + 2 :]
    if len(args) != 2:
        print(
            f"Usage: {sys.argv[0]} <base-branch> <staging-dir> [--routing <path>] [--agent-context <path>] [--summary-config <path>]"
        )
        sys.exit(1)

    base_branch = args[0]
    staging_dir = Path(args[1])
    staging_dir.mkdir(parents=True, exist_ok=True)

    routing, persona_paths = load_routing(routing_path)
    summary_config: dict[str, Any] | None = None
    if summary_config_path is not None:
        if not summary_config_path.exists():
            log("ERROR", f"--summary-config file not found: {summary_config_path}")
            sys.exit(1)
        summary_config = json.loads(summary_config_path.read_text(encoding="utf-8"))

    refspec, repo_root, all_files, file_status, status_pairs = _setup_git(
        base_branch, staging_dir
    )
    agent_context = load_agent_context_config(agent_context_path)
    personas = _resolve_routing_personas(persona_paths, repo_root)
    reviewable, excluded = _classify_files(all_files, routing)

    if not reviewable:
        log("WARN", "All changed files excluded — nothing to review")
        sys.exit(2)

    sa_dir = _detect_sa_dir()
    tasks, review_agents, excluded = _stage_reviewable_files(
        reviewable, excluded, routing, refspec, staging_dir, repo_root,
        sa_dir, base_branch, file_status, agent_context, personas,
    )

    batch_size = int(os.environ.get("COPILOT_BATCH_SIZE", "15"))
    batch_entries, code_skill_names = _stage_skill_batches(
        tasks, review_agents, excluded, staging_dir, repo_root,
        base_branch, refspec, batch_size,
    )

    cross_entries, cross_skills = _stage_cross_skill_batches(
        all_files, sa_dir, staging_dir, repo_root, base_branch, refspec,
        file_status, status_pairs, agent_context, code_skill_names, summary_config,
    )
    for entry in reversed(cross_entries):
        batch_entries.insert(0, entry)
    batch_plan_skills = cross_skills + code_skill_names
    _write_batch_plan(batch_entries, batch_plan_skills, staging_dir)


if __name__ == "__main__":
    main()
