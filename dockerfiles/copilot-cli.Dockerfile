# Copilot CLI image — Node + Python + git + @github/copilot
#
# GitHub CI / open internet (public indexes, no registry secrets):
#   docker build --target public -t majordomo-agent -f dockerfiles/copilot-cli.Dockerfile .
#
# Corp (default stage): corporate pull-through base + package registry.
# Build context must be the majordomo root (has pyproject.toml), e.g. `.` or `.majordomo/`.
#   DOCKER_BUILDKIT=1 docker build \
#     --build-arg BASE_IMAGE=<DOCKER_PULL_DOMAIN>/node:20-slim \
#     --build-arg PACKAGE_REGISTRY_HOST=<PACKAGE_REGISTRY_HOST> \
#     --build-arg CORP_CA_CERT_URL=<CORP_CA_CERT_URL> \
#     --build-arg DEBIAN_REPO_PATH=<DEBIAN_REPO_PATH> \
#     --build-arg PIP_INDEX_PATH=<PIP_INDEX_PATH> \
#     --build-arg NPM_VIRTUAL_PATH=<NPM_VIRTUAL_PATH> \
#     --build-arg GH_HOST=<optional-ghe-host> \
#     --secret id=username,env=REGISTRY_USER \
#     --secret id=token,env=REGISTRY_TOKEN \
#     -t majordomo-agent -f dockerfiles/copilot-cli.Dockerfile .

ARG BASE_IMAGE=node:20-slim

# ---------------------------------------------------------------------------
# public — Hub + public apt/PyPI/npm (GitHub Actions, no proxy/corp registry)
# ---------------------------------------------------------------------------
FROM node:20-slim AS public

WORKDIR /workspace

ENV PIP_BREAK_SYSTEM_PACKAGES=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
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
    && sed -i 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
    && locale-gen \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml /tmp/uv-install/pyproject.toml
RUN pip install --no-cache-dir uv \
    && cd /tmp/uv-install \
    && uv pip install --system --no-cache --break-system-packages --group pipeline \
    && rm -rf /tmp/uv-install

ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt

ARG GH_HOST=
ENV GH_HOST=${GH_HOST}

RUN npm install -g @github/copilot

RUN mkdir -p /root/.copilot/agents && \
    printf '{"trusted_folders":["/workspace","/tmp","/root"]}' \
        > /root/.copilot/config.json

CMD ["copilot"]

# ---------------------------------------------------------------------------
# corp — corporate pull-through base + package registry (default final stage)
# ---------------------------------------------------------------------------
FROM ${BASE_IMAGE} AS corp

ARG PACKAGE_REGISTRY_HOST
ARG CORP_CA_CERT_URL
ARG DEBIAN_REPO_PATH
ARG PIP_INDEX_PATH
ARG NPM_VIRTUAL_PATH
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY
ARG GH_HOST=

WORKDIR /workspace

ENV PIP_BREAK_SYSTEM_PACKAGES=1

# Bootstrap CA, then install packages from the corporate Debian mirror.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh \
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
    && sed -i 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
    && locale-gen

COPY pyproject.toml /tmp/uv-install/pyproject.toml

# Install uv + pipeline deps from the corporate PyPI index.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/registry-user.sh && \
    REGISTRY_USER=$(read_registry_user_sanitized /run/secrets/username) && \
    REGISTRY_TOKEN=$(tr -d '\r\n' < /run/secrets/token) && \
    PIP_INDEX_URL="https://${REGISTRY_USER}:${REGISTRY_TOKEN}@${PACKAGE_REGISTRY_HOST}/${PIP_INDEX_PATH}" && \
    pip install --no-cache-dir --index-url "${PIP_INDEX_URL}" --trusted-host "${PACKAGE_REGISTRY_HOST}" uv && \
    cd /tmp/uv-install && \
    uv pip install --system --no-cache --break-system-packages --system-certs \
        --index-url "${PIP_INDEX_URL}" \
        --group pipeline \
    && rm -rf /tmp/uv-install

ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt
ENV GH_HOST=${GH_HOST}

# Optional proxy — only applied when build-args are passed (empty = unset).
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTPS_PROXY}
ENV NO_PROXY=${NO_PROXY}

RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/install-npm-tool.sh @github/copilot

# Unset proxy after install to avoid runtime issues when the agent has no proxy.
ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV NO_PROXY=

RUN mkdir -p /root/.copilot/agents && \
    printf '{"trusted_folders":["/workspace","/tmp","/root"]}' \
        > /root/.copilot/config.json

CMD ["copilot"]
