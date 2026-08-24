"""Publish a Copilot PR summary to Bitbucket Server via REST API.

Usage:
    python publish-pr-summary.py <pr_number> <summary_file> <mode>

Arguments:
    pr_number     Bitbucket PR number.
    summary_file  Path to the summary.md file to publish.
    mode          One of: auto | comment | description

Modes:
    auto          Claim the PR description if it is empty or was previously set
                  by this tool (detected via an HTML comment marker); otherwise
                  post a link comment pointing to the summary artifact.
    comment       Always post the full summary content as a PR comment.
    description   Always replace the PR description with the summary content.

Required environment variables:
    BITBUCKET_URL        Bitbucket Server base URL (e.g. https://bitbucket.example.com)
    BITBUCKET_TOKEN      Personal access token with repository write permission
    BB_PROJECT           Bitbucket project key (e.g. MYPROJ)
    BB_REPO              Bitbucket repository slug (e.g. my-repo)

Required for auto mode when the description is already owned by someone else:
    SUMMARY_ARTIFACT_URL      Summary artifact URL (used in link comment fallback)

Optional:
    SUMMARY_HTML_ARTIFACT_URL  Summary HTML URL; appended as a link to all messages when set.
    TECH_REVIEW_ARTIFACT_URL   Tech-review HTML URL; appended as a link to all messages when set.
    TECH_DEEP_ARTIFACT_URL     Tech-review-deep HTML URL; appended as a link to all messages when set.
    SA_ARTIFACT_URLS           JSON array of {"slug": str, "url": str} for each SA tool that ran;
                               each entry appended as a [ 🔬 slug ](url) link.

Exit codes:
    0  Published successfully.
    1  Fatal error.
"""

from __future__ import annotations

import json
import os
import re
import sys
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path

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
# Bitbucket REST client
# ---------------------------------------------------------------------------

COPILOT_MARKER = "<!-- copilot-review -->"
_VALID_MODES = {"auto", "comment", "description"}


