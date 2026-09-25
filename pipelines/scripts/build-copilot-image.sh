#!/bin/bash
# Build and push a Docker image (majordomo-agent / forge CLI / SA tool) to the registry.
# Usage: ./build-copilot-image.sh <registry> <image-name> <image-tag> <dockerfile-path>
#
# Environment:
#   REGISTRY_USR / REGISTRY_PSW   - Docker registry login (optional for public/local)
#   REGISTRY_USER / REGISTRY_TOKEN - package-registry credentials (corp builds)
#   DOCKER_BUILD_TARGET           - public | corp (default: corp when PACKAGE_REGISTRY_HOST set)
#   PACKAGE_REGISTRY_HOST, CORP_CA_CERT_URL, DEBIAN_REPO_PATH, PIP_INDEX_PATH,
#   NPM_VIRTUAL_PATH, HADOLINT_IMAGE, BASE_IMAGE, DOCKER_PULL_DOMAIN, GH_HOST
#   HTTP_PROXY / HTTPS_PROXY / NO_PROXY - passed as build-args only when set
#   SKIP_PUSH=true                - build only (GitHub Actions / local smoke)
#
# Context:
#   sa-tools/*          → dockerfiles/sa-tools/
#   Dockerfile.agent|gh|glab → majordomo root (parent of dockerfiles/)
#                         (`.majordomo/` when vendored as a submodule)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

setup_error_trap
require_args "$0" 4 $#

REGISTRY="$1"
IMAGE_NAME="$2"
IMAGE_TAG="$3"
DOCKERFILE_PATH="$4"

FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
DOCKERFILE_DIR="$(cd "$(dirname "${DOCKERFILE_PATH}")" && pwd)"
DOCKERFILE_ABS="${DOCKERFILE_DIR}/$(basename "${DOCKERFILE_PATH}")"
DOCKERFILE_BASE="$(basename "${DOCKERFILE_PATH}")"

export REGISTRY_USER="${REGISTRY_USER:-}"
export REGISTRY_TOKEN="${REGISTRY_TOKEN:-}"

