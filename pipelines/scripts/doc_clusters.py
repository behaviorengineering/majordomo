"""Doc-link clustering for changed markdown files.

Parses ``[text](path)`` links from changed files and groups them into connected
components using a union-find structure, so that docs that link to each other
land in the same review batch.

Also provides:

- Reverse link lookup: given a set of changed files, scans the full repo
  to find which unchanged ``.md`` files link to any of them.
- Corpus index builder: extracts title, headings, and key terms from every
  ``.md`` file in the repo for agent-side semantic context.

Stdlib-only — no third-party dependencies required.

Usage::

    from doc_clusters import (
        build_corpus_index,
        cluster_aware_batches,
        cluster_docs,
        reverse_links,
    )
    from pathlib import Path

    clusters = cluster_docs(changed_files, Path("/repo"))

    batches = cluster_aware_batches(skill_tasks, batch_size=15, repo_root=Path("/repo"))

    rev = reverse_links(changed_files, Path("/repo"))

    index = build_corpus_index(Path("/repo"))
"""

from __future__ import annotations

import re
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

# Minimum character length for a term to be included in the corpus index.
_MIN_TERM_LEN: int = 3

# Directories to skip when scanning the full repo tree for .md files.
_SCAN_EXCLUDE_DIRS: frozenset[str] = frozenset(
    {
        "node_modules",
        ".git",
        "__pycache__",
        ".venv",
        "venv",
        "dist",
        "build",
        ".tox",
        ".mypy_cache",
        ".pytest_cache",
        ".ruff_cache",
        ".eggs",
    }
)

# Matches inline markdown links: [text](target)
# Group 1: link text (unused); Group 2: raw link target.
# The negative lookbehind (?<!!) skips image links ![]().
_INLINE_LINK_RE: re.Pattern[str] = re.compile(
    r"(?<!!)\[([^\]]*)\]\(([^)]+)\)",
    re.MULTILINE,
)

# Matches reference-style link definitions: [label]: url
# Group 1: label; Group 2: URL.
_REF_DEF_RE: re.Pattern[str] = re.compile(
    r"^\[([^\]]+)\]:\s+(\S+)",
    re.MULTILINE,
)

# Matches reference-style link usages: [text][label] or [text][]
# Group 1: display text; Group 2: label (empty string for shortcut form).
_REF_USE_RE: re.Pattern[str] = re.compile(
    r"(?<!!)\[([^\]]+)\]\[([^\]]*)\]",
    re.MULTILINE,
)

# Matches H1 headings. Group 1: heading text.
_H1_RE: re.Pattern[str] = re.compile(r"^#\s+(.+)$", re.MULTILINE)

# Matches H2 and H3 headings. Group 1: heading text.
_H2_H3_RE: re.Pattern[str] = re.compile(r"^#{2,3}\s+(.+)$", re.MULTILINE)

# Matches inline code spans. Group 1: span content.
# Excludes backtick and newline inside the span to avoid fenced block bleed.
_BACKTICK_RE: re.Pattern[str] = re.compile(r"`([^`\n]+)`")

# Matches bold text. Group 1: bold content.
_BOLD_RE: re.Pattern[str] = re.compile(r"\*\*([^*\n]+)\*\*")


# ---------------------------------------------------------------------------
# Union-Find
# ---------------------------------------------------------------------------


class _UnionFind:
    """Path-compressed, union-by-rank structure over string keys."""

    def __init__(self, items: list[str]) -> None:
        """Initialise with one component per item.

        Args:
            items: All keys to track.
        """
        self._parent: dict[str, str] = {item: item for item in items}
        self._rank: dict[str, int] = dict.fromkeys(items, 0)

    def find(self, item: str) -> str:
        """Return root representative with path compression.

        Args:
            item: Key to find root for.

        Returns:
            Root representative of the component containing *item*.
        """
        if self._parent[item] != item:
            self._parent[item] = self.find(self._parent[item])
        return self._parent[item]

    def union(self, a: str, b: str) -> None:
        """Merge the components containing *a* and *b*.

        Args:
            a: First key.
            b: Second key.
        """
        root_a, root_b = self.find(a), self.find(b)
        if root_a == root_b:
            return
        if self._rank[root_a] < self._rank[root_b]:
            root_a, root_b = root_b, root_a
        self._parent[root_b] = root_a
        if self._rank[root_a] == self._rank[root_b]:
            self._rank[root_a] += 1

    def components(self) -> list[list[str]]:
        """Return all connected components.

        Returns:
            List of components; each component is a list of keys sharing a root.
        """
        groups: dict[str, list[str]] = {}
        for item in self._parent:
            root = self.find(item)
            groups.setdefault(root, []).append(item)
        return list(groups.values())