class BitbucketClient:
    """Minimal Bitbucket Server REST client using only stdlib."""

    def __init__(
        self, base_url: str, token: str, project: str, repo: str, pr: str
    ) -> None:
        """Initialise the client.

        Args:
            base_url: Bitbucket Server base URL.
            token: Personal access token.
            project: Bitbucket project key.
            repo: Repository slug.
            pr: Pull-request number.
        """
        self._token = token
        self._pr_url = (
            f"{base_url.rstrip('/')}/rest/api/1.0"
            f"/projects/{project}/repos/{repo}/pull-requests/{pr}"
        )

    def _request(
        self,
        method: str,
        url: str,
        body: dict | None = None,
    ) -> dict:
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(
            url,
            data=data,
            method=method,
            headers={
                "Authorization": f"Bearer {self._token}",
                "Content-Type": "application/json",
            },
        )
        try:
            with urllib.request.urlopen(req) as resp:
                return json.loads(resp.read().decode())
        except urllib.error.HTTPError as exc:
            body_text = exc.read().decode(errors="replace")
            log_error(f"HTTP {exc.code} from {url}: {body_text}")
            sys.exit(1)

    def get_pr(self) -> dict:
        """Fetch current PR metadata."""
        log_info("Fetching current PR metadata...")
        return self._request("GET", self._pr_url)

    def put_description(self, pr_meta: dict, description: str) -> None:
        """Replace the PR description using an optimistic-lock PUT.

        Args:
            pr_meta: Current PR metadata dict (from get_pr).
            description: New description text.
        """
        payload = {
            "version": pr_meta["version"],
            "title": pr_meta["title"],
            "description": description,
            "toRef": pr_meta["toRef"],
            "reviewers": [{"user": r["user"]} for r in pr_meta.get("reviewers", [])],
        }
        log_info(f"PR version: {pr_meta['version']}, title: {pr_meta['title']}")
        self._request("PUT", self._pr_url, payload)
        log_info("PR description updated successfully")

    def post_comment(self, text: str) -> None:
        """Post a comment on the PR.

        Args:
            text: Comment body text.
        """
        self._request("POST", f"{self._pr_url}/comments", {"text": text})
        log_info("Comment posted successfully")


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _require_env(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        log_error(f"{name} must be set")
        sys.exit(1)
    return value


def _summary_html_url() -> str:
    """Return the summary HTML artifact URL if set, else empty string."""
    return os.environ.get("SUMMARY_HTML_ARTIFACT_URL", "").strip()


def _tech_review_url() -> str:
    """Return the tech-review HTML artifact URL if set, else empty string."""
    return os.environ.get("TECH_REVIEW_ARTIFACT_URL", "").strip()


def _tech_deep_url() -> str:
    """Return the tech-review-deep HTML artifact URL if set, else empty string."""
    return os.environ.get("TECH_DEEP_ARTIFACT_URL", "").strip()


def _sa_artifact_urls() -> list[dict]:
    """Return list of {slug, url} dicts for SA tool artifacts if SA_ARTIFACT_URLS is set."""
    raw = os.environ.get("SA_ARTIFACT_URLS", "").strip()
    if not raw:
        return []
    try:
        return json.loads(raw)
    except (json.JSONDecodeError, TypeError):
        return []


def _build_review_links(summary_fallback_url: str = "") -> list[str]:
    """Build ordered list of review link strings from available artifact URLs."""
    links: list[str] = []
    summary_url = _summary_html_url() or summary_fallback_url
    if summary_url:
        links.append(f"[ 👁 PR Summary ]({summary_url})")
    tech_url = _tech_review_url()
    if tech_url:
        links.append(f"[ 🔍 Technical Review ]({tech_url})")
    tech_deep_url = _tech_deep_url()
    if tech_deep_url:
        links.append(f"[ 🧪 Technical Deep Review ]({tech_deep_url})")
    for sa in _sa_artifact_urls():
        slug = sa.get("slug", "")
        url = sa.get("url", "")
        if slug and url:
            links.append(f"[ 🔬 {slug} ]({url})")
    return links


def _with_review_links(text: str) -> str:
    """Append a review-links footer to *text* for any artifact URLs that are set."""
    links = _build_review_links()
    if not links:
        return text
    return f"{text}\n\n---\n{' · '.join(links)}"


def _has_body_content(summary: str) -> bool:
    """Return True if the summary contains non-heading, non-blank, non-comment lines."""
    for line in summary.splitlines():
        stripped = line.strip()
        if not stripped:
            continue
        if stripped.startswith("#"):
            continue
        if re.match(r"^<!--.*-->$", stripped):
            continue
        return True
    return False


# ---------------------------------------------------------------------------
# Mode handlers
# ---------------------------------------------------------------------------


def _run_auto(client: BitbucketClient, summary: str) -> None:
    pr_meta = client.get_pr()
    current_desc = pr_meta.get("description") or ""

    if not current_desc:
        log_info("PR description is empty — claiming it with the review summary")
        client.put_description(pr_meta, f"{COPILOT_MARKER}\n{_with_review_links(summary)}")

    elif COPILOT_MARKER in current_desc:
        log_info("PR description is owned by a previous review run — updating it")
        client.put_description(pr_meta, f"{COPILOT_MARKER}\n{_with_review_links(summary)}")

    else:
        log_info("PR description has user content — posting a link comment")
        artifact_url = _require_env("SUMMARY_ARTIFACT_URL")
        log_info(f"Artifact URL: {artifact_url}")
        links_list = _build_review_links(summary_fallback_url=artifact_url)
        client.post_comment(f"🤖 **Copilot PR Review complete** — {' · '.join(links_list)}")


def _run_description(client: BitbucketClient, summary: str) -> None:
    pr_meta = client.get_pr()
    log_info("Writing summary to PR description...")
    client.put_description(pr_meta, f"{COPILOT_MARKER}\n{_with_review_links(summary)}")


def _run_comment(client: BitbucketClient, summary: str) -> None:
    log_info("Posting full summary as PR comment...")
    client.post_comment(_with_review_links(summary))


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def main() -> None:
    """Parse args, validate inputs, and dispatch to the requested mode."""
    if len(sys.argv) != 4:
        print(
            f"Usage: {sys.argv[0]} <pr_number> <summary_file> <mode>",
            file=sys.stderr,
        )
        sys.exit(1)

    pr_number, summary_file_arg, mode = sys.argv[1], sys.argv[2], sys.argv[3]

    if mode not in _VALID_MODES:
        log_error(f"Mode must be one of {sorted(_VALID_MODES)}, got: {mode!r}")
        sys.exit(1)

    summary_path = Path(summary_file_arg)
    if not summary_path.is_file():
        log_error(f"Summary file not found: {summary_path}")
        sys.exit(1)

    summary = summary_path.read_text(encoding="utf-8")

    if not _has_body_content(summary):
        log_info("summary.md contains no body content — skipping publish")
        sys.exit(0)

    client = BitbucketClient(
        base_url=_require_env("BITBUCKET_URL"),
        token=_require_env("BITBUCKET_TOKEN"),
        project=_require_env("BB_PROJECT"),
        repo=_require_env("BB_REPO"),
        pr=pr_number,
    )

    log_header(f"Publishing PR summary to Bitbucket (mode: {mode})")
    log_info(f"PR:       #{pr_number}")
    log_info(f"Source:   {summary_path}")
    log_info(f"Project:  {os.environ.get('BB_PROJECT', '')}")
    log_info(f"Repo:     {os.environ.get('BB_REPO', '')}")

    match mode:
        case "auto":
            _run_auto(client, summary)
        case "description":
            _run_description(client, summary)
        case "comment":
            _run_comment(client, summary)

    log_header("Publish PR summary complete")


if __name__ == "__main__":
    main()
