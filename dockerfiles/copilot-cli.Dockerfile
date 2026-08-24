# Copilot CLI image — Node + Python + git + @github/copilot
# Triggered by: Changes to this Dockerfile
#
# Base image pulled via Artifactory pull-through cache to avoid Docker Hub rate limits

ARG BASE_IMAGE=a01a0f-met-docker-snapshot-dependencies.artifactory.srv.westpac.com.au/node:20-slim

FROM ${BASE_IMAGE}

# Build arguments for proxy configuration
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY=localhost,127.0.0.1,artifactory.srv.westpac.com.au

WORKDIR /workspace

# python3-pip is installed via apt (Debian bookworm), which marks the environment as
# "externally managed" per PEP 668. Inside a Docker container that guard is irrelevant
# (the container is ephemeral), so we override it globally rather than repeating the
# --break-system-packages flag on every pip call.
ENV PIP_BREAK_SYSTEM_PACKAGES=1

# Disable default Debian repos, configure Artifactory mirror, install packages and Westpac CA cert — all in one layer.
# Uses BuildKit secrets so credentials are never baked into image layers.
# Acquire::https::Verify-Peer=false required: CA cert isn't trusted yet when pulling from Artifactory.
RUN --mount=type=secret,id=salary_id \
    --mount=type=secret,id=jfrog_token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/artifactory-user.sh && \
    ARTIFACTORY_USER_SANITIZED=$(read_artifactory_user_sanitized /run/secrets/salary_id) && \
    JFROG_ID_TOKEN=$(cat /run/secrets/jfrog_token) && \
    rm -f /etc/apt/sources.list.d/*.list && \
    mkdir -p /etc/apt/auth.conf.d && \
    echo "# Disabled - using Artifactory mirror" > /etc/apt/sources.list && \
    echo "deb https://artifactory.srv.westpac.com.au/artifactory/debian-repo bookworm main" \
        > /etc/apt/sources.list.d/artifactory.list && \
    printf 'machine artifactory.srv.westpac.com.au\nlogin %s\npassword %s\n' "${ARTIFACTORY_USER_SANITIZED}" "${JFROG_ID_TOKEN}" \
        > /etc/apt/auth.conf.d/artifactory.conf && \
    chmod 600 /etc/apt/auth.conf.d/artifactory.conf && \
    apt-get -o "Acquire::https::Verify-Peer=false" update && \
    apt-get -o "Acquire::https::Verify-Peer=false" install -y --no-install-recommends \
        ca-certificates \
        curl \
        locales \
        python3 \
        python3-pip \
        git \
        vim \
        jq \
        ripgrep \
        fd-find \
        less \
        procps \
    && curl --insecure -o /usr/local/share/ca-certificates/westpac-ca.crt \
       https://artifactory.srv.westpac.com.au/artifactory/wdp-001_generic/au/com/westpac/security/certificates/20200302/cacert-20200302.pem \
    && update-ca-certificates \
    && sed -i 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
    && locale-gen \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /etc/apt/sources.list.d/artifactory.list \
    && rm -f /etc/apt/auth.conf.d/artifactory.conf

# Copy project definition so uv can read [dependency-groups.pipeline] from it.
# Build context is the post_creator workspace root; this repo lives at .majordomo/.
COPY .majordomo/pyproject.toml /tmp/uv-install/pyproject.toml

# Install pipeline Python dependencies via uv.
# uv understands PEP 735 dependency groups natively; pip does not.
# pyproject.toml is the single source of truth — no version duplication here.
RUN --mount=type=secret,id=salary_id \
    --mount=type=secret,id=jfrog_token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/artifactory-user.sh && \
    ARTIFACTORY_USER_SANITIZED=$(read_artifactory_user_sanitized /run/secrets/salary_id) && \
    JFROG_ID_TOKEN=$(cat /run/secrets/jfrog_token) && \
    printf 'machine artifactory.srv.westpac.com.au\nlogin %s\npassword %s\n' "${ARTIFACTORY_USER_SANITIZED}" "${JFROG_ID_TOKEN}" > /root/.netrc && \
    chmod 600 /root/.netrc && \
    PIP_INDEX_URL="https://artifactory.srv.westpac.com.au/artifactory/api/pypi/a01a0f-met-pypi-snapshot-dependencies/simple" && \
    pip install --no-cache-dir --index-url "${PIP_INDEX_URL}" uv && \
    cd /tmp/uv-install && \
    uv pip install --system --no-cache --break-system-packages --system-certs \
        --index-url "${PIP_INDEX_URL}" \
        --group pipeline \
    && rm -f /root/.netrc \
    && rm -rf /tmp/uv-install

# Locale / encoding — en_US.UTF-8 is a fully-generated POSIX locale ensuring Node.js 20
# uses UTF-8 for process streams. C.UTF-8 is glibc-built-in but Node's stream encoding
# detection can fall back to ASCII/Latin-1 on slim images without a full POSIX locale,
# producing Windows-1252 mojibake (â€" instead of —) in AI-generated markdown output.
# PYTHONIOENCODING / PYTHONUTF8 ensure Python 3 text-mode I/O never falls back to ASCII.
ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1

# Configure SSL and proxy for network operations
ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTPS_PROXY}
ENV NO_PROXY=${NO_PROXY}
# Point Copilot CLI at the Westpac GitHub Enterprise instance
ENV GH_HOST=https://westpac.ghe.com

# Install GitHub Copilot CLI via Artifactory npm virtual registry
# Uses BuildKit secrets for Artifactory credentials (never baked into image layers)
ARG NPM_REGISTRY=https://artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/
RUN --mount=type=secret,id=salary_id \
    --mount=type=secret,id=jfrog_token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/artifactory-user.sh && \
    ARTIFACTORY_USER_SANITIZED=$(read_artifactory_user_sanitized /run/secrets/salary_id) && \
    JFROG_ID_TOKEN=$(cat /run/secrets/jfrog_token) && \
    npm install -g @github/copilot \
        --registry "${NPM_REGISTRY}" \
        --//artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/:username="${ARTIFACTORY_USER_SANITIZED}" \
        --//artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/:_password="$(echo -n "${JFROG_ID_TOKEN}" | base64)" \
        --//artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/:always-auth=true

# Unset proxy after install to avoid runtime issues
ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV NO_PROXY=

# Pre-trust workspace and temp directories so copilot CLI never prompts for folder trust.
# /workspace is the source code volume mount; /tmp is used for intermediate files.
# HOME is /root in this container (set via docker run -e HOME=/root).
# agents/ is pre-created so setup_workspace() can populate it with symlinks at runtime.
RUN mkdir -p /root/.copilot/agents && \
    printf '{"trusted_folders":["/workspace","/tmp","/root"]}' \
        > /root/.copilot/config.json

# Default command when run standalone; Jenkins Docker agent overrides this automatically.
CMD ["copilot"]
