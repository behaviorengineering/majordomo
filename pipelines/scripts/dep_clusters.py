"""Dependency-aware clustering for changed files.

Parses import statements from changed files (Python and JS/TS) and groups them
into connected components using a union-find structure, so that files that
import each other land in the same review batch.

Also provides reverse dependency lookup: given a set of changed files, scans
the full repo tree to find which unchanged files import any of them.

Files in unsupported languages form single-member clusters.

Stdlib-only — no third-party dependencies required.

Usage::

    from dep_clusters import cluster_files, cluster_aware_batches, reverse_deps
    from pathlib import Path

    clusters = cluster_files(changed_files, Path("/repo"))

    batches = cluster_aware_batches(skill_tasks, batch_size=15, repo_root=Path("/repo"))

    rev = reverse_deps(changed_files, Path("/repo"))
"""

from __future__ import annotations

import ast
import re
from pathlib import Path

# ---------------------------------------------------------------------------
# Language constants
# ---------------------------------------------------------------------------

_JS_EXTS: frozenset[str] = frozenset({".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"})

# Matches both ESM and CommonJS import specifiers:
#   import Foo from './foo'
#   import { X } from '../bar'
#   import * as ns from './ns'
#   const x = require('./x')
_JS_IMPORT_RE: re.Pattern[str] = re.compile(
    r"""(?:import\s+[\w*{}\s,]+\s+from\s+|require\s*\(\s*)['"]([^'"]+)['"]""",
    re.MULTILINE,
)


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
# Python import resolution
# ---------------------------------------------------------------------------


def _module_to_candidates(module_path: str, root: Path) -> list[Path]:
    """Return candidate file paths for a dotted module name.

    Args:
        module_path: Dotted module name (e.g. ``"foo.bar.baz"``).
        root: Repository root used as the resolution base.

    Returns:
        List of candidate absolute paths (may not exist).
    """
    parts = module_path.replace(".", "/")
    return [
        root / f"{parts}.py",
        root / parts / "__init__.py",
    ]


def _resolve_relative_import(
    level: int,
    module: str,
    current_file: Path,
    _root: Path,
) -> list[Path]:
    """Resolve a relative Python import to candidate file paths.

    Args:
        level: Number of leading dots (1 = same package, 2 = parent, …).
        module: Dotted tail after the dots (may be empty).
        current_file: Absolute path of the file containing the import.
        _root: Repository root (unused — resolution is purely relative).

    Returns:
        List of candidate absolute paths (may not exist).
    """
    package = current_file.parent
    for _ in range(level - 1):
        package = package.parent
    if not module:
        return [package / "__init__.py"]
    parts = module.replace(".", "/")
    return [
        package / f"{parts}.py",
        package / parts / "__init__.py",
    ]


def _parse_python_imports(
    path: Path,
    root: Path,
    changed: frozenset[str],
) -> set[str]:
    """Return the subset of *changed* directly imported by the Python file at *path*.

    Args:
        path: Absolute path to the Python source file.
        root: Repository root for resolving absolute imports.
        changed: Frozenset of repo-relative POSIX paths in the changed set.

    Returns:
        Subset of *changed* directly imported by this file.
    """
    try:
        tree = ast.parse(path.read_text(encoding="utf-8", errors="replace"))
    except SyntaxError:
        return set()

    found: set[str] = set()
    for node in ast.walk(tree):
        candidates: list[Path] = []
        if isinstance(node, ast.Import):
            for alias in node.names:
                candidates.extend(_module_to_candidates(alias.name, root))
        elif isinstance(node, ast.ImportFrom):
            module = node.module or ""
            level = node.level or 0
            if level > 0:
                candidates.extend(_resolve_relative_import(level, module, path, root))
            else:
                candidates.extend(_module_to_candidates(module, root))

        for candidate in candidates:
            try:
                rel = candidate.relative_to(root)
            except ValueError:
                continue
            rel_str = rel.as_posix()
            if rel_str in changed:
                found.add(rel_str)

    return found


# ---------------------------------------------------------------------------
# JS/TS import resolution
# ---------------------------------------------------------------------------


def _resolve_js_import(
    specifier: str,
    current_file: Path,
    _root: Path,
) -> list[Path]:
    """Resolve a JS/TS import specifier to candidate file paths.

    Only relative specifiers (starting with ``./`` or ``../``) are resolved.
    Third-party and bare specifiers are ignored.

    Args:
        specifier: Import path string from the source file.
        current_file: Absolute path of the importing file.
        _root: Repository root (unused — resolution is relative to current file).

    Returns:
        List of candidate absolute paths (may not exist).
    """
    if not specifier.startswith("."):
        return []
    base = (current_file.parent / specifier).resolve()
    return [
        base,
        base.with_suffix(".js"),
        base.with_suffix(".ts"),
        base.with_suffix(".tsx"),
        base.with_suffix(".jsx"),
        base / "index.js",
        base / "index.ts",
    ]


