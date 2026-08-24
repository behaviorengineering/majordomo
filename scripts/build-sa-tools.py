"""Build SA tool Docker images locally to validate Dockerfiles before CI.

Usage (run from anywhere inside the repo):
    python .majordomo/scripts/build-sa-tools.py [--dry-run] [-v]

Required env vars:
    REGISTRY_USER                package registry username (email or bare name — domain is stripped automatically)
    ARTIFACTORY_ACCESS_TOKEN  package registry token

Exit code: 0 if all images built successfully, 1 if any failed.
"""

from __future__ import annotations

import inspect
import os
import shlex
import subprocess
import sys
from pathlib import Path

_SA_TOOLS_DIR = Path(".majordomo/dockerfiles/sa-tools")
_REGISTRY = "example-docker-snapshot-dependencies.packages.example.com"


def _workspace_root() -> Path:
    """Return the repository workspace root (parent of the submodule directory).

    This script lives inside the submodule at .majordomo/scripts/, so
    the workspace root is two levels up — the parent repo that contains the
    submodule and provides the Podman build context.

    Returns:
        Absolute path to the workspace root.
    """
    return Path(__file__).parent.parent.parent.resolve()


def _discover_dockerfiles(sa_tools_dir: Path) -> list[Path]:
    """Find all Dockerfile files inside the SA tools directory.

    Args:
        sa_tools_dir: Absolute path to the sa-tools directory.

    Returns:
        Sorted list of absolute Dockerfile paths.
    """
    return sorted(sa_tools_dir.glob("*.Dockerfile"))


def _tool_name(dockerfile: Path) -> str:
    """Derive the SA tool name from its Dockerfile path.

    Args:
        dockerfile: Path to the Dockerfile.

    Returns:
        Tool name string, e.g. 'ruff' from 'ruff.Dockerfile'.
    """
    return dockerfile.stem


def _image_tag(tool: str) -> str:
    """Compose a local test image tag for the given SA tool.

    Args:
        tool: SA tool name.

    Returns:
        Image tag string suitable for local use only.
    """
    return f"sa-{tool}:local-test"


def _write_wsl_secret(name: str, value: str) -> str:
    """Write a secret value to a WSL-native temp file accessible by Podman.

    Windows temp paths are not reachable by Podman's WSL backend when using
    --secret src=. Writing via 'wsl -- sh -c' creates the file in the Linux
    filesystem where Podman inside WSL can read it directly.

    Args:
        name: Secret identifier, used as part of the filename.
        value: Secret value to write.

    Returns:
        Linux path to the created secret file.
    """
    linux_path = f"/tmp/sa-secret-{name}"
    subprocess.run(
        ["wsl", "--", "sh", "-c", f"printf '%s' '{value}' > {linux_path} && chmod 600 {linux_path}"],
        check=True,
        capture_output=True,
    )
    result = subprocess.run(
        ["wsl", "--", "test", "-f", linux_path],
        check=True,
        capture_output=True,
    )
    return linux_path


def _delete_wsl_secret(name: str) -> None:
    """Remove a WSL secret file.

    Args:
        name: Secret identifier used when creating the file.
    """
    subprocess.run(
        ["wsl", "--", "rm", "-f", f"/tmp/sa-secret-{name}"],
        capture_output=True,
    )


def _to_wsl_path(path: Path) -> str:
    """Convert a Windows path to an absolute WSL path.

    Args:
        path: Windows path.

    Returns:
        Absolute Linux path in WSL, e.g. /mnt/c/....
    """
    path_str = str(path.resolve()).replace("\\", "/")
    if len(path_str) < 2 or path_str[1] != ":":
        return path_str
    drive = path_str[0].lower()
    remainder = path_str[2:]
    return f"/mnt/{drive}{remainder}"


def _run_build(
    dockerfile: Path,
    workspace_root: Path,
    username_path: str,
    token_path: str,
    tag: str,
) -> tuple[bool, list[str]]:
    """Execute podman build for one SA tool Dockerfile.

    Build context is the workspace root so bind-mounted helper files
    (e.g. setup-corp-apt.sh) resolve via their relative source paths.

    The build is executed via Podman inside WSL. This avoids the Windows
    Podman client bug where --secret src= can fail with
    "podman-build-secret... no such file or directory".

    Secrets are passed as src= Linux paths under /tmp.

    A temporary .dockerignore is written before the build and removed after so
    that Podman only tars .majordomo/ — avoids 'archive/tar: write too
    long' errors from long Windows paths in the rest of the workspace.

    Args:
        dockerfile: Absolute path to the Dockerfile.
        workspace_root: Absolute path to the workspace root.
        username_path: WSL Linux path to the username secret file.
        token_path: WSL Linux path to the token secret file.
        tag: Image tag to apply on success.

    Returns:
        Tuple of (success, output_lines).
    """
    ignorefile = workspace_root / ".dockerignore-sa-test"
    ignorefile.write_text("*\n!.majordomo/**\n", encoding="utf-8")
    try:
        workspace_wsl = _to_wsl_path(workspace_root)
        dockerfile_wsl = _to_wsl_path(dockerfile)
        ignorefile_wsl = _to_wsl_path(ignorefile)
        cmd_str = (
            "unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy ALL_PROXY all_proxy && "
            f"cd {shlex.quote(workspace_wsl)} && "
            f"podman build "
            f"--file {shlex.quote(dockerfile_wsl)} "
            f"--ignorefile {shlex.quote(ignorefile_wsl)} "
            f"--secret id=username,src={shlex.quote(username_path)} "
            f"--secret id=token,src={shlex.quote(token_path)} "
            f"--tag {shlex.quote(tag)} ."
        )
        result = subprocess.run(
            ["wsl", "--", "sh", "-lc", cmd_str],
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
        )
    finally:
        ignorefile.unlink(missing_ok=True)
    stdout_text = result.stdout or ""
    stderr_text = result.stderr or ""
    output = (stdout_text + stderr_text).splitlines()
    return result.returncode == 0, output


