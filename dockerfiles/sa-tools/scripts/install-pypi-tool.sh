#!/bin/sh
# Shared PyPI tool installer for Python-based SA images.
#
# Why this is a shell script:
# - It runs inside multiple container images during Docker build.
# - Shell is the common runtime available in those build environments.
#
# Required BuildKit secrets:
# - username
# - token
#
# Usage:
#   /bin/sh /tmp/install-pypi-tool.sh <package-name>

set -e

TOOL_NAME="$1"
if [ -z "$TOOL_NAME" ]; then
    echo "install-pypi-tool.sh: missing package name" >&2
    exit 2
fi

if [ ! -f /tmp/sa-scripts/registry-user.sh ]; then
    echo "install-pypi-tool.sh: missing /tmp/sa-scripts/registry-user.sh" >&2
    exit 2
fi

. /tmp/sa-scripts/registry-user.sh

REGISTRY_USER_SANITIZED=$(read_registry_user_sanitized /run/secrets/username)
REGISTRY_TOKEN=$(tr -d '\r\n' < /run/secrets/token)

cleanup() {
    rm -f /root/.netrc
}
trap cleanup EXIT

cat > /root/.netrc <<EOF
machine packages.example.com
login ${REGISTRY_USER_SANITIZED}
password ${REGISTRY_TOKEN}
EOF

chmod 600 /root/.netrc

pip install --no-cache-dir \
    --index-url "https://packages.example.com/package-registry/api/pypi/example-pypi-snapshot-dependencies/simple" \
    "$TOOL_NAME"
