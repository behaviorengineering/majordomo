#!/bin/bash
# Verify gh/glab is on PATH for the forge SCM. Control towers run this inside
# majordomo-gh / majordomo-glab job containers.
#
# Usage: verify-forge-cli.sh
# Env:   SCM (github|gitlab|bitbucket|…)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

scm="${SCM:-${1:-}}"
scm="$(echo "${scm}" | tr '[:upper:]' '[:lower:]')"

case "${scm}" in
  github)
    command -v gh >/dev/null 2>&1 || {
      log_error "gh missing in forge job container (set MAJORDOMO_GH_IMAGE and run majordomo-forge-images)"
      exit 1
    }
    gh --version
    ;;
  gitlab)
    command -v glab >/dev/null 2>&1 || {
      log_error "glab missing in forge job container (set MAJORDOMO_GLAB_IMAGE and run majordomo-forge-images)"
      exit 1
    }
    glab --version
    ;;
  *)
    log_info "SCM=${scm}: no forge CLI required"
    ;;
esac