# ---------------------------------------------------------------------------
# Private helpers
# ---------------------------------------------------------------------------


def _resolve_md_link(target: str, current_file: Path, root: Path) -> str | None:
    """Resolve a markdown link target to a repo-relative POSIX path.

    Strips anchor fragments. Returns None for external links, anchor-only
    links, or targets that resolve outside the repository root.

    Args:
        target: Raw link target string from the markdown source.
        current_file: Absolute path of the file containing the link.
        root: Repository root used to relativise resolved paths.

    Returns:
        Repo-relative POSIX path, or None if the link is not a local file ref.
    """
    if "#" in target:
        target = target[: target.index("#")]

    target = target.strip()

    if not target:
        return None

    if target.startswith(("http://", "https://", "ftp://", "mailto:", "//")):
        return None

    if target.startswith("/"):
        resolved = (root / target.lstrip("/")).resolve()
    else:
        resolved = (current_file.parent / target).resolve()

    try:
        rel = resolved.relative_to(root.resolve())
    except ValueError:
        return None

    return rel.as_posix()


def _parse_md_links(
    path: Path,
    root: Path,
    target_set: frozenset[str],
) -> set[str]:
    """Return the subset of target_set directly linked from the markdown file at path.

    Handles both inline links ``[text](url)`` and reference-style links
    ``[text][label]`` with ``[label]: url`` definitions.

    Args:
        path: Absolute path to the markdown file to parse.
        root: Repository root for resolving relative link paths.
        target_set: Frozenset of repo-relative POSIX paths to match against.

    Returns:
        Subset of target_set that this file links to directly.
    """
    try:
        content = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return set()

    ref_defs: dict[str, str] = {}
    for match in _REF_DEF_RE.finditer(content):
        label = match.group(1).lower().strip()
        ref_defs[label] = match.group(2).strip()

    raw_targets: list[str] = []

    for match in _INLINE_LINK_RE.finditer(content):
        raw_targets.append(match.group(2).strip())

    for match in _REF_USE_RE.finditer(content):
        text = match.group(1).strip()
        label = match.group(2).strip()
        lookup = label if label else text
        resolved_ref = ref_defs.get(lookup.lower())
        if resolved_ref:
            raw_targets.append(resolved_ref)

    found: set[str] = set()
    for raw in raw_targets:
        resolved = _resolve_md_link(raw, path, root)
        if resolved and resolved in target_set:
            found.add(resolved)

    return found


def _extract_title(content: str) -> str:
    """Return the first H1 heading text, or empty string if none.

    Args:
        content: Raw markdown content string.

    Returns:
        Heading text without the leading ``#`` and surrounding whitespace,
        or an empty string if no H1 heading is found.
    """
    match = _H1_RE.search(content)
    return match.group(1).strip() if match else ""


def _extract_headings(content: str) -> list[str]:
    """Return all H2 and H3 heading texts in document order.

    Args:
        content: Raw markdown content string.

    Returns:
        List of heading texts without leading ``#`` markers.
    """
    return [m.group(1).strip() for m in _H2_H3_RE.finditer(content)]


def _extract_key_terms(content: str) -> list[str]:
    """Return sorted, deduplicated key terms from backtick spans and bold text.

    Terms shorter than _MIN_TERM_LEN characters are discarded to filter out
    single-character operators and punctuation.

    Args:
        content: Raw markdown content string.

    Returns:
        Sorted list of unique term strings.
    """
    terms: set[str] = set()

    for match in _BACKTICK_RE.finditer(content):
        term = match.group(1).strip()
        if len(term) >= _MIN_TERM_LEN:
            terms.add(term)

    for match in _BOLD_RE.finditer(content):
        term = match.group(1).strip()
        if len(term) >= _MIN_TERM_LEN:
            terms.add(term)

    return sorted(terms)


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def cluster_docs(
    changed_files: list[str],
    repo_root: Path,
) -> list[list[str]]:
    """Group changed markdown files into link-based clusters.

    Files that link to each other (directly, within the changed set) are
    placed in the same cluster. Non-markdown files and files with no
    intra-PR link edges each form single-member clusters.

    Args:
        changed_files: Repo-relative POSIX paths of changed files.
        repo_root: Absolute path to the repository root.

    Returns:
        List of clusters; each cluster is a list of repo-relative paths.
    """
    if not changed_files:
        return []

    changed: frozenset[str] = frozenset(changed_files)
    uf = _UnionFind(changed_files)

    for rel_str in changed_files:
        path = repo_root / rel_str
        if not path.exists() or path.suffix.lower() != ".md":
            continue
        neighbours = _parse_md_links(path, repo_root, changed)
        for neighbour in neighbours:
            uf.union(rel_str, neighbour)

    return uf.components()


