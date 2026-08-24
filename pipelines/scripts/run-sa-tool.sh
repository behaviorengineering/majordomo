#!/bin/bash
# Run a single SA tool container against a list of changed files and write
# human-readable output to .sa/<tool-slug>.txt in the workspace.
#
# Usage:
#   run-sa-tool.sh <tool-slug> <image> <command> <workspace> <file>...
#
# Arguments:
#   tool-slug   Short identifier for the tool (used as output filename, e.g. ruff)
#   image       Full Docker image reference to run (e.g. my-registry/sa-ruff:abc123)
#   command     Command string passed to the container entrypoint (e.g. "check --output-format=concise")
#   workspace   Absolute path to the checked-out repository root (mounted as /workspace)
#   file...     One or more repo-relative file paths to analyse
#
# Output:
#   <workspace>/.sa/<tool-slug>.txt   — tool output (may be empty if no findings)
#
# Exit codes:
#   0  Tool ran successfully (findings or no findings)
#   1  Setup or argument error
#
# SA tool exit codes (non-zero = findings found) are treated as success — a linter
# reporting findings is not a pipeline failure. Tool execution errors (missing image,
# docker daemon failure) are logged as warnings and do not fail the pipeline.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

if [ $# -lt 5 ]; then
    log_error "Usage: $0 <tool-slug> <image> <command> <workspace> <file>..."
    exit 1
fi

TOOL_SLUG="$1"
IMAGE="$2"
COMMAND="$3"
WORKSPACE="$4"
shift 4
FILES=("$@")

SA_DIR="${WORKSPACE}/.sa"
OUTPUT_FILE="${SA_DIR}/${TOOL_SLUG}.txt"

mkdir -p "${SA_DIR}"

log_header "SA: ${TOOL_SLUG}"
log_info "  Image:     ${IMAGE}"
log_info "  Command:   ${COMMAND}"
log_info "  Files:     ${#FILES[@]}"

require_command docker

# Convert repo-relative paths to /workspace-relative paths for the container
CONTAINER_FILES=()
for f in "${FILES[@]}"; do
    CONTAINER_FILES+=("/workspace/${f}")
done

# Run the SA tool. Non-zero exit from the linter (findings present) is not an error.
# Capture combined stdout+stderr — most tools write findings to stdout.
# ${COMMAND} is intentionally unquoted to allow word-splitting of multi-flag command strings.
set +e
# shellcheck disable=SC2086
docker run --rm \
    -v "${WORKSPACE}:/workspace:ro" \
    "${IMAGE}" \
    ${COMMAND} "${CONTAINER_FILES[@]}" \
    > "${OUTPUT_FILE}" 2>&1
TOOL_RC=$?
set -e

FINDING_COUNT=$(wc -l < "${OUTPUT_FILE}" | tr -d ' ')

if [ "${TOOL_RC}" -eq 0 ] && [ "${FINDING_COUNT}" -eq 0 ]; then
    log_info "  Result:    no findings"
elif [ "${TOOL_RC}" -le 1 ]; then
    # Exit code 0 = clean, 1 = findings found — both are normal linter behaviour
    log_info "  Result:    ${FINDING_COUNT} line(s) of output written to .sa/${TOOL_SLUG}.txt"
else
    # Exit code ≥2 means tool invocation error (bad args, missing config, crashed, etc.)
    # Log the output and fail — a broken SA tool must not silently pass the pipeline.
    log_error "  Tool exited with code ${TOOL_RC} — invocation error; output:"
    cat "${OUTPUT_FILE}" >&2
    exit 1
fi

log_info "Done: ${TOOL_SLUG}"
