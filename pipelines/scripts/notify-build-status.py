"""Notify Bitbucket Server of a build status via the commit-builds REST API.

Usage:
    python notify-build-status.py <commit_sha> <state>

Arguments:
    commit_sha  Git commit SHA to annotate.
    state       One of: INPROGRESS | SUCCESSFUL | FAILED

Required environment variables:
    BB_BASE_URL           Bitbucket Server base URL (e.g. https://bitbucket.example.com)
    BB_PROJECT_KEY        Bitbucket project key
                          (e.g. MYPROJECT or ~username for personal repos)
    BB_REPO_SLUG          Repository slug (e.g. my-repo)
    BITBUCKET_TOKEN       Personal access token with repository write permission
    BUILD_URL             Jenkins build URL
    BB_BUILD_KEY          Stable pipeline identity key (e.g. job name) — used as
                          ``parent``; per-run ``key`` becomes
                          ``BB_BUILD_KEY#BB_BUILD_NUMBER``.
    BB_BUILD_NAME         Human-readable build name (e.g. "ModuleCI #42")
    BB_BUILD_DESCRIPTION  Short status description (e.g. "CI pipeline running")

Optional environment variables:
    BB_BUILD_NUMBER       Build number for a unique per-run key (e.g. "42")
    BB_BUILD_REF          Git ref being built (e.g. refs/heads/feature-branch)

Exit codes:
    0  Notified successfully.
    1  Fatal error.
"""

from __future__ import annotations

import json
import os
import ssl
import sys
import urllib.error
import urllib.request
from datetime import datetime

# ---------------------------------------------------------------------------
# Logging — mirror the bash common.sh style for consistent CI output
# ---------------------------------------------------------------------------

_LOG_FMT = "[{ts}] [{level}] {msg}"
_SECTION_FMT = "[{ts}] [{level}] ========== {msg} =========="


def _ts() -> str:
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def log_info(msg: str) -> None:
    """Log an INFO message."""
    print(_LOG_FMT.format(ts=_ts(), level="INFO", msg=msg))


def log_error(msg: str) -> None:
    """Log an ERROR message to stderr."""
    print(_LOG_FMT.format(ts=_ts(), level="ERROR", msg=msg), file=sys.stderr)


def log_header(msg: str) -> None:
    """Log a section header."""
    print(_SECTION_FMT.format(ts=_ts(), level="INFO", msg=msg))


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