def cluster_aware_batches(
    skill_tasks: list[dict[str, object]],
    batch_size: int,
    repo_root: Path,
) -> list[list[dict[str, object]]]:
    """Pack manifest tasks into batches that keep doc link clusters together.

    Docs that link to each other land in the same batch. When a cluster
    exceeds batch_size, it is split into consecutive same-cluster sub-batches
    to preserve locality. Unrelated clusters are bin-packed greedily.

    Args:
        skill_tasks: All manifest tasks for one skill (may include multiple
            tasks per file when the file was chunked).
        batch_size: Maximum number of tasks per batch.
        repo_root: Absolute path to the repository root (for link resolution).

    Returns:
        List of batches; each batch is a list of task dicts.
    """
    if not skill_tasks:
        return []

    file_to_tasks: dict[str, list[dict[str, object]]] = {}
    for task in skill_tasks:
        file_key = str(task["file"])
        file_to_tasks.setdefault(file_key, []).append(task)

    changed_files = list(file_to_tasks.keys())
    clusters = cluster_docs(changed_files, repo_root)
    clusters.sort(key=lambda c: -len(c))

    batches: list[list[dict[str, object]]] = []
    current: list[dict[str, object]] = []

    for cluster in clusters:
        cluster_tasks: list[dict[str, object]] = [
            task for file_path in cluster for task in file_to_tasks.get(file_path, [])
        ]
        if not cluster_tasks:
            continue

        if len(current) + len(cluster_tasks) <= batch_size:
            current.extend(cluster_tasks)
        else:
            if current:
                batches.append(current)
                current = []
            for task in cluster_tasks:
                current.append(task)
                if len(current) == batch_size:
                    batches.append(current)
                    current = []

    if current:
        batches.append(current)

    return batches


def reverse_links(
    changed_files: list[str],
    repo_root: Path,
) -> dict[str, list[str]]:
    """Return unchanged repo .md files that directly link to any of the changed files.

    Scans every .md file in the repo tree (excluding the changed files
    themselves and common non-source directories), resolves their links,
    and inverts the graph to show which unchanged docs reference each
    changed file.

    Args:
        changed_files: Repo-relative POSIX paths of changed files.
        repo_root: Absolute path to the repository root.

    Returns:
        Dict mapping each changed file to a sorted list of repo-relative
        paths of .md files that link to it. Only changed files that have
        at least one linker are present as keys.
    """
    if not changed_files:
        return {}

    changed: frozenset[str] = frozenset(changed_files)
    result: dict[str, list[str]] = {}

    for path in repo_root.rglob("*.md"):
        if any(part in _SCAN_EXCLUDE_DIRS for part in path.parts):
            continue
        if not path.is_file():
            continue
        try:
            rel_str = path.relative_to(repo_root).as_posix()
        except ValueError:
            continue
        if rel_str in changed:
            continue
        linked = _parse_md_links(path, repo_root, changed)
        for dep in linked:
            result.setdefault(dep, []).append(rel_str)

    return {dep: sorted(linkers) for dep, linkers in result.items()}


def build_corpus_index(repo_root: Path) -> list[dict[str, object]]:
    """Extract title, headings, key terms, and outgoing links from every .md file.

    Intended to be written as ``corpus-index.json`` in the staging directory
    so agents can identify semantically related docs without loading the full
    corpus into their context window.

    Args:
        repo_root: Absolute path to the repository root.

    Returns:
        List of entry dicts (one per .md file), sorted by file path. Each
        entry contains the keys ``file``, ``title``, ``headings``,
        ``key_terms``, and ``links_out``.
    """
    all_md_paths: dict[str, Path] = {}
    for path in sorted(repo_root.rglob("*.md")):
        if not path.is_file():
            continue
        if any(part in _SCAN_EXCLUDE_DIRS for part in path.parts):
            continue
        try:
            rel_str = path.relative_to(repo_root).as_posix()
        except ValueError:
            continue
        all_md_paths[rel_str] = path

    all_md: frozenset[str] = frozenset(all_md_paths)
    entries: list[dict[str, object]] = []

    for rel_str, path in all_md_paths.items():
        try:
            content = path.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        entries.append(
            {
                "file": rel_str,
                "title": _extract_title(content),
                "headings": _extract_headings(content),
                "key_terms": _extract_key_terms(content),
                "links_out": sorted(_parse_md_links(path, repo_root, all_md)),
            }
        )

    return entries
