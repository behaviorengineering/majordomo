#!/usr/bin/env python3
"""Jenkins central pipeline setup for majordomo.

Sets up the single central Jenkins job (MajordomoReview.Central.CI.Jenkinsfile)
that serves all onboarded repos. Also validates per-repo config files in
majordomo-central-config/.

Usage:
    # Validate org defaults only
    python scripts/setup-majordomo-central.py --validate-only

    # Validate + check credentials + create central job
    python scripts/setup-majordomo-central.py \\
        --jenkins-url https://jenkins.example.com \\
        --username alice \\
        --api-token <token> \\
        --job-name copilot-central \\
        --folder MyOrg/Non-Prod \\
        --repo-url ssh://git@bitbucket.example.com/TOOLS/majordomo.git \\
        --create-job

    # Validate a specific repo's onboarding config
    python scripts/setup-majordomo-central.py \\
        --validate-repo payments-api \\
        --validate-only

The API token can also be supplied via JENKINS_API_TOKEN environment variable.
The username defaults to REGISTRY_USER environment variable if omitted.
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
from dataclasses import dataclass, field
from pathlib import Path
from typing import TYPE_CHECKING

if isinstance(sys.stdout, io.TextIOWrapper):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
if isinstance(sys.stderr, io.TextIOWrapper):
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

import urllib.error
import urllib.parse
import urllib.request

if TYPE_CHECKING:
    from collections.abc import Callable

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

_CENTRAL_CONFIG_DIR = Path("majordomo-central-config")
_DEFAULTS_FILE = _CENTRAL_CONFIG_DIR / "_defaults.groovy"
_EXAMPLE_REPO_CONFIG = _CENTRAL_CONFIG_DIR / "example.repo-config.groovy"
_SCRIPT_PATH = ".majordomo/pipelines/MajordomoReview.Central.CI.Jenkinsfile"
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

# GWT variable extractions for the central job.
# Adds REPO_SLUG and PROJECT_KEY on top of the per-repo set.
_GWT_VARS: list[tuple[str, str, str]] = [
    (
        "REPO_SLUG",
        "$.pullRequest.fromRef.repository.slug",
        "App repo slug — selects configs/<slug>.groovy",
    ),
    (
        "PROJECT_KEY",
        "$.pullRequest.fromRef.repository.project.key",
        "Bitbucket project key",
    ),
    ("CHANGE_ID", "$.pullRequest.id", "PR number"),
    ("CHANGE_TARGET", "$.pullRequest.toRef.displayId", "PR target branch"),
    ("CHANGE_BRANCH", "$.pullRequest.fromRef.displayId", "PR source branch"),
    ("CHANGE_COMMIT", "$.pullRequest.fromRef.latestCommit", "PR source branch HEAD"),
    ("PUSH_BRANCH", "$.changes[0].ref.displayId", "Pushed branch (repo:refs_changed)"),
    ("PR_EVENT_TYPE", "$.eventKey", "Bitbucket event key"),
    ("ACTOR_NAME", "$.actor.displayName", "Bitbucket actor name"),
]

# Pipeline string parameters for the central job.
_STRING_PARAMS: list[tuple[str, str, str]] = [
    (
        "REPO_SLUG",
        "",
        "App repo slug — selects <slug>.groovy in central config dir "
        "($.pullRequest.fromRef.repository.slug)",
    ),
    (
        "PROJECT_KEY",
        "",
        "Bitbucket project key ($.pullRequest.fromRef.repository.project.key)",
    ),
    ("CHANGE_ID", "", "PR number — pr:opened / pr:from_ref_updated ($.pullRequest.id)"),
    ("CHANGE_TARGET", "master", "PR target branch ($.pullRequest.toRef.displayId)"),
    ("CHANGE_BRANCH", "", "PR source branch ($.pullRequest.fromRef.displayId)"),
    ("CHANGE_COMMIT", "", "PR HEAD commit ($.pullRequest.fromRef.latestCommit)"),
    ("PUSH_BRANCH", "", "Pushed branch — repo:refs_changed ($.changes[0].ref.id)"),
    ("PR_EVENT_TYPE", "", "Bitbucket event key ($.eventKey)"),
    ("REVIEW_PROFILE", "", "Named pipeline profile (optional, Parameterized Builds)"),
    ("REPLAY_OF_BUILD", "", "Internal: source build number for replay reruns"),
]

_CHOICE_PARAMS: list[tuple[str, list[str], str]] = [
    (
        "SUMMARY_PUBLISH_MODE",
        ["auto", "comment", "description", "off"],
        "How to publish the PR summary to Bitbucket",
    ),
]

# ---------------------------------------------------------------------------
# Dataclasses
# ---------------------------------------------------------------------------


@dataclass
class RegistryConfig:
    """Docker registry settings from _defaults.groovy."""

    pull_domain: str = ""
    pull_url: str = ""
    push_domain: str = ""
    credentials_id: str = ""


@dataclass
class CredentialsConfig:
    """Jenkins credential IDs from _defaults.groovy."""

    package-registry_credentials_id: str = ""
    github_copilot_credentials_id: str = ""
    bitbucket_token_credentials_id: str = ""
    bitbucket_ssh_credentials_id: str = ""
    gwt_token_credentials_id: str = ""
    prm_token_credentials_id: str = ""


@dataclass
class CentralConfig:
    """Parsed representation of majordomo-central-config/_defaults.groovy."""

    registry: RegistryConfig = field(default_factory=RegistryConfig)
    credentials: CredentialsConfig = field(default_factory=CredentialsConfig)


@dataclass
class RepoBitbucketConfig:
    """Bitbucket identity block from a per-repo config file."""

    project_key: str = ""
    repo_slug: str = ""
    clone_ssh_url: str = ""


@dataclass
class RepoConfig:
    """Parsed representation of majordomo-central-config/<slug>.groovy."""

    slug: str = ""
    bitbucket: RepoBitbucketConfig = field(default_factory=RepoBitbucketConfig)


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
    gwt_token_credentials_id: str = ""
    prm_token_credentials_id: str = ""
    dump_xml_path: str = ""


# ---------------------------------------------------------------------------
# Config parsing
# ---------------------------------------------------------------------------


def _strip_groovy_line_comments(source: str) -> str:
    """Strip // comments while preserving // inside quoted strings.

    Args:
        source: Groovy source text.

    Returns:
        Source with line comments removed.
    """
    out_lines: list[str] = []
    for line in source.splitlines():
        in_single = in_double = escaped = False
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
            elif ch == '"' and not in_single:
                in_double = not in_double
            elif (
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


def parse_defaults(config_path: Path) -> CentralConfig:
    """Parse majordomo-central-config/_defaults.groovy into a CentralConfig.

    Args:
        config_path: Path to the Groovy defaults file.

    Returns:
        CentralConfig populated with values from the file.

    Raises:
        FileNotFoundError: If the path does not exist.
    """
    source = _strip_groovy_line_comments(config_path.read_text(encoding="utf-8"))
    reg = _extract_block(source, "registry")
    cred = _extract_block(source, "credentials")
    return CentralConfig(
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


def parse_repo_config(config_path: Path) -> RepoConfig:
    """Parse a per-repo config file into a RepoConfig.

    Args:
        config_path: Path to the per-repo Groovy config file.

    Returns:
        RepoConfig populated with values from the file.

    Raises:
        FileNotFoundError: If the path does not exist.
    """
    source = _strip_groovy_line_comments(config_path.read_text(encoding="utf-8"))
    bb = _extract_block(source, "bitbucket")
    slug = config_path.stem  # filename without .groovy extension
    return RepoConfig(
        slug=slug,
        bitbucket=RepoBitbucketConfig(
            project_key=_extract_string(bb, "projectKey"),
            repo_slug=_extract_string(bb, "repoSlug"),
            clone_ssh_url=_extract_string(bb, "cloneSshUrl"),
        ),
    )


# ---------------------------------------------------------------------------
# Validation
# ---------------------------------------------------------------------------


def validate_defaults(cfg: CentralConfig) -> ValidationResult:
    """Check required fields in the org-wide defaults config.

    Args:
        cfg: Parsed CentralConfig to validate.

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
            "service account token used for PR status and comment callbacks"
        )
    if not cfg.credentials.bitbucket_ssh_credentials_id:
        result.errors.append(
            "credentials.bitbucketSshCredentialsId is required — "
            "SSH key used to clone app repos at runtime"
        )
    return result


