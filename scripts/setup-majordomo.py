#!/usr/bin/env python3
"""Jenkins pipeline setup and validation for majordomo.

Parses .majordomo-config.groovy, verifies each credential ID exists
in the Jenkins global credential store, and creates the pipeline job if it
does not yet exist.

Usage:
    python scripts/setup-majordomo.py --validate-only
    python scripts/setup-majordomo.py \\
        --jenkins-url https://jenkins.example.com \\
        --username alice \\
        --api-token <token> \\
        --job-name copilot-pr-review \\
        --repo-url ssh://git@bitbucket.example.com/project/repo.git \\
        --create-job

The API token can also be supplied via the JENKINS_API_TOKEN environment
variable instead of --api-token. The username defaults to the REGISTRY_USER
environment variable if --username is omitted.
"""

from __future__ import annotations

import argparse
import base64
import inspect
import io
import json
import os
import re
import socket
import subprocess
import sys

# Ensure stdout/stderr can emit Unicode (emoji) on Windows cp1252 consoles.
if isinstance(sys.stdout, io.TextIOWrapper):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
if isinstance(sys.stderr, io.TextIOWrapper):
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable

_DEFAULT_CONFIG = Path(".majordomo-config.groovy")
_SCRIPT_PATH = ".majordomo/pipelines/MajordomoReview.CI.Jenkinsfile"
_DEFAULT_BRANCH = "*/pipelines"
_DEFAULT_JENKINS_URL = "https://jenkins.example.com"
_HTTP_NOT_FOUND = 404
_HTTPS_PORT = 443
_CONNECT_TIMEOUT = 5.0

# Expected Jenkins credential type for each config key.
_CRED_EXPECTED_TYPES: dict[str, str] = {
    "registry.credentialsId": "Username with password",
    "credentials.package-registryCredentialsId": "Username with password",
    "credentials.githubCopilotCredentialsId": "Secret text",
    "credentials.bitbucketTokenCredentialsId": "Secret text",
    "credentials.bitbucketSshCredentialsId": "SSH Username with private key",
    "credentials.gwtTokenCredentialsId": "Secret text",
    "credentials.prmTokenCredentialsId": "Secret text",
    "--scm-credentials-id": "SSH Username with private key",
}

# GWT variable extractions: (param_name, jsonpath_expression, description)
_GWT_VARS: list[tuple[str, str, str]] = [
    ("CHANGE_ID", "$.pullRequest.id", "PR number"),
    ("CHANGE_TARGET", "$.pullRequest.toRef.displayId", "PR target branch"),
    ("CHANGE_BRANCH", "$.pullRequest.fromRef.displayId", "PR source branch"),
    ("CHANGE_COMMIT", "$.pullRequest.fromRef.latestCommit", "PR source branch HEAD"),
    ("PUSH_BRANCH", "$.changes[0].ref.displayId", "Pushed branch (repo:refs_changed)"),
    ("PR_EVENT_TYPE", "$.eventKey", "Bitbucket event key"),
    ("ACTOR_NAME", "$.actor.displayName", "Bitbucket actor name"),
]

# Pipeline string parameters: (name, default, description)
_STRING_PARAMS: list[tuple[str, str, str]] = [
    ("CHANGE_ID", "", "PR number — pr:opened / pr:from_ref_updated ($.pullRequest.id)"),
    ("CHANGE_TARGET", "master", "PR target branch ($.pullRequest.toRef.displayId)"),
    ("CHANGE_BRANCH", "", "PR source branch ($.pullRequest.fromRef.displayId)"),
    ("CHANGE_COMMIT", "", "PR HEAD commit ($.pullRequest.fromRef.latestCommit)"),
    ("PUSH_BRANCH", "", "Pushed branch — repo:refs_changed ($.changes[0].ref.id)"),
    ("PR_EVENT_TYPE", "", "Bitbucket event key ($.eventKey)"),
    ("REPLAY_OF_BUILD", "", "Internal: source build number for replay reruns"),
]

# Pipeline choice parameters: (name, ordered_choices, description)
_CHOICE_PARAMS: list[tuple[str, list[str], str]] = [
    (
        "SUMMARY_PUBLISH_MODE",
        ["auto", "comment", "description", "off"],
        "How to publish the PR summary to Bitbucket",
    ),
]


@dataclass
class RegistryConfig:
    """Docker registry settings from .majordomo-config.groovy."""

    pull_domain: str = ""
    pull_url: str = ""
    push_domain: str = ""
    credentials_id: str = ""


@dataclass
class CredentialsConfig:
    """Jenkins credential IDs from .majordomo-config.groovy."""

    package-registry_credentials_id: str = ""
    github_copilot_credentials_id: str = ""
    bitbucket_token_credentials_id: str = ""
    bitbucket_ssh_credentials_id: str = ""
    gwt_token_credentials_id: str = ""
    prm_token_credentials_id: str = ""


@dataclass
class PipelineConfig:
    """Parsed representation of .majordomo-config.groovy."""

    registry: RegistryConfig = field(default_factory=RegistryConfig)
    credentials: CredentialsConfig = field(default_factory=CredentialsConfig)


@dataclass
class ValidationResult:
    """Outcome of config field validation."""

    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)

    @property
    def ok(self) -> bool:
        """True when no errors were found."""
        return not self.errors


@dataclass
class JobSetupConfig:
    """Parameters controlling Jenkins job creation."""

    job_name: str
    folder: str
    repo_url: str
    pipeline_branch: str
    create_if_missing: bool
    update_if_exists: bool
    scm_credentials_id: str
    gwt_credentials_id: str
    prm_token_credentials_id: str = ""
    dump_xml_path: str = ""


# ---------------------------------------------------------------------------
# Config parsing
# ---------------------------------------------------------------------------


def _extract_block(source: str, key: str) -> str:
    """Return the bracketed body of a named Groovy map entry.

    Args:
        source: Groovy source text (comments already stripped).
        key: Map entry name (e.g. ``registry``).

    Returns:
        Text between the brackets, or an empty string if not found.
    """
    match = re.search(rf"{re.escape(key)}\s*:\s*\[", source)
    if not match:
        return ""
    start = match.end()
    depth = 1
    pos = start
    while pos < len(source) and depth > 0:
        if source[pos] == "[":
            depth += 1
        elif source[pos] == "]":
            depth -= 1
        pos += 1
    return source[start : pos - 1]


def _extract_string(block: str, key: str) -> str:
    """Return the string value for a key in a Groovy map block.

    Args:
        block: Groovy map block text.
        key: Groovy map key name.

    Returns:
        Extracted value, or an empty string if not found.
    """
    match = re.search(rf"""{re.escape(key)}\s*:\s*['"]([^'"\\]*)['"]""", block)
    return match.group(1) if match else ""


