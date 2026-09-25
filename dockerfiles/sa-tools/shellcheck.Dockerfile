# SA tool image — shellcheck (shell script analyser)
#
# GitHub CI / open internet:
#   docker build --target public -t sa-shellcheck -f shellcheck.Dockerfile .
#
# Corp (default stage): pass PACKAGE_REGISTRY_* build-args + BuildKit secrets.
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-shellcheck -S warning -f gcc /workspace/path/to/file.sh

ARG BASE_IMAGE=debian:bookworm-slim

FROM debian:bookworm-slim AS public

WORKDIR /workspace

RUN apt-get update \
    && apt-get install -y --no-install-recommends shellcheck \
    && rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["shellcheck"]
CMD ["--help"]

FROM ${BASE_IMAGE} AS corp

ARG PACKAGE_REGISTRY_HOST
ARG CORP_CA_CERT_URL
ARG DEBIAN_REPO_PATH
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

WORKDIR /workspace

RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh shellcheck

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["shellcheck"]
CMD ["--help"]
