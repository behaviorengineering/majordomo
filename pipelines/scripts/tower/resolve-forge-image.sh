#!/bin/bash
# Resolve the forge job container image for an SCM.
#
# Usage: resolve-forge-image.sh <scm>
# Env:   MAJORDOMO_GH_IMAGE, MAJORDOMO_GLAB_IMAGE
# Prints the image reference to stdout. Exits 1 when github/gitlab and unset.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

scm="$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')"
img=""

case "${scm}" in
  gitlab) img="${MAJORDOMO_GLAB_IMAGE:-}" ;;
  github) img="${MAJORDOMO_GH_IMAGE:-}" ;;
  *) img="" ;;
esac

if [ "${scm}" = "github" ] || [ "${scm}" = "gitlab" ]; then
  if [ -z "${img}" ]; then
    log_error "Missing forge image for scm=${scm} (set MAJORDOMO_GH_IMAGE / MAJORDOMO_GLAB_IMAGE)"
    exit 1
  fi
fi

printf '%s' "${img}"
