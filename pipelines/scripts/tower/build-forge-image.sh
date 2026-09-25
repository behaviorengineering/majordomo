#!/bin/bash
# Build (and optionally push) one majordomo forge CLI image from the pinned submodule.
#
# Usage: build-forge-image.sh <registry> <image-name> <tag> <dockerfile-basename>
# Env:   DOCKER_BUILD_TARGET (default public), SKIP_PUSH, REGISTRY_USR, REGISTRY_PSW
# Run from tower repo root; expects .majordomo/ submodule checkout.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_args "$0" 4 $#

registry="$1"
image_name="$2"
tag="$3"
dockerfile="$4"

majordomo_root="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
if [ ! -f "${majordomo_root}/pipelines/scripts/build-copilot-image.sh" ]; then
  log_error "Cannot find build-copilot-image.sh under ${majordomo_root}"
  exit 1
fi

dockerfile_rel="$(
  if [[ "${dockerfile}" == */* ]]; then
    echo "${dockerfile}"
  else
    echo "dockerfiles/${dockerfile}"
  fi
)"

export DOCKER_BUILD_TARGET="${DOCKER_BUILD_TARGET:-public}"
# build-copilot-image.sh resolves dockerfile paths from the current working directory.
cd "${majordomo_root}"
bash "${majordomo_root}/pipelines/scripts/build-copilot-image.sh" \
  "${registry}" \
  "${image_name}" \
  "${tag}" \
  "${dockerfile_rel}"