def _parse_js_imports(
    path: Path,
    root: Path,
    changed: frozenset[str],
) -> set[str]:
    """Return the subset of *changed* directly imported by the JS/TS file at *path*.

    Args:
        path: Absolute path to the JS/TS source file.
        root: Repository root for relativising resolved paths.
        changed: Frozenset of repo-relative POSIX paths in the changed set.

    Returns:
        Subset of *changed* directly imported by this file.
    """
    try:
        content = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return set()

    found: set[str] = set()
    for match in _JS_IMPORT_RE.finditer(content):
        specifier = match.group(1)
        for candidate in _resolve_js_import(specifier, path, root):
            try:
                rel = candidate.relative_to(root)
            except ValueError:
                continue
            rel_str = rel.as_posix()
            if rel_str in changed:
                found.add(rel_str)
    return found


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def cluster_files(
    changed_files: list[str],
    repo_root: Path,
) -> list[list[str]]:
    """Group changed files into dependency clusters.

    Files that import each other (directly or transitively within the changed
    set) are placed in the same cluster.  Files with no intra-PR import edges
    form single-member clusters.

    Python imports are resolved via the ``ast`` module (precise).  JS/TS
    imports are resolved with a regex covering ESM and CommonJS.  Files in
    other languages form single-member clusters.

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
        if not path.exists():
            continue
        suffix = path.suffix.lower()
        if suffix == ".py":
            neighbours = _parse_python_imports(path, repo_root, changed)
        elif suffix in _JS_EXTS:
            neighbours = _parse_js_imports(path, repo_root, changed)
        else:
            continue
        for neighbour in neighbours:
            uf.union(rel_str, neighbour)

    return uf.components()


def cluster_aware_batches(
    skill_tasks: list[dict[str, object]],
    batch_size: int,
    repo_root: Path,
) -> list[list[dict[str, object]]]:
    """Pack manifest tasks into batches that keep dependency clusters together.

    Files that import each other land in the same batch.  When a cluster
    exceeds *batch_size*, it is split into consecutive same-cluster batches
    (related files stay adjacent).  Unrelated clusters are bin-packed greedily.

    Args:
        skill_tasks: All manifest tasks for one skill (may include multiple
            tasks per file when the file was chunked).
        batch_size: Maximum number of tasks per batch.
        repo_root: Absolute path to the repository root (for import resolution).

    Returns:
        List of batches; each batch is a list of task dicts.
    """
    if not skill_tasks:
        return []

    # Map each unique file to its tasks (chunked files have multiple tasks).
    file_to_tasks: dict[str, list[dict[str, object]]] = {}
    for task in skill_tasks:
        file_key = str(task["file"])
        file_to_tasks.setdefault(file_key, []).append(task)

    changed_files = list(file_to_tasks.keys())
    clusters = cluster_files(changed_files, repo_root)

    # Largest clusters first — they are most likely to need their own batch
    # and should be scheduled before small/singleton clusters to avoid waste.
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
            # Whole cluster fits in the open batch.
            current.extend(cluster_tasks)
        else:
            # Flush open batch, then pack this cluster (split only within cluster).
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


# Directories to skip when scanning the full repo tree for reverse deps.
# These never contain reviewable source files and skipping them keeps the
# scan fast on large repos.
_SCAN_EXCLUDE_DIRS: frozenset[str] = frozenset({
    "node_modules", ".git", "__pycache__", ".venv", "venv",
    "dist", "build", ".tox", ".mypy_cache", ".pytest_cache",
    ".ruff_cache", ".eggs", "*.egg-info",
})


def reverse_deps(
    changed_files: list[str],
    repo_root: Path,
) -> dict[str, list[str]]:
    """Return unchanged repo files that directly import any of the changed files.

    Scans every ``.py`` and JS/TS file in the repo tree (excluding the changed
    files themselves and common non-source directories), resolves their imports
    using the same logic as ``cluster_files``, and inverts the graph to show
    which unchanged files reference each changed file.

    Args:
        changed_files: Repo-relative POSIX paths of changed files.
        repo_root: Absolute path to the repository root.

    Returns:
        Dict mapping each changed file to a sorted list of repo-relative paths
        of files that import it.  Only changed files that have at least one
        importer are present as keys.
    """
    if not changed_files:
        return {}

    changed: frozenset[str] = frozenset(changed_files)
    result: dict[str, list[str]] = {}

    for path in repo_root.rglob("*"):
        # Skip excluded directories — check every part of the path.
        if any(part in _SCAN_EXCLUDE_DIRS for part in path.parts):
            continue
        if not path.is_file():
            continue

        suffix = path.suffix.lower()
        if suffix != ".py" and suffix not in _JS_EXTS:
            continue

        try:
            rel_str = path.relative_to(repo_root).as_posix()
        except ValueError:
            continue

        # Only scan files outside the changed set — we want reverse edges
        # from unchanged callers pointing into the changed set.
        if rel_str in changed:
            continue

        if suffix == ".py":
            imported = _parse_python_imports(path, repo_root, changed)
        else:
            imported = _parse_js_imports(path, repo_root, changed)

        for dep in imported:
            result.setdefault(dep, []).append(rel_str)

    # Sort importer lists for deterministic output.
    return {dep: sorted(importers) for dep, importers in result.items()}