def _strip_groovy_line_comments(source: str) -> str:
    """Strip // comments while preserving // inside quoted strings.

    Args:
        source: Groovy source text.

    Returns:
        Source with line comments removed.
    """
    out_lines: list[str] = []
    for line in source.splitlines():
        in_single = False
        in_double = False
        escaped = False
        idx = 0
        while idx < len(line):
            ch = line[idx]
            if escaped:
                escaped = False
                idx += 1
                continue
            if ch == "\\":
                escaped = True
                idx += 1
                continue
            if ch == "'" and not in_double:
                in_single = not in_single
                idx += 1
                continue
            if ch == '"' and not in_single:
                in_double = not in_double
                idx += 1
                continue
            if (
                not in_single
                and not in_double
                and ch == "/"
                and idx + 1 < len(line)
                and line[idx + 1] == "/"
            ):
                line = line[:idx]
                break
            idx += 1
        out_lines.append(line)
    return "\n".join(out_lines)


def parse_config(config_path: Path) -> PipelineConfig:
    """Parse .majordomo-config.groovy into a PipelineConfig.

    Args:
        config_path: Path to the Groovy config file.

    Returns:
        PipelineConfig populated with values from the file.

    Raises:
        FileNotFoundError: If the path does not exist.
    """
    source = config_path.read_text(encoding="utf-8")
    source = _strip_groovy_line_comments(source)
    reg = _extract_block(source, "registry")
    cred = _extract_block(source, "credentials")
    return PipelineConfig(
        registry=RegistryConfig(
            pull_domain=_extract_string(reg, "pullDomain"),
            pull_url=_extract_string(reg, "pullUrl"),
            push_domain=_extract_string(reg, "pushDomain"),
            credentials_id=_extract_string(reg, "credentialsId"),
        ),
        credentials=CredentialsConfig(
            package-registry_credentials_id=_extract_string(
                cred, "package-registryCredentialsId"
            ),
            github_copilot_credentials_id=_extract_string(
                cred, "githubCopilotCredentialsId"
            ),
            bitbucket_token_credentials_id=_extract_string(
                cred, "bitbucketTokenCredentialsId"
            ),
            bitbucket_ssh_credentials_id=_extract_string(
                cred, "bitbucketSshCredentialsId"
            ),
            gwt_token_credentials_id=_extract_string(cred, "gwtTokenCredentialsId"),
            prm_token_credentials_id=_extract_string(cred, "prmTokenCredentialsId"),
        ),
    )


# ---------------------------------------------------------------------------
# Validation
# ---------------------------------------------------------------------------


def validate_config(cfg: PipelineConfig) -> ValidationResult:
    """Check required and optional fields in a PipelineConfig.

    Args:
        cfg: Parsed PipelineConfig to validate.

    Returns:
        ValidationResult with any errors and warnings.
    """
    result = ValidationResult()
    if not cfg.registry.pull_domain:
        result.errors.append("registry.pullDomain is required")
    if not cfg.registry.pull_url:
        result.errors.append("registry.pullUrl is required")
    if not cfg.registry.push_domain:
        result.errors.append("registry.pushDomain is required")
    if not cfg.registry.credentials_id:
        result.errors.append("registry.credentialsId is required")
    if not cfg.credentials.package-registry_credentials_id:
        result.errors.append("credentials.package-registryCredentialsId is required")
    if not cfg.credentials.github_copilot_credentials_id:
        result.errors.append("credentials.githubCopilotCredentialsId is required")
    if not cfg.credentials.bitbucket_token_credentials_id:
        result.errors.append(
            "credentials.bitbucketTokenCredentialsId is required — "
            "pipeline cannot write PR status or comments back to Bitbucket without it"
        )
    if not cfg.credentials.bitbucket_ssh_credentials_id:
        result.errors.append(
            "credentials.bitbucketSshCredentialsId is required — "
            "used by the Pipeline Snapshot Guard for ls-remote and Jenkins job SCM"
        )
    if not cfg.credentials.gwt_token_credentials_id:
        if not cfg.credentials.prm_token_credentials_id:
            result.errors.append(
                "credentials.gwtTokenCredentialsId is required — "
                "Snapshot Guard uses it to re-fire the webhook on submodule drift. "
                "If set incorrectly it routes to the wrong job with no error. "
                "Set prmTokenCredentialsId to use parameterized remote trigger instead."
            )
        else:
            result.warnings.append(
                "credentials.gwtTokenCredentialsId is not set — "
                "Snapshot Guard webhook re-firing will not work. "
                "Only parameterized remote trigger "
                "(prmTokenCredentialsId) is configured."
            )
    return result


# ---------------------------------------------------------------------------
# Jenkins HTTP client
# ---------------------------------------------------------------------------


