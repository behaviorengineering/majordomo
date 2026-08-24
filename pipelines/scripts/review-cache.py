"""Cluster cache helpers for pre-analysis cache gating and cache lookup.

This script manages git-tracked cluster analysis cache files named as:
``analysis-<cluster_sha>.<ext>``

Supported commands:
- ``precheck``: resolve retention, prune expired cache files, validate metadata,
  and build an in-memory index keyed by ``cluster_sha``.
- ``lookup``: evaluate whether a cluster cache entry is a valid hit for the
  current run context.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path

_FRONTMATTER_DELIM = "---"
_CACHE_FILE_PATTERN = "analysis-*.*"
_TIMESTAMP_FMT = "%Y-%m-%dT%H:%M:%SZ"

_REQUIRED_FIELDS = (
    "cluster_sha",
    "fingerprint_version",
    "cluster_files",
    "cluster_files_hash",
    "model_id",
    "instruction_bundle_hash",
    "prompt_template_hash",
    "scoring_rubric_hash",
    "output_schema_version",
    "analysis_payload_hash",
    "created_at",
)

_VALID_KEY_RE = re.compile(r"^[a-z_][a-z0-9_]*$")
_ANALYSIS_NAME_RE = re.compile(r"^analysis-([a-f0-9]{64})\.[A-Za-z0-9]+$")
_CLUSTER_SHA_RE = re.compile(r"^[a-f0-9]{64}$")
_STORE_METADATA_ORDER = (
    "cluster_sha",
    "skill_name",
    "fingerprint_version",
    "cluster_files",
    "cluster_files_hash",
    "model_id",
    "model_revision",
    "instruction_bundle_hash",
    "prompt_template_hash",
    "scoring_rubric_hash",
    "output_schema_version",
    "analysis_payload_hash",
    "markdown_artifact_file",
    "markdown_artifact_hash",
    "markdown_artifact_count",
    "created_at",
)


@dataclass(frozen=True)
class _CacheRecord:
    file_path: Path
    metadata: dict[str, str | list[str]]
    created_at: datetime


def _normalize_cluster_files(cluster_files: list[str]) -> list[str]:
    normalized = [item.strip().replace("\\", "/") for item in cluster_files]
    filtered = [item for item in normalized if item]
    return sorted(filtered)


def _compute_cluster_files_hash(cluster_files: list[str]) -> str:
    normalized = _normalize_cluster_files(cluster_files)
    joined = "\n".join(normalized)
    return hashlib.sha256(joined.encode("utf-8")).hexdigest()


def _strip_wrapped_quotes(raw_value: str) -> str:
    stripped = raw_value.strip()
    if (
        len(stripped) >= 2
        and stripped[0] == stripped[-1]
        and stripped[0]
        in {
            '"',
            "'",
        }
    ):
        return stripped[1:-1]
    return stripped


def _parse_frontmatter_block(block_text: str) -> dict[str, str | list[str]]:
    metadata: dict[str, str | list[str]] = {}
    current_list_key = ""

    for raw_line in block_text.splitlines():
        line = raw_line.rstrip()
        if not line.strip():
            continue

        list_match = re.match(r"^\s*-\s+(.*)$", line)
        if list_match:
            if not current_list_key:
                raise ValueError("List item appears before a list key")
            current_value = metadata.get(current_list_key)
            if not isinstance(current_value, list):
                raise ValueError(f"Key '{current_list_key}' is not a list")
            current_value.append(_strip_wrapped_quotes(list_match.group(1)))
            continue

        current_list_key = ""
        key_match = re.match(r"^([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*)$", line)
        if not key_match:
            raise ValueError(f"Unsupported frontmatter line: {line!r}")

        key_name = key_match.group(1)
        if not _VALID_KEY_RE.match(key_name):
            raise ValueError(f"Invalid metadata key: {key_name!r}")

        value_raw = key_match.group(2)
        if value_raw == "":
            metadata[key_name] = []
            current_list_key = key_name
            continue

        metadata[key_name] = _strip_wrapped_quotes(value_raw)

    return metadata


def _extract_frontmatter(content: str) -> tuple[dict[str, str | list[str]], str]:
    lines = content.splitlines()
    if not lines or lines[0].strip() != _FRONTMATTER_DELIM:
        raise ValueError("Missing frontmatter start delimiter")

    end_index = None
    for index, current_line in enumerate(lines[1:], start=1):
        if current_line.strip() == _FRONTMATTER_DELIM:
            end_index = index
            break

    if end_index is None:
        raise ValueError("Missing frontmatter end delimiter")

    block_text = "\n".join(lines[1:end_index])
    body_text = "\n".join(lines[end_index + 1 :])
    metadata = _parse_frontmatter_block(block_text)
    return metadata, body_text


def _parse_utc_timestamp(raw_timestamp: str) -> datetime:
    parsed = datetime.strptime(raw_timestamp, _TIMESTAMP_FMT)
    return parsed.replace(tzinfo=UTC)


def _validate_cluster_sha_field(metadata: dict[str, str | list[str]]) -> list[str]:
    errors: list[str] = []
    cluster_sha = metadata.get("cluster_sha")

    if isinstance(cluster_sha, str):
        if not re.fullmatch(r"[a-f0-9]{64}", cluster_sha):
            errors.append("cluster_sha must be a lowercase 64-char hex sha256")
        return errors

    if cluster_sha is not None:
        errors.append("cluster_sha must be a string")

    return errors


def _validate_cluster_files_fields(metadata: dict[str, str | list[str]]) -> list[str]:
    errors: list[str] = []
    cluster_files = metadata.get("cluster_files")

    if not isinstance(cluster_files, list):
        if cluster_files is not None:
            errors.append("cluster_files must be a list")
        return errors

    normalized = _normalize_cluster_files([str(item) for item in cluster_files])
    if not normalized:
        errors.append("cluster_files must contain at least one path")
        return errors

    expected_hash = _compute_cluster_files_hash(normalized)
    cluster_files_hash = metadata.get("cluster_files_hash")

    if not isinstance(cluster_files_hash, str):
        errors.append("cluster_files_hash must be a string")
        return errors

    if cluster_files_hash != expected_hash:
        errors.append("cluster_files_hash does not match cluster_files")

    return errors


def _validate_created_at_field(metadata: dict[str, str | list[str]]) -> list[str]:
    errors: list[str] = []
    created_at = metadata.get("created_at")

    if isinstance(created_at, str):
        try:
            _parse_utc_timestamp(created_at)
        except ValueError:
            errors.append("created_at must use UTC format YYYY-MM-DDTHH:MM:SSZ")
        return errors

    if created_at is not None:
        errors.append("created_at must be a string")

    return errors


def _validate_metadata(metadata: dict[str, str | list[str]]) -> list[str]:
    errors: list[str] = []

    for field_name in _REQUIRED_FIELDS:
        if field_name not in metadata:
            errors.append(f"missing field '{field_name}'")

    errors.extend(_validate_cluster_sha_field(metadata))
    errors.extend(_validate_cluster_files_fields(metadata))
    errors.extend(_validate_created_at_field(metadata))

    return errors


def _load_cache_record(file_path: Path) -> tuple[_CacheRecord | None, list[str]]:
    try:
        content = file_path.read_text(encoding="utf-8", errors="replace")
    except OSError as err:
        return None, [f"read failure: {err}"]

    try:
        metadata, _ = _extract_frontmatter(content)
    except ValueError as err:
        return None, [str(err)]

    errors = _validate_metadata(metadata)
    if errors:
        return None, errors

    created_at_value = metadata.get("created_at")
    if not isinstance(created_at_value, str):
        return None, ["created_at must be a string"]

    created_at = _parse_utc_timestamp(created_at_value)
    return _CacheRecord(
        file_path=file_path, metadata=metadata, created_at=created_at
    ), []


def _resolve_retention_days(
    project_days: int | None,
    central_days: int | None,
    global_days: int,
    min_days: int,
) -> tuple[int, str]:
    if min_days < 0:
        raise ValueError("min retention days must be non-negative")
    if global_days < 0:
        raise ValueError("global retention days must be non-negative")

    value_source = "global"
    resolved = global_days

    if central_days is not None:
        if central_days < 0:
            raise ValueError("central retention days must be non-negative")
        resolved = central_days
        value_source = "central"

    if project_days is not None:
        if project_days < 0:
            raise ValueError("project retention days must be non-negative")
        resolved = project_days
        value_source = "project"

    resolved = max(resolved, min_days)

    return resolved, value_source


def _collect_cache_files(cache_dir: Path) -> list[Path]:
    return sorted(
        path for path in cache_dir.rglob(_CACHE_FILE_PATTERN) if path.is_file()
    )


def _build_index(records: list[_CacheRecord]) -> dict[str, _CacheRecord]:
    index: dict[str, _CacheRecord] = {}
    for record in records:
        cluster_sha = record.metadata.get("cluster_sha")
        if not isinstance(cluster_sha, str):
            continue

        skill_name = record.metadata.get("skill_name")
        if isinstance(skill_name, str) and skill_name.strip():
            index_key = f"{skill_name}:{cluster_sha}"
        else:
            index_key = cluster_sha

        existing = index.get(index_key)
        if existing is None or record.created_at >= existing.created_at:
            index[index_key] = record
    return index


def _to_rel(file_path: Path, root_dir: Path) -> str:
    return file_path.relative_to(root_dir).as_posix()


def _precheck_command(args: argparse.Namespace) -> int:
    cache_dir = args.cache_dir.resolve()
    if not cache_dir.exists():
        cache_dir.mkdir(parents=True, exist_ok=True)

    retention_days, source = _resolve_retention_days(
        project_days=args.project_retention_days,
        central_days=args.central_retention_days,
        global_days=args.global_retention_days,
        min_days=args.min_retention_days,
    )

    now_utc = datetime.now(UTC)
    cutoff = now_utc - timedelta(days=retention_days)

    expired_files: list[str] = []
    invalid_files: dict[str, list[str]] = {}
    valid_records: list[_CacheRecord] = []

    for file_path in _collect_cache_files(cache_dir):
        record, errors = _load_cache_record(file_path)

        if record is None:
            invalid_files[_to_rel(file_path, cache_dir)] = errors
            continue

        if record.created_at < cutoff:
            expired_files.append(_to_rel(file_path, cache_dir))
            try:
                file_path.unlink()
            except OSError as err:
                invalid_files[_to_rel(file_path, cache_dir)] = [
                    f"failed to delete expired file: {err}"
                ]
            continue

        file_name = file_path.name
        match = _ANALYSIS_NAME_RE.match(file_name)
        if not match:
            invalid_files[_to_rel(file_path, cache_dir)] = [
                "file name must match analysis-<sha256>.<ext>"
            ]
            continue

        cluster_sha = record.metadata.get("cluster_sha")
        if isinstance(cluster_sha, str) and cluster_sha != match.group(1):
            invalid_files[_to_rel(file_path, cache_dir)] = [
                "cluster_sha does not match file name hash"
            ]
            continue

        valid_records.append(record)

    index_records = _build_index(valid_records)
    index_entries: dict[str, dict[str, str | list[str]]] = {}

    for _, record in index_records.items():
        metadata_for_index: dict[str, str | list[str]] = {
            key_name: value
            for key_name, value in record.metadata.items()
            if key_name
            in {
                "cluster_sha",
                "skill_name",
                "fingerprint_version",
                "cluster_files",
                "cluster_files_hash",
                "model_id",
                "model_revision",
                "instruction_bundle_hash",
                "prompt_template_hash",
                "scoring_rubric_hash",
                "output_schema_version",
                "analysis_payload_hash",
                "markdown_artifact_file",
                "markdown_artifact_hash",
                "markdown_artifact_count",
                "created_at",
            }
        }

        metadata_for_index["file"] = _to_rel(record.file_path, cache_dir)
        skill_name = metadata_for_index.get("skill_name")
        cluster_sha = metadata_for_index.get("cluster_sha")
        if (
            isinstance(skill_name, str)
            and skill_name.strip()
            and isinstance(cluster_sha, str)
        ):
            index_entries[f"{skill_name}:{cluster_sha}"] = metadata_for_index
        elif isinstance(cluster_sha, str):
            index_entries[cluster_sha] = metadata_for_index

    result = {
        "project_id": args.project_id,
        "cache_dir": cache_dir.as_posix(),
        "retention_days": retention_days,
        "retention_source": source,
        "scanned_files": len(_collect_cache_files(cache_dir)),
        "expired_deleted": len(expired_files),
        "expired_files": expired_files,
        "invalid_files": invalid_files,
        "valid_entries": len(index_entries),
        "index": index_entries,
    }

    output_json = json.dumps(result, indent=2, sort_keys=True)
    if args.index_out is not None:
        args.index_out.parent.mkdir(parents=True, exist_ok=True)
        args.index_out.write_text(output_json + "\n", encoding="utf-8")

    print(output_json)
    return 0


def _parse_cluster_files_argument(
    cluster_files: list[str],
    cluster_files_file: Path | None,
) -> list[str]:
    merged: list[str] = [item for item in cluster_files if item.strip()]

    if cluster_files_file is not None:
        raw_lines = cluster_files_file.read_text(encoding="utf-8", errors="replace")
        for line in raw_lines.splitlines():
            current = line.strip()
            if current:
                merged.append(current)

    normalized = _normalize_cluster_files(merged)
    if not normalized:
        raise ValueError("cluster file list is empty")
    return normalized


def _format_frontmatter(metadata: dict[str, str | list[str]]) -> str:
    lines: list[str] = [_FRONTMATTER_DELIM]
    for key_name in _STORE_METADATA_ORDER:
        if key_name not in metadata:
            continue

        value = metadata[key_name]
        if isinstance(value, list):
            lines.append(f"{key_name}:")
            for item in value:
                encoded = json.dumps(item)
                lines.append(f"  - {encoded}")
            continue

        encoded_value = json.dumps(value)
        lines.append(f"{key_name}: {encoded_value}")

    lines.append(_FRONTMATTER_DELIM)
    return "\n".join(lines)


def _load_payload_text(analysis_file: Path) -> str:
    return analysis_file.read_text(encoding="utf-8", errors="replace")


def _load_manifest_slugs(analysis_file: Path) -> list[str]:
    raw = analysis_file.read_text(encoding="utf-8", errors="replace")
    parsed = json.loads(raw)
    if not isinstance(parsed, dict):
        raise ValueError("analysis file must contain a JSON object")

    reviewable = parsed.get("reviewable", [])
    if not isinstance(reviewable, list):
        raise ValueError("analysis file reviewable field must be a list")

    slugs = {
        str(task.get("slug", "")).strip()
        for task in reviewable
        if isinstance(task, dict)
    }
    return sorted(slug for slug in slugs if slug)


def _collect_markdown_artifact(
    analysis_file: Path,
    reports_dir: Path,
    artifact_files: list[str],
) -> tuple[dict[str, str], str]:
    slugs = _load_manifest_slugs(analysis_file)
    requested_files = {f"{slug}.md" for slug in slugs}
    requested_files.update(name.strip() for name in artifact_files if name.strip())

    markdown_files: dict[str, str] = {}
    for report_name in sorted(requested_files):
        if "/" in report_name or "\\" in report_name:
            continue
        if not report_name.endswith(".md"):
            continue
        report_path = reports_dir / report_name
        if not report_path.is_file():
            continue
        markdown_files[report_name] = report_path.read_text(
            encoding="utf-8", errors="replace"
        )

    artifact_hash = ""
    if markdown_files:
        canonical = json.dumps(markdown_files, sort_keys=True, ensure_ascii=False)
        artifact_hash = hashlib.sha256(canonical.encode("utf-8")).hexdigest()

    return markdown_files, artifact_hash


def _write_markdown_artifact_file(
    artifact_file_path: Path,
    markdown_files: dict[str, str],
) -> None:
    artifact_payload = {"files": markdown_files}
    artifact_file_path.write_text(
        json.dumps(artifact_payload, sort_keys=True, ensure_ascii=False, indent=2)
        + "\n",
        encoding="utf-8",
    )


def _parse_optional_int(value: str | list[str] | None) -> int:
    if isinstance(value, str):
        return int(value)
    raise ValueError("expected integer metadata value")


def _is_existing_cache_payload_unchanged(
    existing_record: _CacheRecord,
    payload_hash: str,
    markdown_artifact_file: str,
    markdown_artifact_hash: str,
    markdown_artifact_count: int,
) -> bool:
    existing_payload_hash = existing_record.metadata.get("analysis_payload_hash")
    existing_artifact_hash = existing_record.metadata.get("markdown_artifact_hash")
    existing_artifact_count = existing_record.metadata.get("markdown_artifact_count")

    if markdown_artifact_file:
        try:
            artifact_unchanged = (
                existing_artifact_hash == markdown_artifact_hash
                and _parse_optional_int(existing_artifact_count)
                == markdown_artifact_count
            )
        except Exception as err:
            if isinstance(err, (ValueError, TypeError)):
                artifact_unchanged = False
            else:
                raise
    else:
        artifact_unchanged = existing_artifact_hash in (None, "")

    return bool(existing_payload_hash == payload_hash and artifact_unchanged)


def _store_command(args: argparse.Namespace) -> int:
    if not _CLUSTER_SHA_RE.fullmatch(args.cluster_sha):
        raise ValueError("cluster_sha must be a lowercase 64-char hex sha256")

    cache_dir = args.cache_dir.resolve()
    cache_dir.mkdir(parents=True, exist_ok=True)

    cluster_files = _parse_cluster_files_argument(
        cluster_files=args.cluster_file,
        cluster_files_file=args.cluster_files_file,
    )
    cluster_files_hash = _compute_cluster_files_hash(cluster_files)

    payload_text = _load_payload_text(args.analysis_file)
    payload_hash = hashlib.sha256(payload_text.encode("utf-8")).hexdigest()

    markdown_files: dict[str, str] = {}
    markdown_artifact_hash = ""
    markdown_artifact_count = 0
    markdown_artifact_file = ""
    if args.reports_dir is not None:
        markdown_files, markdown_artifact_hash = _collect_markdown_artifact(
            args.analysis_file,
            args.reports_dir,
            args.artifact_file,
        )
        markdown_artifact_count = len(markdown_files)
        if markdown_artifact_count > 0:
            markdown_artifact_file = (
                f"{args.skill_name}/markdown-{args.cluster_sha}.json"
            )

    created_at = datetime.now(UTC).strftime(_TIMESTAMP_FMT)

    metadata: dict[str, str | list[str]] = {
        "cluster_sha": args.cluster_sha,
        "skill_name": args.skill_name,
        "fingerprint_version": args.fingerprint_version,
        "cluster_files": cluster_files,
        "cluster_files_hash": cluster_files_hash,
        "model_id": args.model_id,
        "instruction_bundle_hash": args.instruction_bundle_hash,
        "prompt_template_hash": args.prompt_template_hash,
        "scoring_rubric_hash": args.scoring_rubric_hash,
        "output_schema_version": args.output_schema_version,
        "analysis_payload_hash": payload_hash,
        "created_at": created_at,
    }
    if args.model_revision:
        metadata["model_revision"] = args.model_revision

    if markdown_artifact_file:
        metadata["markdown_artifact_file"] = markdown_artifact_file
        metadata["markdown_artifact_hash"] = markdown_artifact_hash
        metadata["markdown_artifact_count"] = str(markdown_artifact_count)

    skill_dir = cache_dir / args.skill_name
    skill_dir.mkdir(parents=True, exist_ok=True)
    output_path = skill_dir / f"analysis-{args.cluster_sha}.json"

    if output_path.exists():
        existing_record, _ = _load_cache_record(output_path)
        if existing_record is not None and _is_existing_cache_payload_unchanged(
            existing_record,
            payload_hash,
            markdown_artifact_file,
            markdown_artifact_hash,
            markdown_artifact_count,
        ):
            print(
                json.dumps(
                    {
                        "written": False,
                        "reason": "payload-unchanged",
                        "cluster_sha": args.cluster_sha,
                        "file": _to_rel(output_path, cache_dir),
                    },
                    sort_keys=True,
                )
            )
            return 0

    if markdown_artifact_file:
        artifact_path = cache_dir / markdown_artifact_file
        artifact_path.parent.mkdir(parents=True, exist_ok=True)
        _write_markdown_artifact_file(artifact_path, markdown_files)

    frontmatter = _format_frontmatter(metadata)
    output_text = f"{frontmatter}\n{payload_text}"
    output_path.write_text(output_text, encoding="utf-8")

    print(
        json.dumps(
            {
                "written": True,
                "cluster_sha": args.cluster_sha,
                "file": _to_rel(output_path, cache_dir),
                "analysis_payload_hash": payload_hash,
                "markdown_artifact_file": markdown_artifact_file,
                "markdown_artifact_count": markdown_artifact_count,
            },
            sort_keys=True,
        )
    )
    return 0


def _restore_command(args: argparse.Namespace) -> int:
    cache_dir = args.cache_dir.resolve()
    entry_path = (cache_dir / args.entry_file).resolve()

    try:
        entry_path.relative_to(cache_dir)
    except ValueError as err:
        raise ValueError("entry-file must resolve inside cache-dir") from err

    record, errors = _load_cache_record(entry_path)
    if record is None:
        joined_errors = "; ".join(errors or ["unknown error"])
        raise ValueError(f"cache entry file is invalid: {joined_errors}")

    artifact_rel = record.metadata.get("markdown_artifact_file")
    if not isinstance(artifact_rel, str) or not artifact_rel.strip():
        print(
            json.dumps(
                {
                    "restored": False,
                    "reason": "no-markdown-artifact",
                    "entry_file": args.entry_file,
                },
                sort_keys=True,
            )
        )
        return 0

    artifact_path = (cache_dir / artifact_rel).resolve()
    try:
        artifact_path.relative_to(cache_dir)
    except ValueError as err:
        raise ValueError("markdown artifact path resolves outside cache-dir") from err

    artifact_payload = json.loads(artifact_path.read_text(encoding="utf-8"))
    files_map = artifact_payload.get("files")
    if not isinstance(files_map, dict):
        raise ValueError("markdown artifact payload missing files map")

    output_dir = args.output_dir.resolve()
    output_dir.mkdir(parents=True, exist_ok=True)

    restored = 0
    for file_name, file_content in files_map.items():
        if not isinstance(file_name, str) or not file_name.endswith(".md"):
            continue
        if "/" in file_name or "\\" in file_name:
            continue
        if not isinstance(file_content, str):
            continue

        (output_dir / file_name).write_text(file_content, encoding="utf-8")
        restored += 1

    print(
        json.dumps(
            {
                "restored": restored > 0,
                "restored_count": restored,
                "entry_file": args.entry_file,
                "artifact_file": artifact_rel,
            },
            sort_keys=True,
        )
    )
    return 0


def _resolve_lookup_entry(
    raw_index: dict[str, object],
    cluster_sha: str,
    skill_name: str,
) -> dict[str, str | list[str]] | None:
    lookup_keys: list[str] = []
    if skill_name:
        lookup_keys.append(f"{skill_name}:{cluster_sha}")
    lookup_keys.append(cluster_sha)

    for key_name in lookup_keys:
        candidate = raw_index.get(key_name)
        if isinstance(candidate, dict):
            return candidate

    if not skill_name:
        return None

    for candidate in raw_index.values():
        if not isinstance(candidate, dict):
            continue
        if candidate.get("cluster_sha") != cluster_sha:
            continue
        if candidate.get("skill_name") != skill_name:
            continue
        return candidate

    return None


def _lookup_command(args: argparse.Namespace) -> int:
    index_data = json.loads(args.index_file.read_text(encoding="utf-8"))
    raw_index = index_data.get("index", {})
    if not isinstance(raw_index, dict):
        raise ValueError("index file missing 'index' object")

    entry = _resolve_lookup_entry(raw_index, args.cluster_sha, args.skill_name)

    if not isinstance(entry, dict):
        print(json.dumps({"hit": False, "reason": "cluster_sha not found"}))
        return 0

    expected_cluster_files = _parse_cluster_files_argument(
        cluster_files=args.cluster_file,
        cluster_files_file=args.cluster_files_file,
    )
    expected_cluster_files_hash = _compute_cluster_files_hash(expected_cluster_files)

    checks: list[tuple[str, str]] = [
        ("cluster_sha", args.cluster_sha),
        ("fingerprint_version", args.fingerprint_version),
        ("cluster_files_hash", expected_cluster_files_hash),
        ("model_id", args.model_id),
        ("instruction_bundle_hash", args.instruction_bundle_hash),
        ("prompt_template_hash", args.prompt_template_hash),
        ("scoring_rubric_hash", args.scoring_rubric_hash),
        ("output_schema_version", args.output_schema_version),
    ]

    if args.model_revision:
        checks.append(("model_revision", args.model_revision))

    mismatches: dict[str, dict[str, str]] = {}

    for key_name, expected_value in checks:
        actual_value = entry.get(key_name)
        if actual_value != expected_value:
            mismatches[key_name] = {
                "expected": expected_value,
                "actual": str(actual_value),
            }

    actual_cluster_files = entry.get("cluster_files")
    if actual_cluster_files != expected_cluster_files:
        mismatches["cluster_files"] = {
            "expected": json.dumps(expected_cluster_files),
            "actual": json.dumps(actual_cluster_files),
        }

    if mismatches:
        print(
            json.dumps(
                {
                    "hit": False,
                    "reason": "metadata mismatch",
                    "mismatches": mismatches,
                },
                sort_keys=True,
            )
        )
        return 0

    print(
        json.dumps(
            {
                "hit": True,
                "cluster_sha": args.cluster_sha,
                "file": entry.get("file", ""),
            },
            sort_keys=True,
        )
    )
    return 0


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Cluster review cache helpers")
    subparsers = parser.add_subparsers(dest="command", required=True)

    precheck_parser = subparsers.add_parser(
        "precheck",
        help="run pre-analysis cache gate and build cache index",
    )
    precheck_parser.add_argument("--project-id", required=True)
    precheck_parser.add_argument("--cache-dir", type=Path, required=True)
    precheck_parser.add_argument("--project-retention-days", type=int)
    precheck_parser.add_argument("--central-retention-days", type=int)
    precheck_parser.add_argument("--global-retention-days", type=int, default=180)
    precheck_parser.add_argument("--min-retention-days", type=int, default=30)
    precheck_parser.add_argument("--index-out", type=Path)
    precheck_parser.set_defaults(handler=_precheck_command)

    lookup_parser = subparsers.add_parser(
        "lookup",
        help="evaluate cache hit for a cluster and run context",
    )
    lookup_parser.add_argument("--index-file", type=Path, required=True)
    lookup_parser.add_argument("--cluster-sha", required=True)
    lookup_parser.add_argument("--skill-name", default="")
    lookup_parser.add_argument("--fingerprint-version", required=True)
    lookup_parser.add_argument("--cluster-file", action="append", default=[])
    lookup_parser.add_argument("--cluster-files-file", type=Path)
    lookup_parser.add_argument("--model-id", required=True)
    lookup_parser.add_argument("--model-revision", default="")
    lookup_parser.add_argument("--instruction-bundle-hash", required=True)
    lookup_parser.add_argument("--prompt-template-hash", required=True)
    lookup_parser.add_argument("--scoring-rubric-hash", required=True)
    lookup_parser.add_argument("--output-schema-version", required=True)
    lookup_parser.set_defaults(handler=_lookup_command)

    store_parser = subparsers.add_parser(
        "store",
        help="write or update cache artifact for a cluster",
    )
    store_parser.add_argument("--cache-dir", type=Path, required=True)
    store_parser.add_argument("--skill-name", required=True)
    store_parser.add_argument("--cluster-sha", required=True)
    store_parser.add_argument("--fingerprint-version", required=True)
    store_parser.add_argument("--cluster-file", action="append", default=[])
    store_parser.add_argument("--cluster-files-file", type=Path)
    store_parser.add_argument("--model-id", required=True)
    store_parser.add_argument("--model-revision", default="")
    store_parser.add_argument("--instruction-bundle-hash", required=True)
    store_parser.add_argument("--prompt-template-hash", required=True)
    store_parser.add_argument("--scoring-rubric-hash", required=True)
    store_parser.add_argument("--output-schema-version", required=True)
    store_parser.add_argument("--analysis-file", type=Path, required=True)
    store_parser.add_argument("--reports-dir", type=Path)
    store_parser.add_argument("--artifact-file", action="append", default=[])
    store_parser.set_defaults(handler=_store_command)

    restore_parser = subparsers.add_parser(
        "restore",
        help="restore cached markdown artifacts for a cache-hit cluster",
    )
    restore_parser.add_argument("--cache-dir", type=Path, required=True)
    restore_parser.add_argument("--entry-file", required=True)
    restore_parser.add_argument("--output-dir", type=Path, required=True)
    restore_parser.set_defaults(handler=_restore_command)

    return parser


def main() -> int:
    """Execute review cache command-line operations."""
    parser = _build_parser()
    parsed_args = parser.parse_args()

    try:
        handler = parsed_args.handler
        if not callable(handler):
            raise ValueError("command handler is not callable")

        result = handler(parsed_args)
        if not isinstance(result, int):
            raise ValueError("command handler must return int")
        return result
    except FileNotFoundError as err:
        print(json.dumps({"error": f"file not found: {err}"}), file=sys.stderr)
        return 1
    except ValueError as err:
        print(json.dumps({"error": str(err)}), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