def validate_repo_config(repo: RepoConfig) -> ValidationResult:
    """Check required fields in a per-repo config file.

    Args:
        repo: Parsed RepoConfig to validate.

    Returns:
        ValidationResult with any errors and warnings.
    """
    result = ValidationResult()
    if not repo.bitbucket.project_key:
        result.errors.append(f"[{repo.slug}] bitbucket.projectKey is required")
    if not repo.bitbucket.repo_slug:
        result.errors.append(f"[{repo.slug}] bitbucket.repoSlug is required")
    if not repo.bitbucket.clone_ssh_url:
        result.errors.append(
            f"[{repo.slug}] bitbucket.cloneSshUrl is required — "
            "used by the central pipeline to clone this repo at runtime"
        )
    elif repo.bitbucket.repo_slug and repo.bitbucket.repo_slug != repo.slug:
        result.warnings.append(
            f"[{repo.slug}] bitbucket.repoSlug '{repo.bitbucket.repo_slug}' "
            f"does not match filename slug '{repo.slug}' — "
            "the filename is used as the lookup key; ensure they match"
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
            folder: Jenkins folder path (slash-separated).
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
        return folder.strip("/").replace("/job/", "/")

    @staticmethod
    def _folder_url_path(folder: str) -> str:
        segments = [urllib.parse.quote(s, safe="") for s in folder.split("/") if s]
        return "/job/".join(segments)

    def _credential_store_urls(self, cred_id: str) -> list[str]:
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
            Dict with id and typeName, or None if the credential does not
            exist in any reachable store.

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
        """Check whether a credential ID exists in the Jenkins store.

        Args:
            cred_id: Jenkins credential ID to look up.

        Returns:
            True if the credential exists.
        """
        return self.credential_info(cred_id) is not None

    def job_exists(self, job_name: str, folder: str) -> bool:
        """Check whether a Jenkins job already exists.

        Args:
            job_name: Jenkins job name.
            folder: Jenkins folder containing the job.

        Returns:
            True if the job exists.
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
        """Overwrite an existing Jenkins job configuration.

        Args:
            job_name: Name of the Jenkins job to update.
            xml_config: Replacement Jenkins job config XML string.
            folder: Jenkins folder containing the job.
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

    def get_job_config(self, job_name: str, folder: str) -> str:
        """Fetch a Jenkins job's current config.xml.

        Args:
            job_name: Jenkins job name.
            folder: Jenkins folder containing the job.

        Returns:
            Raw XML config string.
        """
        enc_folder = self._folder_url_path(self._normalize_folder(folder))
        enc_name = urllib.parse.quote(job_name, safe="")
        url = f"{self._base}/job/{enc_folder}/job/{enc_name}/config.xml"
        return self._request(url).decode("utf-8")

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
            segments = [s for s in self._folder.split("/") if s]
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
            except urllib.error.HTTPError as exc:
                if exc.code == _HTTP_NOT_FOUND:
                    continue
                raise
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
    """Build the GWT variable extraction XML fragment for the central job.

    Returns:
        Joined GenericVariable XML elements.
    """
    parts: list[str] = []
    for name, expr, _desc in _GWT_VARS:
        parts.append(
            "        <org.jenkinsci.plugins.gwt.GenericVariable>\n"
            "          <expressionType>JSONPath</expressionType>\n"
            f"          <key>{_xml_escape(name)}</key>\n"
            f"          <value>{_xml_escape(expr)}</value>\n"
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
        parts.append(
            "        <hudson.model.StringParameterDefinition>\n"
            f"          <name>{name}</name>\n"
            f"          <defaultValue>{_xml_escape(default)}</defaultValue>\n"
            f"          <description>{_xml_escape(desc)}</description>\n"
            "          <trim>false</trim>\n"
            "        </hudson.model.StringParameterDefinition>"
        )
    for name, choices, desc in _CHOICE_PARAMS:
        choices_inner = "\n".join(f"            <string>{c}</string>" for c in choices)
        parts.append(
            "        <hudson.model.ChoiceParameterDefinition>\n"
            f"          <name>{name}</name>\n"
            '          <choices class="java.util.Arrays$ArrayList">\n'
            '            <a class="string-array">\n'
            f"{choices_inner}\n"
            "            </a>\n"
            "          </choices>\n"
            f"          <description>{_xml_escape(desc)}</description>\n"
            "        </hudson.model.ChoiceParameterDefinition>"
        )
    return "\n".join(parts)


def build_job_xml(
    repo_url: str,
    scm_cred_id: str,
    pipeline_branch: str,
    gwt_token_cred_id: str = "",
    prm_token_cred_id: str = "",
) -> str:
    """Build Jenkins pipeline job configuration XML for the central job.

    The central job can optionally use a GWT token credential configured in
    defaults (credentials.gwtTokenCredentialsId).

    Args:
        repo_url: Git repository SSH URL (this pipeline repo).
        scm_cred_id: Jenkins credential ID for Git checkout.
        pipeline_branch: SCM branch spec (e.g. ``*/main``).
        gwt_token_cred_id: Optional Jenkins credential ID for the Generic
            Webhook Trigger token.
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
    safe_gwt = _xml_escape(gwt_token_cred_id)
    safe_prm = _xml_escape(prm_token_cred_id)
    safe_branch = _xml_escape(pipeline_branch)
    safe_script = _xml_escape(_SCRIPT_PATH)
    return (
        "<?xml version='1.1' encoding='UTF-8'?>\n"
        '<flow-definition plugin="workflow-job">\n'
        "  <description>Majordomo — repository operations for evolving software."
        " (central) — created by setup-majordomo-central.py</description>\n"
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
        "          <causeString>Triggered by ${REPO_SLUG}: ${PR_EVENT_TYPE} in ${CHANGE_BRANCH} by ${ACTOR_NAME}</causeString>\n"
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
        "      <doGenerateSubmoduleConfigurations>false</doGenerateSubmoduleConfigurations>\n"
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
# CLI steps
# ---------------------------------------------------------------------------


def _check_registry_reachability(cfg: CentralConfig) -> None:
    """TCP-probe the configured registry domains on port 443.

    Args:
        cfg: Parsed CentralConfig with registry domains to probe.
    """
    domains = {
        "registry.pullDomain": cfg.registry.pull_domain,
        "registry.pushDomain": cfg.registry.push_domain,
    }
    print("\nChecking registry reachability ...")
    for label, domain in domains.items():
        if not domain:
            print(f"  ⏭️  {label}: (not set — skip)")
            continue
        try:
            with socket.create_connection(
                (domain, _HTTPS_PORT), timeout=_CONNECT_TIMEOUT
            ):
                print(f"  ✅ {label}: {domain}")
        except OSError:
            print(
                f"  ⚠️  {label}: {domain} — TCP connect failed "
                "(may still be reachable from the Jenkins agent)"
            )


def _run_defaults_validation(config_path: Path) -> CentralConfig:
    """Parse and validate _defaults.groovy, exiting on errors.

    Args:
        config_path: Path to the defaults Groovy file.

    Returns:
        Parsed and validated CentralConfig.
    """
    if not config_path.exists():
        msg = inspect.cleandoc(f"""
            Defaults file not found: {config_path}
            Copy the template and edit it:
              cp example.majordomo-central-config/_defaults.groovy \\
                 majordomo-central-config/_defaults.groovy
        """)
        print(msg)
        sys.exit(1)

    print(f"Parsing {config_path} ...")
    cfg = parse_defaults(config_path)

    print("\nValidating defaults fields ...")
    result = validate_defaults(cfg)
    for err in result.errors:
        print(f"  ❌ ERROR   {err}")
    for warn in result.warnings:
        print(f"  ⚠️  WARN    {warn}")
    if result.ok and not result.warnings:
        print("  ✅ All fields OK")

    if not result.ok:
        print(f"\n{len(result.errors)} error(s) — fix config and re-run.")
        sys.exit(1)

    return cfg


def _run_repo_validation(slug: str) -> RepoConfig:
    """Parse and validate a per-repo config file, exiting on errors.

    Args:
        slug: Repo slug matching the config filename.

    Returns:
        Parsed and validated RepoConfig.
    """
    config_path = _CENTRAL_CONFIG_DIR / f"{slug}.groovy"
    if not config_path.exists():
        available_str = ", ".join(
            p.stem
            for p in _CENTRAL_CONFIG_DIR.glob("*.groovy")
            if not p.name.startswith("_") and not p.name.startswith("example")
        )
        msg = inspect.cleandoc(f"""
            Repo config not found: {config_path}
            Available repos: {available_str or "(none)"}
            Create one from the template:
              cp example.majordomo-central-config/example.repo-config.groovy \\
                 majordomo-central-config/{slug}.groovy
        """)
        print(msg)
        sys.exit(1)

    print(f"\nParsing {config_path} ...")
    repo = parse_repo_config(config_path)

    print(f"Validating repo config [{slug}] ...")
    result = validate_repo_config(repo)
    for err in result.errors:
        print(f"  ❌ ERROR   {err}")
    for warn in result.warnings:
        print(f"  ⚠️  WARN    {warn}")
    if result.ok and not result.warnings:
        print(f"  ✅ [{slug}] OK")

    if not result.ok:
        print(f"\n{len(result.errors)} error(s) — fix repo config and re-run.")
        sys.exit(1)

    return repo


def _print_candidates(client: JenkinsClient, type_name: str) -> None:
    """Print credential IDs of the given type as suggestions.

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
    cfg: CentralConfig,
    scm_credentials_id: str,
) -> None:
    """Verify all configured credential IDs exist in the Jenkins store.

    Args:
        client: Authenticated Jenkins client.
        cfg: Parsed CentralConfig with credential IDs to check.
        scm_credentials_id: SSH credential used for SCM checkout.
    """
    print("\nVerifying credentials in Jenkins ...")
    checks: list[tuple[str, str]] = [
        ("registry.credentialsId", cfg.registry.credentials_id),
        (
            "credentials.githubCopilotCredentialsId",
            cfg.credentials.github_copilot_credentials_id,
        ),
        (
            "credentials.package-registryCredentialsId",
            cfg.credentials.package-registry_credentials_id,
        ),
        (
            "credentials.bitbucketTokenCredentialsId",
            cfg.credentials.bitbucket_token_credentials_id,
        ),
        (
            "credentials.bitbucketSshCredentialsId",
            cfg.credentials.bitbucket_ssh_credentials_id,
        ),
        (
            "credentials.gwtTokenCredentialsId",
            cfg.credentials.gwt_token_credentials_id,
        ),
        (
            "credentials.prmTokenCredentialsId",
            cfg.credentials.prm_token_credentials_id,
        ),
        ("--scm-credentials-id", scm_credentials_id),
    ]
    all_ok = True
    for cfg_key, cred_id in checks:
        if not cred_id:
            print(f"  ⏭️  {cfg_key}: (not set — skip)")
            expected = _CRED_EXPECTED_TYPES.get(cfg_key, "")
            if expected:
                _print_candidates(client, expected)
            continue
        try:
            info = client.credential_info(cred_id)
        except urllib.error.HTTPError as exc:
            print(f"  ⚠️  {cfg_key}: API error HTTP {exc.code} — skip")
            continue
        if info is None:
            print(f"  ❌ {cfg_key}: {cred_id} — not found in Jenkins")
            all_ok = False
            expected = _CRED_EXPECTED_TYPES.get(cfg_key, "")
            if expected:
                _print_candidates(client, expected)
            continue
        expected = _CRED_EXPECTED_TYPES.get(cfg_key, "")
        actual = info.get("typeName", "")
        if expected and actual and actual != expected:
            print(
                f"  ⚠️  {cfg_key}: {cred_id} — "
                f"wrong type '{actual}' (expected '{expected}')"
            )
            _print_candidates(client, expected)
            all_ok = False
        else:
            print(f"  ✅ {cfg_key}: {cred_id}")
    if not all_ok:
        print(
            "\nOne or more credentials not found or wrong type —"
            " fix in Jenkins and re-run."
        )
        sys.exit(1)


def _dump_or_submit_xml(xml_config: str, dump_xml_path: str, label: str) -> bool:
    """Write XML to a file when dump_xml_path is set; return True if dumped.

    Args:
        xml_config: Jenkins job config XML string.
        dump_xml_path: File path to write, or empty string to skip.
        label: Short label for the printed message.

    Returns:
        True when XML was written to disk.
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


def _submit_xml_request(action_label: str, submit_fn: Callable[[], None]) -> None:
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
            pass
        print(f"  ❌ Job {action_label} failed (HTTP {exc.code}).")
        if body:
            log_path = Path("setup-majordomo-central-error.html")
            log_path.write_text(body, encoding="utf-8")
            print(f"     Full response written to {log_path}")
        sys.exit(1)


def _run_job_setup(client: JenkinsClient, setup: JobSetupConfig) -> None:
    """Check whether the central Jenkins job exists and optionally create/update it.

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

    xml_config = build_job_xml(
        repo_url=setup.repo_url,
        scm_cred_id=setup.scm_credentials_id,
        pipeline_branch=setup.pipeline_branch,
        gwt_token_cred_id=setup.gwt_token_credentials_id,
        prm_token_cred_id=setup.prm_token_credentials_id,
    )

    if exists:
        if not setup.update_if_exists:
            print(f"  ✅ Job '{job_path}' already exists — nothing to create.")
            print("  Re-run with --update-job to overwrite its configuration.")
            return
        if _dump_or_submit_xml(xml_config, setup.dump_xml_path, "update"):
            return
        print(f"  Updating job '{job_path}' ...")
        _submit_xml_request(
            "update",
            lambda: client.update_job(setup.job_name, xml_config, setup.folder),
        )
        print(f"  ✅ Job '{job_path}' updated.")
        return

    if not setup.create_if_missing:
        msg = inspect.cleandoc(f"""
            Job '{job_path}' not found.
            Re-run with --create-job to create it.
        """)
        print(msg)
        sys.exit(1)

    if _dump_or_submit_xml(xml_config, setup.dump_xml_path, "create"):
        return

    print(f"  Creating job '{job_path}' ...")
    _submit_xml_request(
        "creation", lambda: client.create_job(setup.job_name, xml_config, setup.folder)
    )

    folder_url_path = client._folder_url_path(client._normalize_folder(setup.folder))
    enc_name = urllib.parse.quote(setup.job_name)
    job_url = f"{client.base_url}/job/{folder_url_path}/job/{enc_name}"
    prm_cred_id = setup.prm_token_credentials_id
    trigger_url = (
        f"{job_url}/buildWithParameters?token={prm_cred_id}" if prm_cred_id else ""
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
        ✅ Job '{job_path}' created.

        Next steps:
          1. Set the GWT token on the job trigger.
          2. Add Bitbucket webhooks on each app repo → this job's GWT endpoint.
             Events: pr:opened, pr:from_ref_updated, repo:refs_changed
             Required GWT vars: REPO_SLUG, PROJECT_KEY
          3. Onboard a repo:
             cp example.majordomo-central-config/example.repo-config.groovy \\
                majordomo-central-config/<slug>.groovy
          4. Open a PR on a managed repo to trigger a test build.
             Job URL: {job_url}
    """)
        + prm_section
    )
    print(msg)


# ---------------------------------------------------------------------------
# Arg parsing + main
# ---------------------------------------------------------------------------


def _get_git_origin_url() -> str:
    """Return the remote origin URL of the current git repo, or empty string.

    Returns:
        Remote origin URL string, or empty string if git command fails.
    """
    try:
        result = subprocess.run(
            ["git", "remote", "get-url", "origin"],
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except FileNotFoundError:
        pass
    return ""


def _parse_args() -> argparse.Namespace:
    """Parse command-line arguments.

    Returns:
        Parsed argument namespace.
    """
    parser = argparse.ArgumentParser(
        description=(
            "Set up the central Majordomo review job "
            "and validate majordomo-central-config/ files."
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )

    # Validation
    parser.add_argument(
        "--defaults-path",
        default=str(_DEFAULTS_FILE),
        metavar="PATH",
        help=f"Path to _defaults.groovy (default: {_DEFAULTS_FILE})",
    )
    parser.add_argument(
        "--validate-repo",
        metavar="REPO_SLUG",
        help="Also validate the per-repo config for this slug.",
    )
    parser.add_argument(
        "--validate-only",
        action="store_true",
        help="Run config validation only — do not connect to Jenkins.",
    )

    # Jenkins connection
    parser.add_argument(
        "--jenkins-url",
        default=os.environ.get("JENKINS_URL", _DEFAULT_JENKINS_URL),
        metavar="URL",
        help="Jenkins base URL (env: JENKINS_URL).",
    )
    parser.add_argument(
        "--username",
        metavar="USER",
        help="Jenkins username (env: REGISTRY_USER).",
    )
    parser.add_argument(
        "--api-token",
        default=os.environ.get("JENKINS_API_TOKEN", ""),
        metavar="TOKEN",
        help="Jenkins API token (env: JENKINS_API_TOKEN).",
    )

    # Job identity
    parser.add_argument(
        "--job-name",
        default="copilot-central",
        metavar="NAME",
        help="Jenkins job name (default: copilot-central).",
    )
    parser.add_argument(
        "--folder",
        default="",
        metavar="FOLDER",
        help="Jenkins folder path, e.g. MyOrg/Non-Prod (default: root).",
    )

    # SCM / pipeline source
    parser.add_argument(
        "--repo-url",
        default=_get_git_origin_url(),
        metavar="URL",
        help="SSH URL for the central pipeline repo (default: git remote origin).",
    )
    parser.add_argument(
        "--pipeline-branch",
        default=_DEFAULT_BRANCH,
        metavar="BRANCH",
        help=f"SCM branch spec (default: {_DEFAULT_BRANCH}).",
    )
    parser.add_argument(
        "--scm-credentials-id",
        default="",
        metavar="CRED_ID",
        help="Jenkins SSH credential ID used to clone this pipeline repo.",
    )

    # Job lifecycle
    parser.add_argument(
        "--create-job",
        action="store_true",
        help="Create the Jenkins job if it does not already exist.",
    )
    parser.add_argument(
        "--update-job",
        action="store_true",
        help="Overwrite an existing job's configuration.",
    )
    parser.add_argument(
        "--dump-xml",
        metavar="PATH",
        default="",
        help="Write the generated job XML to this path (instead of submitting).",
    )

    return parser.parse_args()


def main() -> None:
    """Entry point for the central Jenkins setup script."""
    args = _parse_args()
    defaults_path = Path(args.defaults_path)
    env_user = os.environ.get("REGISTRY_USER", "").split("@")[0]
    username: str = str(args.username or "") or env_user

    # ── 1. Validate org defaults ──────────────────────────────────────────
    cfg = _run_defaults_validation(defaults_path)

    # ── 2. Validate per-repo config (optional) ────────────────────────────
    if args.validate_repo:
        _run_repo_validation(args.validate_repo)

    if args.validate_only:
        print("\nConfig validation complete.")
        return

    # ── 3. Check Jenkins credentials are supplied ─────────────────────────
    if not username or not args.api_token:
        print(
            "ERROR: --username and --api-token "
            "(or env vars REGISTRY_USER / JENKINS_API_TOKEN) "
            "are required when not using --validate-only."
        )
        sys.exit(1)

    client = JenkinsClient(
        base_url=args.jenkins_url,
        username=username,
        api_token=args.api_token,
        folder=args.folder,
    )

    # ── 4. Registry reachability (advisory) ───────────────────────────────
    _check_registry_reachability(cfg)

    # ── 5. Credential verification ────────────────────────────────────────
    scm_cred_id = (
        args.scm_credentials_id or cfg.credentials.bitbucket_ssh_credentials_id
    )
    _run_credential_verification(client, cfg, scm_cred_id)

    # ── 6. Job creation / update ──────────────────────────────────────────
    if not args.repo_url:
        print(
            "ERROR: --repo-url is required (could not detect from git remote). "
            "Pass the SSH URL for this pipeline repository."
        )
        sys.exit(1)

    setup = JobSetupConfig(
        job_name=args.job_name,
        folder=args.folder,
        repo_url=args.repo_url,
        pipeline_branch=args.pipeline_branch,
        scm_credentials_id=scm_cred_id,
        create_if_missing=args.create_job,
        update_if_exists=args.update_job,
        gwt_token_credentials_id=cfg.credentials.gwt_token_credentials_id,
        prm_token_credentials_id=cfg.credentials.prm_token_credentials_id,
        dump_xml_path=args.dump_xml,
    )
    _run_job_setup(client, setup)


if __name__ == "__main__":
    main()
