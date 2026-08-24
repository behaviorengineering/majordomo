#!/bin/bash
# Build and push the Copilot CLI Docker image to the registry
# Usage: ./build-copilot-image.sh <registry> <image-name> <image-tag> <dockerfile-path>
#
# Environment (injected via withCredentials):
#   REGISTRY_USR  - Docker registry username
#   REGISTRY_PSW  - Docker registry password

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
DOCKERFILE_DIR="$(dirname "${DOCKERFILE_PATH}")"

log_header "Building Copilot CLI Image"
log_info "Image:      ${FULL_IMAGE}"
log_info "Dockerfile: ${DOCKERFILE_PATH}"

require_command docker

# Login to registry using stdin (password never exposed as a shell argument)
if [ -n "${REGISTRY_USR:-}" ] && [ -n "${REGISTRY_PSW:-}" ]; then
    log_info "Authenticating with registry: [REDACTED credentials]"
    echo "${REGISTRY_PSW}" | docker login "${REGISTRY}" -u "${REGISTRY_USR}" --password-stdin
fi

# Build — using BuildKit secrets so Artifactory credentials are never baked into image layers
# Credentials are injected via Jenkins withCredentials (SALARY_ID, JFROG_TOKEN env vars)
log_info "Building image..."
DOCKER_BUILDKIT=1 docker build \
    --tag "${FULL_IMAGE}" \
    --file "${DOCKERFILE_PATH}" \
    --secret id=salary_id,env=SALARY_ID \
    --secret id=jfrog_token,env=JFROG_TOKEN \
    ${HTTP_PROXY:+--build-arg HTTP_PROXY="${HTTP_PROXY}"} \
    ${HTTPS_PROXY:+--build-arg HTTPS_PROXY="${HTTPS_PROXY}"} \
    ${NO_PROXY:+--build-arg NO_PROXY="${NO_PROXY}"} \
    --label "build.number=${BUILD_NUMBER:-local}" \
    --label "build.timestamp=$(date -Iseconds)" \
    .

# Push
log_info "Pushing image to registry..."
docker push "${FULL_IMAGE}"

log_info "Image available: ${FULL_IMAGE}"