class JenkinsClient:
    """Minimal authenticated HTTP client for the Jenkins REST API."""

    def __init__(
        self,
        base_url: str,
        username: str,
        api_token: str,
        folder: str = "",
    ) -> None:
        """Initialise the client.

        Args:
            base_url: Jenkins root URL (no trailing slash).
            username: Jenkins username.
            api_token: Jenkins API token or password.
            folder: Jenkins folder path (slash-separated for nested folders).
                Used to walk the credential store hierarchy when looking up
                credentials — most-specific folder first, system store last.
        """
        self._base = base_url.rstrip("/")
        self._folder = self._normalize_folder(folder)
        raw = f"{username}:{api_token}".encode()
        self._auth = f"Basic {base64.b64encode(raw).decode()}"
        self._crumb: dict[str, str] | None = None

    @property
    def base_url(self) -> str:
        """Jenkins root URL."""
        return self._base

    def _fetch_crumb(self) -> dict[str, str]:
        """Fetch a Jenkins CSRF crumb for POST requests.

        Returns:
            Dict mapping crumb field name to crumb value.
            Empty dict when CSRF protection is disabled.

        Raises:
            urllib.error.HTTPError: On unexpected API errors (not 404).
        """
        url = f"{self._base}/crumbIssuer/api/json"
        req = urllib.request.Request(url, headers={"Authorization": self._auth})
        try:
            with urllib.request.urlopen(req) as resp:
                data: dict[str, str] = json.loads(resp.read())
            return {data["crumbRequestField"]: data["crumb"]}
        except urllib.error.HTTPError as exc:
            if exc.code == _HTTP_NOT_FOUND:
                return {}
            raise

    def _get_crumb(self) -> dict[str, str]:
        """Return the cached CSRF crumb, fetching it on the first call.

        Returns:
            Dict mapping crumb field name to crumb value.
        """
        if self._crumb is None:
            self._crumb = self._fetch_crumb()
        return self._crumb

    def _request(
        self,
        url: str,
        *,
        method: str = "GET",
        body: bytes | None = None,
        content_type: str | None = None,
    ) -> bytes:
        """Make an authenticated request to Jenkins.

        Args:
            url: Full request URL.
            method: HTTP method.
            body: Request body for POST requests.
            content_type: Content-Type header value.

        Returns:
            Response body bytes.

        Raises:
            urllib.error.HTTPError: On any non-2xx response.
        """
        headers: dict[str, str] = {"Authorization": self._auth}
        if content_type:
            headers["Content-Type"] = content_type
        if method == "POST":
            headers.update(self._get_crumb())
        req = urllib.request.Request(url, data=body, headers=headers, method=method)
        with urllib.request.urlopen(req) as resp:
            return bytes(resp.read())

    @staticmethod
    def _normalize_folder(folder: str) -> str:
        """Strip /job/ URL separators from a Jenkins folder path.

        Accepts both ``A01A0F_met-app/app-ci`` and
        ``A01A0F_met-app/job/app-ci`` (copied from a Jenkins URL) formats.

        Args:
            folder: Jenkins folder path in either format.

        Returns:
            Slash-separated folder path without /job/ separators.
        """
        return folder.strip("/").replace("/job/", "/")

    @staticmethod
    def _folder_url_path(folder: str) -> str:
        """Convert a normalized folder path to a Jenkins URL path segment.

        E.g. ``A01A0F_met-app/app-ci`` → ``A01A0F_met-app/job/app-ci``.

        Args:
            folder: Normalized slash-separated folder path.

        Returns:
            Jenkins URL path with /job/ separators and percent-encoded segments.
        """
        segments = [urllib.parse.quote(s, safe="") for s in folder.split("/") if s]
        return "/job/".join(segments)

    def _credential_store_urls(self, cred_id: str) -> list[str]:
        """Build credential store URLs to check, most-specific folder first.

        Jenkins resolves credentials by walking up: job folder → parent folders
        → system store. This method mirrors that order.

        Args:
            cred_id: Jenkins credential ID.

        Returns:
            List of API URLs to try in order.
        """
        encoded = urllib.parse.quote(cred_id, safe="")
        suffix = (
            "/credentials/store/folder/domain/_/credential/"
            f"{encoded}/api/json?tree=id,typeName"
        )
        urls: list[str] = []
        if self._folder:
            segments = [s for s in self._folder.split("/") if s]
            for i in range(len(segments), 0, -1):
                path = self._folder_url_path("/".join(segments[:i]))
                urls.append(f"{self._base}/job/{path}{suffix}")
        urls.append(
            f"{self._base}/credentials/store/system/domain/_"
            f"/credential/{encoded}/api/json?tree=id,typeName"
        )
        return urls

    def credential_info(self, cred_id: str) -> dict[str, str] | None:
        """Return id and typeName for a credential, or None if not found.

        Walks the folder credential store hierarchy before falling back to the
        system store, mirroring how Jenkins resolves credentials for a job.
        Falls back to listing all credentials when the per-credential endpoint
        returns 404 (some Jenkins versions do not expose it).

        Args:
            cred_id: Jenkins credential ID to look up.

        Returns:
            Dict with ``id`` and ``typeName`` keys, or None if the credential
            does not exist in any reachable store.

        Raises:
            urllib.error.HTTPError: On unexpected API errors (not 404).
        """
        if not cred_id:
            return None
        for url in self._credential_store_urls(cred_id):
            try:
                data: dict[str, str] = json.loads(self._request(url))
                return data
            except urllib.error.HTTPError as exc:
                if exc.code == _HTTP_NOT_FOUND:
                    continue
                raise
        # Per-credential endpoint not available — fall back to listing all.
        for cred in self.list_credentials():
            if cred.get("id") == cred_id:
                return cred
        return None

    def credential_exists(self, cred_id: str) -> bool:
        """Check whether a credential ID exists in the Jenkins global store.

        Args:
            cred_id: Jenkins credential ID to look up.

        Returns:
            True if the credential exists, False if not found.

        Raises:
            urllib.error.HTTPError: On unexpected API errors (not 404).
        """
        return self.credential_info(cred_id) is not None

    def job_exists(self, job_name: str, folder: str) -> bool:
        """Check whether a Jenkins job already exists.

        Args:
            job_name: Jenkins job name.
            folder: Jenkins folder containing the job.

        Returns:
            True if the job exists.

        Raises:
            urllib.error.HTTPError: On unexpected API errors (not 404).
        """
        enc_folder = self._folder_url_path(self._normalize_folder(folder))
        enc_name = urllib.parse.quote(job_name, safe="")
        url = f"{self._base}/job/{enc_folder}/job/{enc_name}/api/json?tree=name"
        try:
            self._request(url)
            return True
        except urllib.error.HTTPError as exc:
            if exc.code == _HTTP_NOT_FOUND:
                return False
            raise

    def create_job(self, job_name: str, xml_config: str, folder: str) -> None:
        """Create a Jenkins pipeline job from XML configuration.

        Args:
            job_name: Name for the new Jenkins job.
            xml_config: Jenkins job config XML string.
            folder: Jenkins folder to create the job in.

        Raises:
            urllib.error.HTTPError: If the creation request fails.
        """
        enc_folder = self._folder_url_path(self._normalize_folder(folder))
        enc_name = urllib.parse.quote(job_name, safe="")
        url = f"{self._base}/job/{enc_folder}/createItem?name={enc_name}"
        self._request(
            url,
            method="POST",
            body=xml_config.encode("utf-8"),
            content_type="application/xml; charset=utf-8",
        )

    def update_job(self, job_name: str, xml_config: str, folder: str) -> None:
        """Overwrite an existing Jenkins job configuration from XML.

        Args:
            job_name: Name of the Jenkins job to update.
            xml_config: Replacement Jenkins job config XML string.
            folder: Jenkins folder that contains the job.

        Raises:
            urllib.error.HTTPError: If the update request fails.
        """
        enc_folder = self._folder_url_path(self._normalize_folder(folder))
        enc_name = urllib.parse.quote(job_name, safe="")
        url = f"{self._base}/job/{enc_folder}/job/{enc_name}/config.xml"
        self._request(
            url,
            method="POST",
            body=xml_config.encode("utf-8"),
            content_type="application/xml; charset=utf-8",
        )

    def list_jobs(self, folder: str) -> list[str]:
        """Return job names in a Jenkins folder, excluding sub-folders.

        Args:
            folder: Jenkins folder path.

        Returns:
            List of job names (no folder entries).

        Raises:
            urllib.error.HTTPError: On API errors.
        """
        enc_folder = self._folder_url_path(self._normalize_folder(folder))
        url = f"{self._base}/job/{enc_folder}/api/json?tree=jobs[name,_class]"
        data: dict[str, list[dict[str, str]]] = json.loads(self._request(url))
        return [
            job["name"]
            for job in data.get("jobs", [])
            if "Folder" not in job.get("_class", "")
        ]

    def get_job_config(self, job_name: str, folder: str) -> str:
        """Fetch a Jenkins job's current config.xml as a string.

        Args:
            job_name: Jenkins job name.
            folder: Jenkins folder containing the job.

        Returns:
            Raw XML config string.

        Raises:
            urllib.error.HTTPError: On API errors.
        """
        enc_folder = self._folder_url_path(self._normalize_folder(folder))
        enc_name = urllib.parse.quote(job_name, safe="")
        url = f"{self._base}/job/{enc_folder}/job/{enc_name}/config.xml"
        return self._request(url).decode("utf-8")

    def jobs_using_script(self, folder: str, script_path: str) -> list[str]:
        """Return job names whose scriptPath matches script_path.

        Args:
            folder: Jenkins folder to search.
            script_path: Jenkinsfile path to match.

        Returns:
            List of matching job names.
        """
        matching: list[str] = []
        for name in self.list_jobs(folder):
            try:
                config = self.get_job_config(name, folder)
                if f"<scriptPath>{script_path}</scriptPath>" in config:
                    matching.append(name)
            except urllib.error.HTTPError:
                pass
        return matching

    def list_credentials(self, type_name: str | None = None) -> list[dict[str, str]]:
        """Return credentials from all stores, optionally filtered by type.

        Args:
            type_name: If set, only return credentials whose typeName matches.

        Returns:
            List of credential dicts with id, typeName, and description keys.
        """
        seen: set[str] = set()
        results: list[dict[str, str]] = []
        tree = urllib.parse.quote("credentials[id,typeName,description]", safe="=[]")
        store_urls: list[str] = []
        if self._folder:
            segments = [seg for seg in self._folder.split("/") if seg]
            for idx in range(len(segments), 0, -1):
                path = self._folder_url_path("/".join(segments[:idx]))
                store_urls.append(
                    f"{self._base}/job/{path}/credentials/store/folder/domain/_/api/json?tree={tree}"
                )
        store_urls.append(
            f"{self._base}/credentials/store/system/domain/_/api/json?tree={tree}"
        )
        for url in store_urls:
            try:
                data: dict[str, list[dict[str, str]]] = json.loads(self._request(url))
                for cred in data.get("credentials", []):
                    cred_id = cred.get("id", "")
                    if not cred_id or cred_id in seen:
                        continue
                    seen.add(cred_id)
                    if type_name is None or cred.get("typeName") == type_name:
                        results.append(cred)
            except Exception:
                pass
        return results


