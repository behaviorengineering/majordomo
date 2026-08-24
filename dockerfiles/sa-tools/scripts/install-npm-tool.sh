#!/bin/sh
# Shared npm global installer for Node-based SA image corp stages.
#
# Required BuildKit secrets: username, token
# Required env (from Dockerfile ARG): PACKAGE_REGISTRY_HOST, NPM_VIRTUAL_PATH
#
# Usage:
#   /bin/sh /tmp/sa-scripts/install-npm-tool.sh <package-name>

set -e

TOOL_NAME="$1"
if [ -z "$TOOL_NAME" ]; then
    echo "install-npm-tool.sh: missing package name" >&2
    exit 2
fi

PACKAGE_REGISTRY_HOST="${PACKAGE_REGISTRY_HOST:?PACKAGE_REGISTRY_HOST is required}"
NPM_VIRTUAL_PATH="${NPM_VIRTUAL_PATH:?NPM_VIRTUAL_PATH is required}"

if [ ! -f /tmp/sa-scripts/registry-user.sh ]; then
    echo "install-npm-tool.sh: missing /tmp/sa-scripts/registry-user.sh" >&2
    exit 2
fi

. /tmp/sa-scripts/registry-user.sh

REGISTRY_USER=$(read_registry_user_sanitized /run/secrets/username)
REGISTRY_TOKEN=$(tr -d '\r\n' < /run/secrets/token)
AUTH_PATH="//${PACKAGE_REGISTRY_HOST}/${NPM_VIRTUAL_PATH}"

cleanup() {
    rm -f /root/.npmrc
}
trap cleanup EXIT

printf '%s\n' \
    "registry=https://${PACKAGE_REGISTRY_HOST}/${NPM_VIRTUAL_PATH}" \
    "${AUTH_PATH}:username=${REGISTRY_USER}" \
    "${AUTH_PATH}:_password=$(echo -n "${REGISTRY_TOKEN}" | base64 | tr -d '\n')" \
    "${AUTH_PATH}:always-auth=true" \
    > /root/.npmrc

export NODE_EXTRA_CA_CERTS="${NODE_EXTRA_CA_CERTS:-/etc/ssl/certs/ca-certificates.crt}"
npm install -g "${TOOL_NAME}"
rm -rf /root/.npm
