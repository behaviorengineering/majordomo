#!/bin/bash
# Clone a served repo with resolved credentials and scrub auth from origin URL.
#
# Usage: clone-served-repo.sh <dest-dir>
# Env:   REPO_ID, SCM, OWNER, CLONE_URL
#        GH_TOKEN_*, GITLAB_TOKEN_*, MAJORDOMO_CREDENTIAL_*, BITBUCKET_TOKEN
# Optional: CLONE_DEPTH (default 500)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_args "$0" 1 $#

dest="$1"
depth="${CLONE_DEPTH:-500}"

repo_key=$(echo "${REPO_ID}" | tr '[:lower:]' '[:upper:]' | sed 's/[^A-Z0-9]/_/g')
owner_key=$(echo "${OWNER}" | tr '[:lower:]' '[:upper:]' | sed 's/[^A-Z0-9]/_/g')
per_repo_var="MAJORDOMO_CREDENTIAL_${repo_key}"
token="${!per_repo_var:-}"

if [ -z "${token}" ]; then
  case "${SCM}" in
    gitlab)
      org_var="GITLAB_TOKEN_${owner_key}"
      token="${!org_var:-}"
      ;;
    bitbucket)
      token="${BITBUCKET_TOKEN:-}"
      ;;
    *)
      org_var="GH_TOKEN_${owner_key}"
      token="${!org_var:-}"
      ;;
  esac
fi

if [ -z "${token}" ]; then
  log_error "No credential for ${REPO_ID} (set MAJORDOMO_CREDENTIAL_${repo_key} or org token for owner ${OWNER})"
  exit 1
fi

log_info "Resolved credential for ${REPO_ID} (owner=${OWNER})"

case "${SCM}" in
  gitlab)
    auth_url=$(echo "${CLONE_URL}" | sed "s#https://#https://oauth2:${token}@#")
    ;;
  bitbucket)
    auth_url=$(echo "${CLONE_URL}" | sed "s#https://#https://x-token-auth:${token}@#")
    ;;
  *)
    auth_url=$(echo "${CLONE_URL}" | sed "s#https://#https://x-access-token:${token}@#")
    ;;
esac

mkdir -p "$(dirname "${dest}")"
git clone --depth "${depth}" "${auth_url}" "${dest}"
git -C "${dest}" fetch --depth "${depth}" origin
git -C "${dest}" remote set-url origin "${CLONE_URL}"