IS_SA_TOOL=false
IS_AGENT_IMAGE=false
IS_CLI_IMAGE=false
if [[ "${DOCKERFILE_PATH}" == *"/sa-tools/"* ]] || [[ "${DOCKERFILE_PATH}" == sa-tools/* ]] \
    || [[ "$(basename "${DOCKERFILE_DIR}")" == "sa-tools" ]]; then
    IS_SA_TOOL=true
elif [[ "${DOCKERFILE_BASE}" == "Dockerfile.cli" ]]; then
    IS_CLI_IMAGE=true
elif [[ "${DOCKERFILE_BASE}" == "Dockerfile.agent" ]] \
    || [[ "${DOCKERFILE_BASE}" == "Dockerfile.gh" ]] \
    || [[ "${DOCKERFILE_BASE}" == "Dockerfile.glab" ]] \
    || [[ "${DOCKERFILE_BASE}" == "copilot-cli.Dockerfile" ]]; then
    IS_AGENT_IMAGE=true
fi

if [ "${IS_SA_TOOL}" = true ]; then
    BUILD_CONTEXT="${DOCKERFILE_DIR}"
    DOCKERFILE_FLAG="${DOCKERFILE_ABS}"
elif [ "${IS_AGENT_IMAGE}" = true ] || [ "${IS_CLI_IMAGE}" = true ]; then
    # majordomo root (native checkout) or .majordomo/ (vendored submodule)
    BUILD_CONTEXT="$(cd "${DOCKERFILE_DIR}/.." && pwd)"
    DOCKERFILE_FLAG="${DOCKERFILE_ABS}"
else
    BUILD_CONTEXT="."
    DOCKERFILE_FLAG="${DOCKERFILE_PATH}"
fi

IS_DUAL_STAGE=false
if [ "${IS_SA_TOOL}" = true ] || [ "${IS_AGENT_IMAGE}" = true ]; then
    IS_DUAL_STAGE=true
fi

if [ "${IS_DUAL_STAGE}" = true ]; then
    if [ -n "${DOCKER_BUILD_TARGET:-}" ]; then
        BUILD_TARGET="${DOCKER_BUILD_TARGET}"
    elif [ -n "${PACKAGE_REGISTRY_HOST:-}" ]; then
        BUILD_TARGET="corp"
    else
        BUILD_TARGET="public"
    fi
else
    BUILD_TARGET=""
fi

log_header "Building Image"
log_info "Image:      ${FULL_IMAGE}"
log_info "Dockerfile: ${DOCKERFILE_PATH}"
log_info "Context:    ${BUILD_CONTEXT}"
log_info "Target:     ${BUILD_TARGET:-default}"

require_command docker

if [ -n "${REGISTRY_USR:-}" ] && [ -n "${REGISTRY_PSW:-}" ] && [ -n "${REGISTRY}" ] && [ "${REGISTRY}" != "local" ]; then
    log_info "Authenticating with registry: [REDACTED credentials]"
    echo "${REGISTRY_PSW}" | docker login "${REGISTRY}" -u "${REGISTRY_USR}" --password-stdin
fi

BUILD_ARGS=()
EXTRA_ARGS=()

if [ -n "${BUILD_TARGET}" ]; then
    EXTRA_ARGS+=(--target "${BUILD_TARGET}")
fi

if [ "${IS_DUAL_STAGE}" = true ] && [ "${BUILD_TARGET}" = "corp" ]; then
    if [ -z "${REGISTRY_USER}" ] || [ -z "${REGISTRY_TOKEN}" ]; then
        log_error "Corp target requires REGISTRY_USER and REGISTRY_TOKEN"
        exit 1
    fi
    if [ -z "${PACKAGE_REGISTRY_HOST:-}" ]; then
        log_error "Corp target requires PACKAGE_REGISTRY_HOST"
        exit 1
    fi

    if [ -n "${BASE_IMAGE:-}" ]; then
        BUILD_ARGS+=(--build-arg "BASE_IMAGE=${BASE_IMAGE}")
    elif [ -n "${DOCKER_PULL_DOMAIN:-}" ]; then
        case "${IMAGE_NAME}" in
            copilot-cli|sa-eslint|majordomo-agent) BASE_FROM="${DOCKER_PULL_DOMAIN}/node:20-slim" ;;
            majordomo-gh|majordomo-glab) BASE_FROM="${DOCKER_PULL_DOMAIN}/debian:bookworm-slim" ;;
            sa-ruff|sa-bandit|sa-mypy) BASE_FROM="${DOCKER_PULL_DOMAIN}/python:3.12-slim" ;;
            sa-shellcheck|sa-hadolint) BASE_FROM="${DOCKER_PULL_DOMAIN}/debian:bookworm-slim" ;;
            *) BASE_FROM="${DOCKER_PULL_DOMAIN}/node:20-slim" ;;
        esac
        BUILD_ARGS+=(--build-arg "BASE_IMAGE=${BASE_FROM}")
    else
        log_error "Corp target requires BASE_IMAGE or DOCKER_PULL_DOMAIN"
        exit 1
    fi

    BUILD_ARGS+=(--build-arg "PACKAGE_REGISTRY_HOST=${PACKAGE_REGISTRY_HOST}")
    [ -n "${CORP_CA_CERT_URL:-}" ] && BUILD_ARGS+=(--build-arg "CORP_CA_CERT_URL=${CORP_CA_CERT_URL}")
    [ -n "${DEBIAN_REPO_PATH:-}" ] && BUILD_ARGS+=(--build-arg "DEBIAN_REPO_PATH=${DEBIAN_REPO_PATH}")
    [ -n "${PIP_INDEX_PATH:-}" ] && BUILD_ARGS+=(--build-arg "PIP_INDEX_PATH=${PIP_INDEX_PATH}")
    [ -n "${NPM_VIRTUAL_PATH:-}" ] && BUILD_ARGS+=(--build-arg "NPM_VIRTUAL_PATH=${NPM_VIRTUAL_PATH}")
    [ -n "${DEBIAN_SUITE:-}" ] && BUILD_ARGS+=(--build-arg "DEBIAN_SUITE=${DEBIAN_SUITE}")
    [ -n "${GH_HOST:-}" ] && BUILD_ARGS+=(--build-arg "GH_HOST=${GH_HOST}")

    if [ "${IMAGE_NAME}" = "sa-hadolint" ]; then
        if [ -n "${HADOLINT_IMAGE:-}" ]; then
            BUILD_ARGS+=(--build-arg "HADOLINT_IMAGE=${HADOLINT_IMAGE}")
        elif [ -n "${DOCKER_PULL_DOMAIN:-}" ]; then
            BUILD_ARGS+=(--build-arg "HADOLINT_IMAGE=${DOCKER_PULL_DOMAIN}/hadolint/hadolint:latest-debian")
        else
            log_error "sa-hadolint corp build requires HADOLINT_IMAGE or DOCKER_PULL_DOMAIN"
            exit 1
        fi
    fi

    EXTRA_ARGS+=(--secret "id=username,env=REGISTRY_USER")
    EXTRA_ARGS+=(--secret "id=token,env=REGISTRY_TOKEN")
elif [ "${IS_DUAL_STAGE}" = true ] && [ "${BUILD_TARGET}" = "public" ]; then
    [ -n "${GH_HOST:-}" ] && BUILD_ARGS+=(--build-arg "GH_HOST=${GH_HOST}")
fi

[ -n "${HTTP_PROXY:-}" ] && BUILD_ARGS+=(--build-arg "HTTP_PROXY=${HTTP_PROXY}")
[ -n "${HTTPS_PROXY:-}" ] && BUILD_ARGS+=(--build-arg "HTTPS_PROXY=${HTTPS_PROXY}")
[ -n "${NO_PROXY:-}" ] && BUILD_ARGS+=(--build-arg "NO_PROXY=${NO_PROXY}")

log_info "Building image..."
DOCKER_BUILDKIT=1 docker build \
    --tag "${FULL_IMAGE}" \
    --file "${DOCKERFILE_FLAG}" \
    "${EXTRA_ARGS[@]}" \
    "${BUILD_ARGS[@]}" \
    --label "build.number=${BUILD_NUMBER:-local}" \
    --label "build.timestamp=$(date -Iseconds)" \
    "${BUILD_CONTEXT}"

if [ -n "${REGISTRY}" ] && [ "${REGISTRY}" != "local" ] && [ "${SKIP_PUSH:-false}" != "true" ]; then
    log_info "Pushing image to registry..."
    docker push "${FULL_IMAGE}"
    log_info "Image available: ${FULL_IMAGE}"
else
    log_info "Skipping push (REGISTRY=${REGISTRY:-empty}, SKIP_PUSH=${SKIP_PUSH:-false})"
    log_info "Image available locally: ${FULL_IMAGE}"
fi