# ---------------------------------------------------------------------------
# Jenkins job XML generation
# ---------------------------------------------------------------------------


def _xml_escape(text: str) -> str:
    """Escape special characters for XML text content.

    Args:
        text: Raw string.

    Returns:
        XML-safe string.
    """
    return (
        text.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
    )


def _build_gwt_vars_xml() -> str:
    """Build the GWT variable extraction XML fragment.

    Returns:
        Joined GenericVariable XML elements.
    """
    parts: list[str] = []
    for name, expr, _desc in _GWT_VARS:
        safe_name = _xml_escape(name)
        safe_expr = _xml_escape(expr)
        parts.append(
            "        <org.jenkinsci.plugins.gwt.GenericVariable>\n"
            "          <expressionType>JSONPath</expressionType>\n"
            f"          <key>{safe_name}</key>\n"
            f"          <value>{safe_expr}</value>\n"
            "          <regexpFilter></regexpFilter>\n"
            "          <defaultValue></defaultValue>\n"
            "        </org.jenkinsci.plugins.gwt.GenericVariable>"
        )
    return "\n".join(parts)


def _build_params_xml() -> str:
    """Build the pipeline parameter definitions XML fragment.

    Returns:
        Joined parameter definition XML elements.
    """
    parts: list[str] = []
    for name, default, desc in _STRING_PARAMS:
        safe_desc = _xml_escape(desc)
        parts.append(
            "        <hudson.model.StringParameterDefinition>\n"
            f"          <name>{name}</name>\n"
            f"          <defaultValue>{default}</defaultValue>\n"
            f"          <description>{safe_desc}</description>\n"
            "          <trim>false</trim>\n"
            "        </hudson.model.StringParameterDefinition>"
        )
    for name, choices, desc in _CHOICE_PARAMS:
        safe_desc = _xml_escape(desc)
        choices_inner = "\n".join(f"            <string>{c}</string>" for c in choices)
        parts.append(
            "        <hudson.model.ChoiceParameterDefinition>\n"
            f"          <name>{name}</name>\n"
            '          <choices class="java.util.Arrays$ArrayList">\n'
            '            <a class="string-array">\n'
            f"{choices_inner}\n"
            "            </a>\n"
            "          </choices>\n"
            f"          <description>{safe_desc}</description>\n"
            "        </hudson.model.ChoiceParameterDefinition>"
        )
    return "\n".join(parts)


def _patch_job_xml(current_xml: str) -> str:
    """Add missing GWT variables and string parameters to an existing job config XML.

    Injects only the elements defined in ``_GWT_VARS`` and ``_STRING_PARAMS``
    that are absent from the live config by inserting raw XML snippets directly
    into the source string — no parse/serialize round-trip, so the existing
    document is left byte-for-byte identical except for the insertions.

    Args:
        current_xml: Raw Jenkins job config XML retrieved from the live job.

    Returns:
        Patched XML string ready to POST back to Jenkins.
    """
    # ------------------------------------------------------------------ #
    # Determine indentation used by existing sibling elements so injected
    # XML matches the document style.
    # ------------------------------------------------------------------ #
    gvar_indent_m = re.search(
        r"(\s+)<org\.jenkinsci\.plugins\.gwt\.GenericVariable>", current_xml
    )
    gvar_indent = gvar_indent_m.group(1) if gvar_indent_m else "            "

    param_indent_m = re.search(
        r"(\s+)<hudson\.model\.StringParameterDefinition>", current_xml
    )
    param_indent = param_indent_m.group(1) if param_indent_m else "        "

    # ------------------------------------------------------------------ #
    # 1. Inject missing GWT variables before </genericVariables>
    # ------------------------------------------------------------------ #
    existing_keys = set(re.findall(r"<key>(.*?)</key>", current_xml))
    gwt_inserts: list[str] = []
    for name, expr, _desc in _GWT_VARS:
        if name not in existing_keys:
            safe_name = _xml_escape(name)
            safe_expr = _xml_escape(expr)
            gwt_inserts.append(
                f"{gvar_indent}<org.jenkinsci.plugins.gwt.GenericVariable>\n"
                f"{gvar_indent}  <expressionType>JSONPath</expressionType>\n"
                f"{gvar_indent}  <key>{safe_name}</key>\n"
                f"{gvar_indent}  <value>{safe_expr}</value>\n"
                f"{gvar_indent}  <regexpFilter></regexpFilter>\n"
                f"{gvar_indent}  <defaultValue></defaultValue>\n"
                f"{gvar_indent}</org.jenkinsci.plugins.gwt.GenericVariable>"
            )
    if gwt_inserts:
        snippet = "\n".join(gwt_inserts) + "\n"
        current_xml = current_xml.replace(
            "</genericVariables>", snippet + "</genericVariables>", 1
        )

    # ------------------------------------------------------------------ #
    # 2. Inject missing string parameters before </parameterDefinitions>
    # ------------------------------------------------------------------ #
    existing_params = set(
        re.findall(
            r"<hudson\.model\.StringParameterDefinition>\s*<name>(.*?)</name>",
            current_xml,
        )
    )
    param_inserts: list[str] = []
    for name, default, desc in _STRING_PARAMS:
        if name not in existing_params:
            safe_desc = _xml_escape(desc)
            safe_default = _xml_escape(default)
            param_inserts.append(
                f"{param_indent}<hudson.model.StringParameterDefinition>\n"
                f"{param_indent}  <name>{name}</name>\n"
                f"{param_indent}  <defaultValue>{safe_default}</defaultValue>\n"
                f"{param_indent}  <description>{safe_desc}</description>\n"
                f"{param_indent}  <trim>false</trim>\n"
                f"{param_indent}</hudson.model.StringParameterDefinition>"
            )
    if param_inserts:
        snippet = "\n".join(param_inserts) + "\n"
        current_xml = current_xml.replace(
            "</parameterDefinitions>", snippet + "</parameterDefinitions>", 1
        )

    return current_xml