_VALID_STATES = {"INPROGRESS", "SUCCESSFUL", "FAILED"}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _require_env(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        log_error(f"{name} must be set")
        sys.exit(1)
    return value


# ---------------------------------------------------------------------------
# API
# ---------------------------------------------------------------------------


def post_build_status(
    base_url: str,
    project_key: str,
    repo_slug: str,
    token: str,
    commit_sha: str,
    state: str,
    key: str,
    parent: str,
    name: str,
    build_url: str,
    description: str,
    build_number: str = "",
    ref: str = "",
) -> None:
    """POST a build status to the Bitbucket Server commit-builds REST API.

    Uses the newer /rest/api/1.0/projects/.../repos/.../commits/.../builds endpoint
    which is required for the Bitbucket "required builds" merge check feature.
    The legacy /rest/build-status/1.0/commits/{sha} endpoint does not satisfy
    required-build merge checks even when the build result is visible in the UI.

    Args:
        base_url: Bitbucket Server base URL.
        project_key: Bitbucket project key (e.g. MYPROJECT or ~username).
        repo_slug: Repository slug.
        token: Personal access token.
        commit_sha: Git commit SHA to annotate.
        state: Build state — INPROGRESS, SUCCESSFUL, or FAILED.
        key: Per-run build key (e.g. ``{job_name}#{build_number}``).
        parent: Stable pipeline identity matched by required-build merge checks.
        name: Human-readable build name displayed in the Bitbucket UI.
        build_url: URL to the Jenkins build for drill-down.
        description: Short status description shown in the tooltip.
        build_number: Included as ``buildNumber`` in the payload when non-empty.
        ref: Git ref being built (e.g. ``refs/heads/main``); included when non-empty.

    Raises:
        SystemExit: On HTTP error or network failure.
    """
    api_url = (
        f"{base_url.rstrip('/')}/rest/api/1.0/projects/"
        f"{project_key}/repos/{repo_slug}/commits/{commit_sha}/builds"
    )
    payload_dict: dict[str, str] = {
        "state": state,
        "key": key,
        "parent": parent,
        "name": name,
        "url": build_url,
        "description": description,
    }
    if build_number:
        payload_dict["buildNumber"] = build_number
    if ref:
        payload_dict["ref"] = ref
    payload = json.dumps(payload_dict).encode()

    req = urllib.request.Request(
        api_url,
        data=payload,
        method="POST",
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
        },
    )

    log_info(f"POST {api_url}")
    ca_file = os.environ.get("REQUESTS_CA_BUNDLE") or os.environ.get("SSL_CERT_FILE")
    if ca_file and os.path.isfile(ca_file):
        ctx = ssl.create_default_context(cafile=ca_file)
    else:
        ctx = ssl.create_default_context()

    try:
        with urllib.request.urlopen(req, context=ctx) as resp:
            body = resp.read().decode(errors="replace")
            body_preview = body[:300] if body else "(empty)"
            log_info(f"Response: HTTP {resp.status} — {body_preview}")
    except urllib.error.HTTPError as exc:
        body_text = exc.read().decode(errors="replace")
        log_error(f"HTTP {exc.code} from {api_url}: {body_text}")
        sys.exit(1)
    except urllib.error.URLError as exc:
        reason = getattr(exc, "reason", exc)
        if isinstance(reason, ssl.SSLError):
            ca_hint = (
                "set REQUESTS_CA_BUNDLE or SSL_CERT_FILE to your corporate CA bundle"
            )
            log_error(
                f"SSL verification failed posting to {api_url}: {reason} — {ca_hint}"
            )
        else:
            log_error(f"Network error posting to {api_url}: {exc}")
        sys.exit(1)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


_EXPECTED_ARGC = 3


def main() -> None:
    """Parse args, validate inputs, and post the build status to Bitbucket."""
    if len(sys.argv) != _EXPECTED_ARGC:
        print(
            f"Usage: {sys.argv[0]} <commit_sha> <state>",
            file=sys.stderr,
        )
        sys.exit(1)

    commit_sha, state = sys.argv[1], sys.argv[2]

    if state not in _VALID_STATES:
        log_error(f"State must be one of {sorted(_VALID_STATES)}, got: {state!r}")
        sys.exit(1)

    base_url = _require_env("BB_BASE_URL")
    project_key = _require_env("BB_PROJECT_KEY")
    repo_slug = _require_env("BB_REPO_SLUG")
    token = _require_env("BITBUCKET_TOKEN")
    build_url = _require_env("BUILD_URL")
    build_key = _require_env("BB_BUILD_KEY")
    build_name = _require_env("BB_BUILD_NAME")
    build_description = _require_env("BB_BUILD_DESCRIPTION")
    build_number = os.environ.get("BB_BUILD_NUMBER", "").strip()
    build_ref = os.environ.get("BB_BUILD_REF", "").strip()

    # parent is the stable pipeline identity configured in required-build merge checks.
    # key is unique per run so each run produces a distinct record on the commit.
    parent_key = build_key
    run_key = f"{build_key}#{build_number}" if build_number else build_key

    log_header(f"Notifying Bitbucket build status: {state}")
    log_info(f"Commit:  {commit_sha}")
    log_info(f"State:   {state}")
    log_info(f"Key:     {run_key}")
    log_info(f"Parent:  {parent_key}")
    log_info(f"Name:    {build_name}")
    log_info(f"Ref:     {build_ref or '(not set)'}")
    log_info(f"Project: {project_key}")
    log_info(f"Repo:    {repo_slug}")

    post_build_status(
        base_url=base_url,
        project_key=project_key,
        repo_slug=repo_slug,
        token=token,
        commit_sha=commit_sha,
        state=state,
        key=run_key,
        parent=parent_key,
        name=build_name,
        build_url=build_url,
        description=build_description,
        build_number=build_number,
        ref=build_ref,
    )

    log_header("Build status notification complete")


if __name__ == "__main__":
    main()
