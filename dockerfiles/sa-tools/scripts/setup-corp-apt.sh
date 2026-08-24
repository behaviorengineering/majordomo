#!/bin/sh
# Corporate apt mirror setup and CA cert installer for SA tool corp stages.
# Called as a BuildKit bind-mounted script from each SA tool Dockerfile corp stage.
# Requires BuildKit secrets: username, token
# Requires env (from Dockerfile ARG): PACKAGE_REGISTRY_HOST, CORP_CA_CERT_URL, DEBIAN_REPO_PATH
# Optional env: DEBIAN_SUITE (default bookworm)
#
# Usage (in Dockerfile, build context = dockerfiles/sa-tools):
#   RUN --mount=type=secret,id=username \
#       --mount=type=secret,id=token \
#       --mount=type=bind,source=scripts,target=/tmp/sa-scripts,ro \
#       /bin/sh /tmp/sa-scripts/setup-corp-apt.sh [extra-apt-package ...]
#
# Tool-image pattern (copilot-league): bootstrap CA via public apt + curl --insecure,
# then switch to the corp mirror for any further packages. After update-ca-certificates,
# later apt MUST verify TLS.
set -e

PACKAGE_REGISTRY_HOST="${PACKAGE_REGISTRY_HOST:?PACKAGE_REGISTRY_HOST is required}"
CORP_CA_CERT_URL="${CORP_CA_CERT_URL:?CORP_CA_CERT_URL is required}"
DEBIAN_REPO_PATH="${DEBIAN_REPO_PATH:?DEBIAN_REPO_PATH is required}"
DEBIAN_SUITE="${DEBIAN_SUITE:-bookworm}"

if [ ! -f /tmp/sa-scripts/registry-user.sh ]; then
    echo "setup-corp-apt.sh: missing /tmp/sa-scripts/registry-user.sh" >&2
    exit 2
fi

. /tmp/sa-scripts/registry-user.sh

REGISTRY_USER=$(read_registry_user_sanitized /run/secrets/username)
REGISTRY_TOKEN=$(tr -d '\r\n' < /run/secrets/token)

# Bootstrap: public apt only long enough to fetch the corporate CA.
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl
curl --insecure -o /usr/local/share/ca-certificates/corp-ca.crt \
    "${CORP_CA_CERT_URL}"
update-ca-certificates
rm -rf /var/lib/apt/lists/*

# Extra packages (if any) from the corporate Debian mirror.
# Credentials live only in this RUN; CA is already in the trust store.
if [ "$#" -gt 0 ]; then
    rm -f /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources
    echo "# Disabled — using corporate apt mirror" > /etc/apt/sources.list
    echo "deb https://${REGISTRY_USER}:${REGISTRY_TOKEN}@${PACKAGE_REGISTRY_HOST}/${DEBIAN_REPO_PATH} ${DEBIAN_SUITE} main" \
        > /etc/apt/sources.list.d/registry.list
    apt-get update
    apt-get install -y --no-install-recommends "$@"
    rm -rf /var/lib/apt/lists/*
    rm -f /etc/apt/sources.list.d/registry.list
fi
