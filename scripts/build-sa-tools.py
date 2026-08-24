"""Build SA tool Docker images locally to validate Dockerfiles before CI.

Usage (run from the majordomo repo root, or a parent that vendors .majordomo/):
    python scripts/build-sa-tools.py [--dry-run] [-v] [--corp]

Modes:
    public (default) — Hub + public indexes; no registry secrets (GitHub Actions).
    --corp           — corporate package registry; requires PACKAGE_REGISTRY_* env
                       and REGISTRY_USER + REGISTRY_TOKEN
                       (and typically DOCKER_PULL_DOMAIN).

Exit code: 0 if all images built successfully, 1 if any failed.
"""

from __future__ import annotations

import inspect
import os
import shlex
import subprocess
import sys
from pathlib import Path

_SCRIPT_PATH = Path(__file__).resolve()
# scripts/build-sa-tools.py → repo root is parent of scripts/
_REPO_ROOT = _SCRIPT_PATH.parent.parent
_SA_TOOLS_DIR = _REPO_ROOT / "dockerfiles" / "sa-tools"
# When this repo is vendored as .majordomo/, also accept that layout for discovery.
_VENDORED_SA_TOOLS = _REPO_ROOT.parent / ".majordomo" / "dockerfiles" / "sa-tools"


def _sa_tools_dir() -> Path:
    """Return the sa-tools directory for this checkout."""
    if _SA_TOOLS_DIR.is_dir():
        return _SA_TOOLS_DIR
    if _VENDORED_SA_TOOLS.is_dir():
        return _VENDORED_SA_TOOLS
    return _SA_TOOLS_DIR


def _workspace_root() -> Path:
    """Return the directory used as the process cwd for builds.

    For a direct majordomo checkout this is the repo root. For a vendored
    `.majordomo/` submodule layout, prefer the parent workspace when present.
    """
    if (_REPO_ROOT / "dockerfiles" / "sa-tools").is_dir():
        return _REPO_ROOT
    parent = _REPO_ROOT.parent
    if (parent / ".majordomo" / "dockerfiles" / "sa-tools").is_dir():
        return parent
    return _REPO_ROOT


def _discover_dockerfiles(sa_tools_dir: Path) -> list[Path]:
    """Find all Dockerfile files inside the SA tools directory."""
    return sorted(sa_tools_dir.glob("*.Dockerfile"))


def _tool_name(dockerfile: Path) -> str:
    """Derive the SA tool name from its Dockerfile path."""
    return dockerfile.stem


def _image_tag(tool: str) -> str:
    """Compose a local test image tag for the given SA tool."""
    return f"sa-{tool}:local-test"


def _build_script() -> Path:
    """Path to build-copilot-image.sh (majordomo or vendored)."""
    candidates = [
        _REPO_ROOT / "pipelines" / "scripts" / "build-copilot-image.sh",
        _workspace_root() / ".majordomo" / "pipelines" / "scripts" / "build-copilot-image.sh",
        _workspace_root() / "pipelines" / "scripts" / "build-copilot-image.sh",
    ]
    for path in candidates:
        if path.is_file():
            return path
    return candidates[0]


def _run_build(
    dockerfile: Path,
    workspace_root: Path,
    tag: str,
    *,
    corp: bool,
) -> tuple[bool, list[str]]:
    """Build one SA tool image via build-copilot-image.sh."""
    build_sh = _build_script()
    tool = _tool_name(dockerfile)
    # Prefer path relative to workspace so the shell script's sa-tools detection works.
    try:
        dockerfile_arg = str(dockerfile.relative_to(workspace_root))
    except ValueError:
        dockerfile_arg = str(dockerfile)

    env = os.environ.copy()
    env["DOCKER_BUILD_TARGET"] = "corp" if corp else "public"
    env["SKIP_PUSH"] = "true"
    if not corp:
        # Ensure accidental corp env on the developer machine does not flip the target.
        env.pop("PACKAGE_REGISTRY_HOST", None)

    cmd = [
        "bash",
        str(build_sh),
        "local",
        f"sa-{tool}",
        "local-test",
        dockerfile_arg,
    ]
    result = subprocess.run(
        cmd,
        cwd=str(workspace_root),
        env=env,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    output = ((result.stdout or "") + (result.stderr or "")).splitlines()
    # Retag to the historical local name if the script used local/sa-tool:local-test
    if result.returncode == 0:
        full = f"local/sa-{tool}:local-test"
        subprocess.run(
            ["docker", "tag", full, tag],
            capture_output=True,
            check=False,
        )
    return result.returncode == 0, output


def _print_result(
    tool: str,
    success: bool,
    output: list[str],
    *,
    verbose: bool,
) -> None:
    """Print the build result for one SA tool."""
    status = "PASS" if success else "FAIL"
    marker = "\u2713" if success else "\u2717"
    print(f"  {marker} sa-{tool}: {status}")
    if not success or verbose:
        for line in output:
            print(f"      {line}")


def main() -> None:
    """Discover and build all SA tool Dockerfiles, then report results."""
    dry_run = "--dry-run" in sys.argv
    verbose = "--verbose" in sys.argv or "-v" in sys.argv
    corp = "--corp" in sys.argv

    if corp:
        if not dry_run and (
            not os.environ.get("REGISTRY_USER")
            or not os.environ.get("REGISTRY_TOKEN")
            or not os.environ.get("PACKAGE_REGISTRY_HOST")
        ):
            print(
                "Error: --corp requires PACKAGE_REGISTRY_HOST, REGISTRY_USER, and REGISTRY_TOKEN."
            )
            sys.exit(1)

    mode = "corp" if corp else "public"
    workspace_root = _workspace_root()
    sa_tools_dir = _sa_tools_dir()
    dockerfiles = _discover_dockerfiles(sa_tools_dir)

    if not dockerfiles:
        print(f"No Dockerfiles found in {sa_tools_dir}")
        sys.exit(1)

    tools_str = ", ".join(_tool_name(d) for d in dockerfiles)
    header = inspect.cleandoc(
        f"""
        SA Tool Image Builder
        Mode:      {mode}
        Context:   {workspace_root}
        Tools:     {tools_str}
        Dry-run:   {dry_run}
        """
    )
    print(header)
    print()

    if dry_run:
        for dockerfile in dockerfiles:
            tool = _tool_name(dockerfile)
            print(f"  [dry-run] would build sa-{tool} ({mode}) from {dockerfile}")
        sys.exit(0)

    results: dict[str, bool] = {}
    for dockerfile in dockerfiles:
        tool = _tool_name(dockerfile)
        tag = _image_tag(tool)
        print(f"Building {tag} ({mode}) ...")
        success, output = _run_build(dockerfile, workspace_root, tag, corp=corp)
        results[tool] = success
        _print_result(tool, success, output, verbose=verbose)

    print()
    passed = sum(1 for ok in results.values() if ok)
    total = len(results)
    result_lines = "\n".join(
        f"  {'PASS' if ok else 'FAIL'}  sa-{t}" for t, ok in sorted(results.items())
    )
    summary = inspect.cleandoc(
        f"""
        Results: {passed}/{total} passed
        {result_lines}
        """
    )
    print(summary)

    if passed < total:
        sys.exit(1)


if __name__ == "__main__":
    main()
