#!/bin/sh
# Shared package registry apt mirror setup and corporate CA cert installer.
# Called as a BuildKit bind-mounted script from each SA tool Dockerfile.
# Requires BuildKit secrets: username, token
#
# Usage (in Dockerfile):
#   RUN --mount=type=secret,id=username \
#       --mount=type=secret,id=token \
#       --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
#       /bin/sh /tmp/sa-scripts/setup-corp-apt.sh [extra-apt-package ...]
#
# Any positional arguments are passed to apt-get install as extra packages.
# Example: /bin/sh /tmp/sa-scripts/setup-corp-apt.sh shellcheck
set -e

if [ ! -f /tmp/sa-scripts/registry-user.sh ]; then
    echo "setup-corp-apt.sh: missing /tmp/sa-scripts/registry-user.sh" >&2
    exit 2
fi

. /tmp/sa-scripts/registry-user.sh

REGISTRY_USER_SANITIZED=$(read_registry_user_sanitized /run/secrets/username)
REGISTRY_TOKEN=$(tr -d '\r\n' < /run/secrets/token)

rm -f /etc/apt/sources.list.d/*.list
mkdir -p /etc/apt/auth.conf.d
echo "# Disabled - using package registry mirror" > /etc/apt/sources.list
echo "deb https://packages.example.com/package-registry/debian-repo bookworm main" \
    > /etc/apt/sources.list.d/package-registry.list
cat > /etc/apt/auth.conf.d/package-registry.conf <<EOF
machine packages.example.com
login ${REGISTRY_USER_SANITIZED}
password ${REGISTRY_TOKEN}
EOF
chmod 600 /etc/apt/auth.conf.d/package-registry.conf

apt-get -o "Acquire::https::Verify-Peer=false" update
apt-get -o "Acquire::https::Verify-Peer=false" install -y --no-install-recommends \
    ca-certificates \
    curl \
    "$@"

curl --insecure -o /usr/local/share/ca-certificates/corp-ca.crt \
    https://packages.example.com/package-registry/example-generic/example/security/certificates/20200302/cacert-20200302.pem

update-ca-certificates
rm -rf /var/lib/apt/lists/*
rm -f /etc/apt/sources.list.d/package-registry.list
rm -f /etc/apt/auth.conf.d/package-registry.conf
