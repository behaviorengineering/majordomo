# Copilot CLI image — Node + Python + git + @github/copilot
# Triggered by: Changes to this Dockerfile
#
# Base image pulled via package registry pull-through cache to avoid Docker Hub rate limits

ARG BASE_IMAGE=example-docker-snapshot-dependencies.packages.example.com/node:20-slim

FROM ${BASE_IMAGE}

# Build arguments for proxy configuration
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY=localhost,127.0.0.1,packages.example.com

WORKDIR /workspace

# python3-pip is installed via apt (Debian bookworm), which marks the environment as
# "externally managed" per PEP 668. Inside a Docker container that guard is irrelevant
# (the container is ephemeral), so we override it globally rather than repeating the
# --break-system-packages flag on every pip call.
ENV PIP_BREAK_SYSTEM_PACKAGES=1

# Disable default Debian repos, configure package registry mirror, install packages and corporate CA cert — all in one layer.
# Uses BuildKit secrets so credentials are never baked into image layers.
# Acquire::https::Verify-Peer=false required: CA cert isn't trusted yet when pulling from package registry.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/registry-user.sh && \
    REGISTRY_USER_SANITIZED=$(read_registry_user_sanitized /run/secrets/username) && \
    REGISTRY_TOKEN=$(cat /run/secrets/token) && \
    rm -f /etc/apt/sources.list.d/*.list && \
    mkdir -p /etc/apt/auth.conf.d && \
    echo "# Disabled - using package registry mirror" > /etc/apt/sources.list && \
    echo "deb https://packages.example.com/package-registry/debian-repo bookworm main" \
        > /etc/apt/sources.list.d/package-registry.list && \
    printf 'machine packages.example.com\nlogin %s\npassword %s\n' "${REGISTRY_USER_SANITIZED}" "${REGISTRY_TOKEN}" \
        > /etc/apt/auth.conf.d/package-registry.conf && \
    chmod 600 /etc/apt/auth.conf.d/package-registry.conf && \
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
    && curl --insecure -o /usr/local/share/ca-certificates/corp-ca.crt \
       https://packages.example.com/package-registry/example-generic/example/security/certificates/20200302/cacert-20200302.pem \
    && update-ca-certificates \
    && sed -i 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
    && locale-gen \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /etc/apt/sources.list.d/package-registry.list \
    && rm -f /etc/apt/auth.conf.d/package-registry.conf

# Copy project definition so uv can read [dependency-groups.pipeline] from it.
# Build context is the post_creator workspace root; this repo lives at .majordomo/.
COPY .majordomo/pyproject.toml /tmp/uv-install/pyproject.toml

# Install pipeline Python dependencies via uv.
# uv understands PEP 735 dependency groups natively; pip does not.
# pyproject.toml is the single source of truth — no version duplication here.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/registry-user.sh && \
    REGISTRY_USER_SANITIZED=$(read_registry_user_sanitized /run/secrets/username) && \
    REGISTRY_TOKEN=$(cat /run/secrets/token) && \
    printf 'machine packages.example.com\nlogin %s\npassword %s\n' "${REGISTRY_USER_SANITIZED}" "${REGISTRY_TOKEN}" > /root/.netrc && \
    chmod 600 /root/.netrc && \
    PIP_INDEX_URL="https://packages.example.com/package-registry/api/pypi/example-pypi-snapshot-dependencies/simple" && \
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
# Point Copilot CLI at the corporate GitHub Enterprise instance
ENV GH_HOST=https://github.example.com

# Install GitHub Copilot CLI via package registry npm virtual registry
# Uses BuildKit secrets for package registry credentials (never baked into image layers)
ARG NPM_REGISTRY=https://packages.example.com/package-registry/api/npm/example-npm-virtual/
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/registry-user.sh && \
    REGISTRY_USER_SANITIZED=$(read_registry_user_sanitized /run/secrets/username) && \
    REGISTRY_TOKEN=$(cat /run/secrets/token) && \
    npm install -g @github/copilot \
        --registry "${NPM_REGISTRY}" \
        --//packages.example.com/package-registry/api/npm/example-npm-virtual/:username="${REGISTRY_USER_SANITIZED}" \
        --//packages.example.com/package-registry/api/npm/example-npm-virtual/:_password="$(echo -n "${REGISTRY_TOKEN}" | base64)" \
        --//packages.example.com/package-registry/api/npm/example-npm-virtual/:always-auth=true

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
