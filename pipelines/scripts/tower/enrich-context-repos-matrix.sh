#!/bin/bash
# Add forge_image to each repo row and emit a GHA matrix JSON object.
#
# Usage: enrich-context-repos-matrix.sh <context-repos.json>
# Env:   MAJORDOMO_GH_IMAGE, MAJORDOMO_GLAB_IMAGE
# Prints: {"include":[...]} to stdout

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_args "$0" 1 $#

infile="$1"
if [ ! -f "${infile}" ]; then
  log_error "input not found: ${infile}"
  exit 1
fi

enriched="$(mktemp)"
trap 'rm -f "${enriched}"' EXIT

jq -c --arg gh "${MAJORDOMO_GH_IMAGE:-}" --arg glab "${MAJORDOMO_GLAB_IMAGE:-}" '
  .repos |= map(
    . + {
      forge_image: (
        if .scm == "gitlab" then $glab
        elif .scm == "github" then $gh
        else ""
        end
      )
    }
  )
' "${infile}" > "${enriched}"

missing="$(jq -r '
  [.repos[] | select(.scm == "github" or .scm == "gitlab") | select(.forge_image == "")] | length
' "${enriched}")"

if [ "${missing}" != "0" ]; then
  log_error "Missing MAJORDOMO_GH_IMAGE / MAJORDOMO_GLAB_IMAGE for github|gitlab digest targets"
  log_error "Publish forge images, then set tower repository variables"
  exit 1
fi

jq -c '{include: .repos}' "${enriched}"