def build_job_xml(
    repo_url: str,
    scm_cred_id: str,
    gwt_cred_id: str,
    pipeline_branch: str,
    prm_token_cred_id: str = "",
) -> str:
    """Build Jenkins pipeline job configuration XML.

    Args:
        repo_url: Git repository URL.
        scm_cred_id: Jenkins credential ID for Git checkout.
        gwt_cred_id: Jenkins credential ID for the GWT token.
        pipeline_branch: SCM branch spec (e.g. ``*/pipelines``).
        prm_token_cred_id: Credential ID used as the parameterized remote trigger
            token — the ID string becomes the ``<authToken>`` value so the trigger
            URL is ``/buildWithParameters?token=<credential-id>``.

    Returns:
        Jenkins job config XML string.
    """
    gwt_vars = _build_gwt_vars_xml()
    params = _build_params_xml()
    safe_repo = _xml_escape(repo_url)
    safe_scm = _xml_escape(scm_cred_id)
    safe_gwt = _xml_escape(gwt_cred_id)
    safe_prm = _xml_escape(prm_token_cred_id)
    safe_branch = _xml_escape(pipeline_branch)
    safe_script = _xml_escape(_SCRIPT_PATH)
    return (
        "<?xml version='1.1' encoding='UTF-8'?>\n"
        '<flow-definition plugin="workflow-job">\n'
        "  <description>Majordomo — repository operations for evolving software."
        " — created by setup-majordomo.py</description>\n"
        + (f"  <authToken>{safe_prm}</authToken>\n" if safe_prm else "")
        + "  <keepDependencies>false</keepDependencies>\n"
        "  <properties>\n"
        "    <org.jenkinsci.plugins.workflow.job.properties.PipelineTriggersJobProperty>\n"
        "      <triggers>\n"
        "        <org.jenkinsci.plugins.gwt.GenericTrigger"
        ' plugin="generic-webhook-trigger">\n'
        "          <spec></spec>\n"
        "          <genericVariables>\n"
        f"{gwt_vars}\n"
        "          </genericVariables>\n"
        "          <regexpFilterText></regexpFilterText>\n"
        "          <regexpFilterExpression></regexpFilterExpression>\n"
        "          <printContributedVariables>true</printContributedVariables>\n"
        "          <printPostContent>false</printPostContent>\n"
        "          <causeString>Triggered by Bitbucket: ${PR_EVENT_TYPE} in ${CHANGE_BRANCH} by ${ACTOR_NAME}</causeString>\n"
        "          <token></token>\n"
        f"          <tokenCredentialId>{safe_gwt}</tokenCredentialId>\n"
        "          <silentResponse>false</silentResponse>\n"
        "          <overrideQuietPeriod>false</overrideQuietPeriod>\n"
        "          <shouldNotFlattern>false</shouldNotFlattern>\n"
        "          <allowSeveralTriggersPerBuild>false</allowSeveralTriggersPerBuild>\n"
        "        </org.jenkinsci.plugins.gwt.GenericTrigger>\n"
        "      </triggers>\n"
        "    </org.jenkinsci.plugins.workflow.job.properties.PipelineTriggersJobProperty>\n"
        "    <hudson.model.ParametersDefinitionProperty>\n"
        "      <parameterDefinitions>\n"
        f"{params}\n"
        "      </parameterDefinitions>\n"
        "    </hudson.model.ParametersDefinitionProperty>\n"
        "  </properties>\n"
        "  <definition"
        ' class="org.jenkinsci.plugins.workflow.cps.CpsScmFlowDefinition"'
        ' plugin="workflow-cps">\n'
        '    <scm class="hudson.plugins.git.GitSCM" plugin="git">\n'
        "      <configVersion>2</configVersion>\n"
        "      <userRemoteConfigs>\n"
        "        <hudson.plugins.git.UserRemoteConfig>\n"
        f"          <url>{safe_repo}</url>\n"
        f"          <credentialsId>{safe_scm}</credentialsId>\n"
        "        </hudson.plugins.git.UserRemoteConfig>\n"
        "      </userRemoteConfigs>\n"
        "      <branches>\n"
        "        <hudson.plugins.git.BranchSpec>\n"
        f"          <name>{safe_branch}</name>\n"
        "        </hudson.plugins.git.BranchSpec>\n"
        "      </branches>\n"
        "      <doGenerateSubmoduleConfigurations>"
        "false</doGenerateSubmoduleConfigurations>\n"
        '      <submoduleCfg class="empty-list"/>\n'
        "      <extensions>\n"
        "        <hudson.plugins.git.extensions.impl.SubmoduleOption>\n"
        "          <disableSubmodules>false</disableSubmodules>\n"
        "          <recursiveSubmodules>false</recursiveSubmodules>\n"
        "          <trackingSubmodules>false</trackingSubmodules>\n"
        "          <reference></reference>\n"
        "          <parentCredentials>true</parentCredentials>\n"
        "          <shallow>false</shallow>\n"
        "        </hudson.plugins.git.extensions.impl.SubmoduleOption>\n"
        "      </extensions>\n"
        "    </scm>\n"
        f"    <scriptPath>{safe_script}</scriptPath>\n"
        "    <lightweight>false</lightweight>\n"
        "  </definition>\n"
        "  <triggers/>\n"
        "  <disabled>false</disabled>\n"
        "</flow-definition>"
    )


# ---------------------------------------------------------------------------
# Config diff helpers
# ---------------------------------------------------------------------------


