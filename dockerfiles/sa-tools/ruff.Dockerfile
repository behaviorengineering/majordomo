# SA tool image — ruff (Python linter/formatter)
#
# GitHub CI / open internet (public indexes, no registry secrets):
#   docker build --target public -t sa-ruff dockerfiles/sa-tools -f dockerfiles/sa-tools/ruff.Dockerfile
#
# Corp (default stage): CA + pip from the corporate package registry.
#   DOCKER_BUILDKIT=1 docker build \
#     --build-arg BASE_IMAGE=<DOCKER_PULL_DOMAIN>/python:3.12-slim \
#     --build-arg PACKAGE_REGISTRY_HOST=<PACKAGE_REGISTRY_HOST> \
#     --build-arg CORP_CA_CERT_URL=<CORP_CA_CERT_URL> \
#     --build-arg DEBIAN_REPO_PATH=<DEBIAN_REPO_PATH> \
#     --build-arg PIP_INDEX_PATH=<PIP_INDEX_PATH> \
#     --secret id=username,env=REGISTRY_USER \
#     --secret id=token,env=REGISTRY_TOKEN \
#     -t sa-ruff -f ruff.Dockerfile .
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-ruff check --output-format=concise /workspace

# ---------------------------------------------------------------------------
# public — Hub + PyPI (GitHub Actions, no proxy/corp registry)
# ---------------------------------------------------------------------------
ARG BASE_IMAGE=python:3.12-slim

FROM python:3.12-slim AS public

WORKDIR /workspace

RUN pip install --no-cache-dir ruff

ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
ENV RUFF_CACHE_DIR=/tmp/.ruff_cache

ENTRYPOINT ["ruff"]
CMD ["--help"]

# ---------------------------------------------------------------------------
# corp — corporate pull-through base + package registry (default final stage)
# ---------------------------------------------------------------------------
FROM ${BASE_IMAGE} AS corp

ARG PACKAGE_REGISTRY_HOST
ARG CORP_CA_CERT_URL
ARG DEBIAN_REPO_PATH
ARG PIP_INDEX_PATH
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

WORKDIR /workspace

RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh

RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/install-pypi-tool.sh ruff

ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
ENV RUFF_CACHE_DIR=/tmp/.ruff_cache

ENTRYPOINT ["ruff"]
CMD ["--help"]