def _wsl_podman_login(registry: str, username: str, password: str) -> None:
    """Log in to a registry using Podman inside WSL.

    Args:
        registry: Registry hostname.
        username: Registry username.
        password: Registry password or access token.
    """
    subprocess.run(
        [
            "wsl",
            "--",
            "podman",
            "login",
            "--username",
            username,
            "--password-stdin",
            registry,
        ],
        input=password,
        text=True,
        capture_output=True,
        check=True,
    )


def _print_result(
    tool: str,
    success: bool,
    output: list[str],
    *,
    verbose: bool,
) -> None:
    """Print the build result for one SA tool.

    Args:
        tool: SA tool name.
        success: Whether the build succeeded (exit code 0).
        output: Build output lines.
        verbose: When True, always print full output.
    """
    status = "PASS" if success else "FAIL"
    marker = "\u2713" if success else "\u2717"
    print(f"  {marker} sa-{tool}: {status}")
    if not success or verbose:
        for line in output:
            print(f"      {line}")


def main() -> None:
    """Discover and build all SA tool Dockerfiles, then report results.

    Reads REGISTRY_USER and ARTIFACTORY_ACCESS_TOKEN from environment variables.
    """
    dry_run = "--dry-run" in sys.argv
    verbose = "--verbose" in sys.argv or "-v" in sys.argv

    username = os.environ.get("REGISTRY_USER", "").split("@")[0]
    token = os.environ.get("ARTIFACTORY_ACCESS_TOKEN", "")

    if not dry_run and (not username or not token):
        missing = [v for v, val in (("REGISTRY_USER", username), ("ARTIFACTORY_ACCESS_TOKEN", token)) if not val]
        missing_str = ", ".join(missing)
        print(f"Error: required env vars not set: {missing_str}")
        print("Set REGISTRY_USER and ARTIFACTORY_ACCESS_TOKEN before running.")
        sys.exit(1)

    mode = "full build"

    workspace_root = _workspace_root()
    sa_tools_dir = workspace_root / _SA_TOOLS_DIR
    dockerfiles = _discover_dockerfiles(sa_tools_dir)

    if not dockerfiles:
        print(f"No Dockerfiles found in {sa_tools_dir}")
        sys.exit(1)

    tools_str = ", ".join(_tool_name(d) for d in dockerfiles)
    header = inspect.cleandoc(f"""
        SA Tool Image Builder
        Mode:      {mode}
        Context:   {workspace_root}
        Tools:     {tools_str}
        Dry-run:   {dry_run}
    """)
    print(header)
    print()

    if dry_run:
        for dockerfile in dockerfiles:
            tool = _tool_name(dockerfile)
            print(f"  [dry-run] would build sa-{tool} from {dockerfile}")
        sys.exit(0)

    try:
        _wsl_podman_login(_REGISTRY, username, token)
    except subprocess.CalledProcessError as err:
        stderr = err.stderr.strip() if err.stderr else ""
        print("Error: failed to log in WSL Podman to package registry registry.")
        if stderr:
            print(stderr)
        sys.exit(1)

    results: dict[str, bool] = {}

    username_path = _write_wsl_secret("username", username)
    token_path = _write_wsl_secret("token", token)
    try:
        for dockerfile in dockerfiles:
            tool = _tool_name(dockerfile)
            tag = _image_tag(tool)
            print(f"Building {tag} ...")
            success, output = _run_build(
                dockerfile, workspace_root, username_path, token_path, tag
            )
            results[tool] = success
            _print_result(tool, success, output, verbose=verbose)
    finally:
        _delete_wsl_secret("username")
        _delete_wsl_secret("token")

    print()
    passed = sum(1 for ok in results.values() if ok)
    total = len(results)
    result_lines = "\n".join(
        f"  {'PASS' if ok else 'FAIL'}  sa-{t}" for t, ok in sorted(results.items())
    )
    summary = inspect.cleandoc(f"""
        Results: {passed}/{total} passed
        {result_lines}
    """)
    print(summary)

    if passed < total:
        sys.exit(1)


if __name__ == "__main__":
    main()