def _gwt_vars_from_xml(xml: str) -> dict[str, str]:
    """Extract {variable_name: jsonpath} from a Jenkins job config.xml.

    Args:
        xml: Raw Jenkins job config XML string.

    Returns:
        Dict mapping GWT variable key to JSONPath expression.
    """
    result: dict[str, str] = {}
    block_re = re.compile(
        r"<org\.jenkinsci\.plugins\.gwt\.GenericVariable>(.*?)"
        r"</org\.jenkinsci\.plugins\.gwt\.GenericVariable>",
        re.DOTALL,
    )
    for match in block_re.finditer(xml):
        key_m = re.search(r"<key>([^<]*)</key>", match.group(1))
        val_m = re.search(r"<value>([^<]*)</value>", match.group(1))
        if key_m and val_m:
            result[key_m.group(1)] = val_m.group(1)
    return result


def _params_from_xml(xml: str) -> set[str]:
    """Extract string parameter names from a Jenkins job config.xml.

    Args:
        xml: Raw Jenkins job config XML string.

    Returns:
        Set of string parameter names.
    """
    return set(
        re.findall(
            r"<hudson\.model\.StringParameterDefinition>.*?<name>([^<]+)</name>",
            xml,
            re.DOTALL,
        )
    )


def _config_needs_update(current_xml: str) -> bool:
    """Return True when the current config has wrong GWT vars or parameters.

    Args:
        current_xml: Raw Jenkins job config XML from the live job.

    Returns:
        True if the config should be overwritten.
    """
    expected_vars = {name: expr for name, expr, _ in _GWT_VARS}
    expected_params = {name for name, _, _ in _STRING_PARAMS}
    return (
        _gwt_vars_from_xml(current_xml) != expected_vars
        or _params_from_xml(current_xml) != expected_params
    )


# ---------------------------------------------------------------------------
# CLI steps
# ---------------------------------------------------------------------------


def _check_registry_reachability(cfg: PipelineConfig) -> None:
    """TCP-probe the configured registry domains on port 443.

    Unreachable domains are reported as warnings only — they may still be
    accessible from the Jenkins agent even if not from this machine.

    Args:
        cfg: Parsed PipelineConfig with registry domains to probe.
    """
    domains = {
        "registry.pullDomain": cfg.registry.pull_domain,
        "registry.pushDomain": cfg.registry.push_domain,
    }
    print("\nChecking registry reachability ...")
    for label, domain in domains.items():
        if not domain:
            print(f"  \u23ed\ufe0f  {label}: (not set \u2014 skip)")
            continue
        try:
            with socket.create_connection(
                (domain, _HTTPS_PORT), timeout=_CONNECT_TIMEOUT
            ):
                print(f"  \u2705 {label}: {domain}")
        except OSError:
            print(
                f"  \u26a0\ufe0f  {label}: {domain} \u2014 TCP connect failed "
                "(may still be reachable from the Jenkins agent)"
            )


def _run_config_validation(config_path: Path) -> PipelineConfig:
    """Parse and validate the config file, exiting on errors.

    Args:
        config_path: Path to .majordomo-config.groovy.

    Returns:
        Parsed and validated PipelineConfig.
    """
    if not config_path.exists():
        config_path_str = str(config_path)
        msg = inspect.cleandoc(f"""
            Config file not found: {config_path_str}
            Copy the template and edit it:
              cp .majordomo/example.majordomo-config.groovy \\
                 .majordomo-config.groovy
        """)
        print(msg)
        sys.exit(1)

    print(f"Parsing {config_path} ...")
    cfg = parse_config(config_path)

    print("\nValidating config fields ...")
    result = validate_config(cfg)
    for err in result.errors:
        print(f"  \u274c ERROR   {err}")
    for warn in result.warnings:
        print(f"  \u26a0\ufe0f  WARN    {warn}")
    if result.ok and not result.warnings:
        print("  \u2705 All fields OK")

    if not result.ok:
        error_count = len(result.errors)
        print(f"\n{error_count} error(s) — fix config and re-run.")
        sys.exit(1)

    return cfg


def _print_candidates(client: JenkinsClient, type_name: str) -> None:
    """Print credential IDs of type_name as configuration suggestions.

    Args:
        client: Authenticated Jenkins client.
        type_name: Jenkins credential type name to filter by.
    """
    try:
        candidates = client.list_credentials(type_name)
    except Exception:
        return
    if not candidates:
        return
    ids = ", ".join(c["id"] for c in candidates)
    print(f"         Candidates ({type_name}): {ids}")


def _run_credential_verification(
    client: JenkinsClient,
    cfg: PipelineConfig,
    scm_credentials_id: str,
) -> None:
    """Verify all configured credential IDs exist in the Jenkins global store.

    Prints a status line for each credential and exits with code 1 if any
    are missing.

    Args:
        client: Authenticated Jenkins client.
        cfg: Parsed PipelineConfig with credential IDs to check.
        scm_credentials_id: SSH credential used for SCM checkout.
    """
    print("\nVerifying credentials in Jenkins ...")
    creds = cfg.credentials
    reg = cfg.registry
    checks: list[tuple[str, str]] = [
        ("registry.credentialsId", reg.credentials_id),
        (
            "credentials.githubCopilotCredentialsId",
            creds.github_copilot_credentials_id,
        ),
        ("credentials.package-registryCredentialsId", creds.package-registry_credentials_id),
        (
            "credentials.bitbucketTokenCredentialsId",
            creds.bitbucket_token_credentials_id,
        ),
        ("credentials.bitbucketSshCredentialsId", creds.bitbucket_ssh_credentials_id),
        ("credentials.gwtTokenCredentialsId", creds.gwt_token_credentials_id),
        ("credentials.prmTokenCredentialsId", creds.prm_token_credentials_id),
        ("--scm-credentials-id", scm_credentials_id),
    ]
    all_ok = True
    for cfg_key, cred_id in checks:
        if not cred_id:
            print(f"  \u23ed\ufe0f  {cfg_key}: (not set \u2014 skip)")
            expected = _CRED_EXPECTED_TYPES.get(cfg_key, "")
            if expected:
                _print_candidates(client, expected)
            continue
        try:
            info = client.credential_info(cred_id)
        except urllib.error.HTTPError as exc:
            print(f"  \u26a0\ufe0f  {cfg_key}: API error HTTP {exc.code} \u2014 skip")
            continue
        if info is None:
            print(f"  \u274c {cfg_key}: {cred_id} \u2014 not found in Jenkins")
            all_ok = False
            expected = _CRED_EXPECTED_TYPES.get(cfg_key, "")
            if expected:
                _print_candidates(client, expected)
            continue
        expected = _CRED_EXPECTED_TYPES.get(cfg_key, "")
        actual = info.get("typeName", "")
        if expected and actual and actual != expected:
            print(
                f"  \u26a0\ufe0f  {cfg_key}: {cred_id} \u2014 "
                f"wrong type '{actual}' (expected '{expected}')"
            )
            _print_candidates(client, expected)
            all_ok = False
        else:
            print(f"  \u2705 {cfg_key}: {cred_id}")
    if not all_ok:
        print(
            "\nOne or more credentials not found or wrong type "
            "\u2014 fix in Jenkins and re-run."
        )
        sys.exit(1)


