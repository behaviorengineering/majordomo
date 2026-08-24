#!/bin/sh
# Shared PyPI tool installer for Python-based SA image corp stages.
#
# Required BuildKit secrets: username, token
# Required env (from Dockerfile ARG): PACKAGE_REGISTRY_HOST, PIP_INDEX_PATH
#
# Usage:
#   /bin/sh /tmp/sa-scripts/install-pypi-tool.sh <package-name>

set -e

TOOL_NAME="$1"
if [ -z "$TOOL_NAME" ]; then
    echo "install-pypi-tool.sh: missing package name" >&2
    exit 2
fi

PACKAGE_REGISTRY_HOST="${PACKAGE_REGISTRY_HOST:?PACKAGE_REGISTRY_HOST is required}"
PIP_INDEX_PATH="${PIP_INDEX_PATH:?PIP_INDEX_PATH is required}"

if [ ! -f /tmp/sa-scripts/registry-user.sh ]; then
    echo "install-pypi-tool.sh: missing /tmp/sa-scripts/registry-user.sh" >&2
    exit 2
fi

. /tmp/sa-scripts/registry-user.sh

REGISTRY_USER=$(read_registry_user_sanitized /run/secrets/username)
REGISTRY_TOKEN=$(tr -d '\r\n' < /run/secrets/token)

pip install --no-cache-dir \
    --index-url "https://${REGISTRY_USER}:${REGISTRY_TOKEN}@${PACKAGE_REGISTRY_HOST}/${PIP_INDEX_PATH}" \
    --trusted-host "${PACKAGE_REGISTRY_HOST}" \
    "$TOOL_NAME"
