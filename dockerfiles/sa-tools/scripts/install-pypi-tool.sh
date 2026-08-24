#!/bin/sh
# Shared PyPI tool installer for Python-based SA images.
#
# Why this is a shell script:
# - It runs inside multiple container images during Docker build.
# - Shell is the common runtime available in those build environments.
#
# Required BuildKit secrets:
# - salary_id
# - jfrog_token
#
# Usage:
#   /bin/sh /tmp/install-pypi-tool.sh <package-name>

set -e

TOOL_NAME="$1"
if [ -z "$TOOL_NAME" ]; then
    echo "install-pypi-tool.sh: missing package name" >&2
    exit 2
fi

if [ ! -f /tmp/sa-scripts/artifactory-user.sh ]; then
    echo "install-pypi-tool.sh: missing /tmp/sa-scripts/artifactory-user.sh" >&2
    exit 2
fi

. /tmp/sa-scripts/artifactory-user.sh

ARTIFACTORY_USER_SANITIZED=$(read_artifactory_user_sanitized /run/secrets/salary_id)
JFROG_ID_TOKEN=$(tr -d '\r\n' < /run/secrets/jfrog_token)

cleanup() {
    rm -f /root/.netrc
}
trap cleanup EXIT

cat > /root/.netrc <<EOF
machine artifactory.srv.westpac.com.au
login ${ARTIFACTORY_USER_SANITIZED}
password ${JFROG_ID_TOKEN}
EOF

chmod 600 /root/.netrc

pip install --no-cache-dir \
    --index-url "https://artifactory.srv.westpac.com.au/artifactory/api/pypi/a01a0f-met-pypi-snapshot-dependencies/simple" \
    "$TOOL_NAME"