def _select_job_interactively(client: JenkinsClient, folder: str) -> str:
    """List jobs using this pipeline script and prompt the user to select one.

    Args:
        client: Authenticated Jenkins client.
        folder: Jenkins folder to search.

    Returns:
        Selected job name.
    """
    print(f"\nFetching jobs in '{folder}' ...")
    try:
        jobs = client.jobs_using_script(folder, _SCRIPT_PATH)
    except urllib.error.HTTPError as exc:
        print(f"  \u274c Failed to list jobs (HTTP {exc.code}).")
        sys.exit(1)
    if not jobs:
        print(f"  No jobs using '{_SCRIPT_PATH}' found in '{folder}'.")
        sys.exit(1)
    print(f"\nAvailable pipelines in '{folder}':")
    for idx, name in enumerate(jobs, start=1):
        print(f"  {idx}. {name}")
    while True:
        raw = input("\nSelect a pipeline (number or name): ").strip()
        if raw.isdigit():
            choice = int(raw) - 1
            if 0 <= choice < len(jobs):
                return jobs[choice]
            print(f"  Enter a number between 1 and {len(jobs)}.")
        elif raw in jobs:
            return raw
        else:
            print(f"  '{raw}' not found. Enter a number or exact name.")


def _dump_or_submit_xml(
    xml_config: str,
    dump_xml_path: str,
    label: str,
) -> bool:
    """Write XML to a file when dump_xml_path is set; return True if dumped.

    Args:
        xml_config: Jenkins job config XML string.
        dump_xml_path: File path to write, or empty string to skip.
        label: Short label for the printed message (e.g. ``update``).

    Returns:
        True when XML was written to disk (caller should return).
    """
    if not dump_xml_path:
        return False
    dump_path = Path(dump_xml_path)
    dump_path.write_text(xml_config, encoding="utf-8")
    msg = inspect.cleandoc(f"""
        XML written to {dump_path}
        Review and submit manually, or re-run without --dump-xml to {label}.
    """)
    print(msg)
    return True


def _submit_xml_request(
    action_label: str,
    submit_fn: Callable[[], None],
) -> None:
    """Call submit_fn and handle HTTPError, exiting on failure.

    Args:
        action_label: Human-readable label (e.g. ``creation``).
        submit_fn: Zero-argument callable that performs the HTTP request.
    """
    try:
        submit_fn()
    except urllib.error.HTTPError as exc:
        body = ""
        try:
            body = exc.read().decode("utf-8", errors="replace")
        except Exception:
            body = ""
        print(f"  \u274c Job {action_label} failed (HTTP {exc.code}).")
        if body:
            log_path = Path("setup-majordomo-error.html")
            log_path.write_text(body, encoding="utf-8")
            print(f"     Full response written to {log_path}")
        hdrs = {k: v for k, v in exc.headers.items()} if exc.headers else {}
        if hdrs:
            print(f"     Response headers: {hdrs}")
        sys.exit(1)


def _update_existing_job(
    client: JenkinsClient,
    setup: JobSetupConfig,
    job_path: str,
) -> None:
    """Patch an existing Jenkins job config to add missing GWT vars and params.

    Fetches the live config, patches only the missing elements (preserving all
    existing properties), and POSTs the result back.

    Args:
        client: Authenticated Jenkins client.
        setup: Job parameters.
        job_path: Jenkins folder/name path for messages.
    """
    if not setup.gwt_credentials_id and not setup.prm_token_credentials_id:
        print(
            "--update-job requires credentials.gwtTokenCredentialsId or "
            "credentials.prmTokenCredentialsId in "
            ".majordomo-config.groovy so the job has a trigger token."
        )
        sys.exit(1)

    try:
        current_xml = client.get_job_config(setup.job_name, setup.folder)
    except urllib.error.HTTPError as exc:
        print(
            f"  ❌ Could not fetch current config (HTTP {exc.code})."
            " Cannot patch without the existing XML."
        )
        sys.exit(1)

    if not _config_needs_update(current_xml):
        print(f"  ✅ Job '{job_path}' is already up to date \u2014 no changes needed.")
        return

    print("  Config out of date — applying patch ...")
    xml_config = _patch_job_xml(current_xml)

    if _dump_or_submit_xml(xml_config, setup.dump_xml_path, "update"):
        if setup.dump_xml_path:
            current_path = Path(setup.dump_xml_path).with_suffix(".current.xml")
            current_path.write_text(current_xml, encoding="utf-8")
            print(
                f"Current Jenkins config saved to {current_path}"
                " (diff against generated XML)"
            )
        return

    print(f"  Updating job '{job_path}' ...")
    _submit_xml_request(
        "update",
        lambda: client.update_job(setup.job_name, xml_config, setup.folder),
    )
    print(f"  ✅ Job '{job_path}' configuration updated.")


def _create_new_job(
    client: JenkinsClient,
    setup: JobSetupConfig,
    job_path: str,
) -> None:
    """Create a new Jenkins job when create_if_missing is set.

    Args:
        client: Authenticated Jenkins client.
        setup: Job parameters.
        job_path: Jenkins folder/name path for messages.
    """
    if not setup.repo_url:
        print("--repo-url is required with --create-job.")
        sys.exit(1)

    if not setup.gwt_credentials_id and not setup.prm_token_credentials_id:
        print(
            "--create-job requires credentials.gwtTokenCredentialsId or "
            "credentials.prmTokenCredentialsId in "
            ".majordomo-config.groovy so the job has a trigger token."
        )
        sys.exit(1)

    xml_config = build_job_xml(
        repo_url=setup.repo_url,
        scm_cred_id=setup.scm_credentials_id,
        gwt_cred_id=setup.gwt_credentials_id,
        pipeline_branch=setup.pipeline_branch,
        prm_token_cred_id=setup.prm_token_credentials_id,
    )

    if _dump_or_submit_xml(xml_config, setup.dump_xml_path, "create"):
        return

    print(f"  Creating job '{job_path}' ...")
    _submit_xml_request(
        "creation",
        lambda: client.create_job(setup.job_name, xml_config, setup.folder),
    )

    folder_url_path = client._folder_url_path(client._normalize_folder(setup.folder))
    enc_name = urllib.parse.quote(setup.job_name)
    job_url = f"{client.base_url}/job/{folder_url_path}/job/{enc_name}"
    prm_cred_id = setup.prm_token_credentials_id
    trigger_url = (
        f"{job_url}/buildWithParameters?token={prm_cred_id}" if prm_cred_id else ""
    )
    gwt_section = (
        (
            "\n  2. Verify the Bitbucket webhook points to this job's GWT endpoint.\n"
            "     Events: pr:opened, pr:from_ref_updated, repo:refs_changed\n"
            "  3. Open a PR to trigger a test build."
        )
        if setup.gwt_credentials_id
        else ""
    )
    prm_section = (
        (
            f"\n\n  Parameterized remote trigger token: '{prm_cred_id}'\n"
            "  (Credential ID is used as the token value.)\n"
            f"  Trigger URL: {trigger_url}"
        )
        if prm_cred_id
        else ""
    )
    msg = (
        inspect.cleandoc(f"""
        \u2705 Job '{job_path}' created.

        Next steps:
          1. Open the job and trigger a first build to register parameters:
             {job_url}
    """)
        + gwt_section
        + prm_section
    )
    print(msg)


