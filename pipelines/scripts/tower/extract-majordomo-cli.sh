#!/bin/bash
# Pull a majordomo CLI image and copy /majordomo into the workspace.
#
# Usage: extract-majordomo-cli.sh <image-ref> [dest-path]
# Default dest: ./majordomo (tower workspace root).
# Run on the Actions host (not inside a forge job container).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_args "$0" 1 $#
require_command docker

image_ref="$1"
dest="${2:-./majordomo}"

if [ -z "${image_ref}" ]; then
  log_error "majordomo image ref is empty (set MAJORDOMO_IMAGE / vars.MAJORDOMO_IMAGE)"
  exit 1
fi

dest_dir="$(dirname "${dest}")"
mkdir -p "${dest_dir}"

log_info "Resolving ${image_ref}"
if ! docker image inspect "${image_ref}" >/dev/null 2>&1; then
  log_info "Pulling ${image_ref}"
  docker pull "${image_ref}"
fi

cid="$(docker create "${image_ref}")"
cleanup() {
  docker rm -f "${cid}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker cp "${cid}:/majordomo" "${dest}"
chmod +x "${dest}"
if [ ! -s "${dest}" ]; then
  log_error "Extracted majordomo CLI is empty: ${dest}"
  exit 1
fi
log_info "Extracted majordomo CLI to ${dest} ($(wc -c < "${dest}" | tr -d ' ') bytes)"
