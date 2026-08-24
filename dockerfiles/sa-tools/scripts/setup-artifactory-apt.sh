#!/bin/sh
# Shared Artifactory apt mirror setup and Westpac CA cert installer.
# Called as a BuildKit bind-mounted script from each SA tool Dockerfile.
# Requires BuildKit secrets: salary_id, jfrog_token
#
# Usage (in Dockerfile):
#   RUN --mount=type=secret,id=salary_id \
#       --mount=type=secret,id=jfrog_token \
#       --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
#       /bin/sh /tmp/sa-scripts/setup-artifactory-apt.sh [extra-apt-package ...]
#
# Any positional arguments are passed to apt-get install as extra packages.
# Example: /bin/sh /tmp/sa-scripts/setup-artifactory-apt.sh shellcheck
set -e

if [ ! -f /tmp/sa-scripts/artifactory-user.sh ]; then
    echo "setup-artifactory-apt.sh: missing /tmp/sa-scripts/artifactory-user.sh" >&2
    exit 2
fi

. /tmp/sa-scripts/artifactory-user.sh

ARTIFACTORY_USER_SANITIZED=$(read_artifactory_user_sanitized /run/secrets/salary_id)
JFROG_ID_TOKEN=$(tr -d '\r\n' < /run/secrets/jfrog_token)

rm -f /etc/apt/sources.list.d/*.list
mkdir -p /etc/apt/auth.conf.d
echo "# Disabled - using Artifactory mirror" > /etc/apt/sources.list
echo "deb https://artifactory.srv.westpac.com.au/artifactory/debian-repo bookworm main" \
    > /etc/apt/sources.list.d/artifactory.list
cat > /etc/apt/auth.conf.d/artifactory.conf <<EOF
machine artifactory.srv.westpac.com.au
login ${ARTIFACTORY_USER_SANITIZED}
password ${JFROG_ID_TOKEN}
EOF
chmod 600 /etc/apt/auth.conf.d/artifactory.conf

apt-get -o "Acquire::https::Verify-Peer=false" update
apt-get -o "Acquire::https::Verify-Peer=false" install -y --no-install-recommends \
    ca-certificates \
    curl \
    "$@"

curl --insecure -o /usr/local/share/ca-certificates/westpac-ca.crt \
    https://artifactory.srv.westpac.com.au/artifactory/wdp-001_generic/au/com/westpac/security/certificates/20200302/cacert-20200302.pem

update-ca-certificates
rm -rf /var/lib/apt/lists/*
rm -f /etc/apt/sources.list.d/artifactory.list
rm -f /etc/apt/auth.conf.d/artifactory.conf