def _run_job_setup(
    client: JenkinsClient,
    setup: JobSetupConfig,
) -> None:
    """Check whether the Jenkins job exists and optionally create or update it.

    Args:
        client: Authenticated Jenkins client.
        setup: Job creation/update parameters.
    """
    job_path = f"{setup.folder.rstrip('/')}/{setup.job_name}"
    print(f"\nChecking job: {job_path} ...")
    try:
        exists = client.job_exists(setup.job_name, setup.folder)
    except urllib.error.HTTPError as exc:
        print(f"  Job check failed (HTTP {exc.code}).")
        sys.exit(1)

    if exists:
        if not setup.update_if_exists:
            print(f"  \u2705 Job '{job_path}' already exists \u2014 nothing to create.")
            print("  Re-run with --update-job to overwrite its configuration.")
            return
        _update_existing_job(client, setup, job_path)
        return

    if not setup.create_if_missing:
        msg = inspect.cleandoc(f"""
            Job '{job_path}' not found.
            Re-run with --create-job to create it:
              python scripts/setup-majordomo.py ... --create-job --repo-url <ssh://...>
        """)
        print(msg)
        sys.exit(1)

    _create_new_job(client, setup, job_path)


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------


def _get_git_origin_url() -> str:
    """Return the SSH URL of the git origin remote, or an empty string."""
    try:
        result = subprocess.run(
            ["git", "remote", "get-url", "origin"],
            capture_output=True,
            text=True,
            check=True,
        )
        return result.stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError, OSError):
        return ""


def _parse_args() -> argparse.Namespace:
    """Parse and return command-line arguments.

    Returns:
        Parsed argument namespace.
    """
    parser = argparse.ArgumentParser(
        description=(
            "Validate .majordomo-config.groovy, verify Jenkins credentials, "
            "and optionally create the pipeline job."
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--config",
        default=str(_DEFAULT_CONFIG),
        metavar="PATH",
        help="Path to config file (default: .majordomo-config.groovy)",
    )
    parser.add_argument(
        "--validate-only",
        action="store_true",
        help="Only check config file structure — skip all Jenkins API calls",
    )
    parser.add_argument(
        "--jenkins-url",
        metavar="URL",
        default=_DEFAULT_JENKINS_URL,
        help=f"Jenkins base URL (default: {_DEFAULT_JENKINS_URL})",
    )
    parser.add_argument(
        "--username",
        metavar="USER",
        help="Jenkins username (default: REGISTRY_USER env var)",
    )
    parser.add_argument(
        "--api-token",
        metavar="TOKEN",
        help="Jenkins API token (or set JENKINS_API_TOKEN env var)",
    )
    parser.add_argument(
        "--job-name",
        metavar="NAME",
        help="Jenkins job name to verify or create",
    )
    parser.add_argument(
        "--folder",
        metavar="NAME",
        help="Jenkins folder that contains (or will contain) the job",
    )
    parser.add_argument(
        "--repo-url",
        metavar="URL",
        help="Git repository SSH URL (default: git remote get-url origin)",
    )
    parser.add_argument(
        "--pipeline-branch",
        default=_DEFAULT_BRANCH,
        metavar="SPEC",
        help="SCM branch spec (default: */pipelines)",
    )
    parser.add_argument(
        "--scm-credentials-id",
        default="",
        metavar="ID",
        help=(
            "Jenkins SSH credential ID for SCM checkout "
            "(default: credentials.bitbucketSshCredentialsId from config)"
        ),
    )
    parser.add_argument(
        "--create-job",
        action="store_true",
        help="Create the Jenkins pipeline job if it does not exist",
    )
    parser.add_argument(
        "--update-job",
        action="store_true",
        help="Overwrite the existing job configuration with freshly-generated XML",
    )
    parser.add_argument(
        "--dump-xml",
        metavar="PATH",
        help="Write the job config XML to a file instead of submitting it",
    )
    return parser.parse_args()


def main() -> None:
    """Entry point: validate config, verify credentials, create job."""
    args = _parse_args()
    cfg = _run_config_validation(Path(args.config))

    if args.validate_only:
        print("\nValidation complete (--validate-only).")
        return

    env_token = os.environ.get("JENKINS_API_TOKEN", "")
    api_token: str = str(args.api_token or "") or env_token
    env_user = os.environ.get("REGISTRY_USER", "").split("@")[0]
    username: str = str(args.username or "") or env_user

    missing: list[str] = []
    if not username:
        missing.append("--username or REGISTRY_USER env var")
    if not api_token:
        missing.append("--api-token or JENKINS_API_TOKEN env var")
    if not args.job_name and not args.update_job:
        missing.append("--job-name")
    if not args.folder:
        missing.append("--folder")
    if missing:
        missing_str = "\n  ".join(missing)
        msg = inspect.cleandoc(f"""
            Missing required arguments for Jenkins API calls:
              {missing_str}
            Use --validate-only to skip Jenkins API checks.
        """)
        print(msg)
        sys.exit(1)

    print(f"\nConnecting to Jenkins: {args.jenkins_url} ...")
    client = JenkinsClient(
        base_url=str(args.jenkins_url),
        username=username,
        api_token=api_token,
        folder=str(args.folder or ""),
    )

    _check_registry_reachability(cfg)

    scm_cred = (
        str(args.scm_credentials_id or "")
        or cfg.credentials.bitbucket_ssh_credentials_id
    )
    _run_credential_verification(client, cfg, scm_credentials_id=scm_cred)

    gwt_cred = cfg.credentials.gwt_token_credentials_id
    prm_cred = cfg.credentials.prm_token_credentials_id
    if not gwt_cred and not prm_cred:
        print(
            "\n\u26a0\ufe0f  Neither gwtTokenCredentialsId "
            "nor prmTokenCredentialsId is set; "
            "--create-job and --update-job will fail until at least one is configured."
        )

    job_name = str(args.job_name or "")
    if not job_name and args.update_job:
        job_name = _select_job_interactively(client, str(args.folder or ""))

    _run_job_setup(
        client,
        JobSetupConfig(
            job_name=job_name,
            folder=str(args.folder or ""),
            repo_url=str(args.repo_url or "") or _get_git_origin_url(),
            pipeline_branch=str(args.pipeline_branch),
            create_if_missing=bool(args.create_job),
            update_if_exists=bool(args.update_job),
            scm_credentials_id=scm_cred,
            gwt_credentials_id=gwt_cred,
            prm_token_credentials_id=prm_cred,
            dump_xml_path=str(args.dump_xml or ""),
        ),
    )


if __name__ == "__main__":
    main()
