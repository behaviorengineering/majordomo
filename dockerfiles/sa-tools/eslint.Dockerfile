# SA tool image — eslint (JavaScript/TypeScript linter)
#
# GitHub CI / open internet:
#   docker build --target public -t sa-eslint -f eslint.Dockerfile .
#
# Corp (default stage): pass PACKAGE_REGISTRY_* build-args + BuildKit secrets.
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-eslint --format unix /workspace/path/to/file.ts

ARG BASE_IMAGE=node:20-slim

FROM node:20-slim AS public

WORKDIR /workspace

RUN npm install -g eslint

ENTRYPOINT ["eslint"]
CMD ["--help"]

FROM ${BASE_IMAGE} AS corp

ARG PACKAGE_REGISTRY_HOST
ARG CORP_CA_CERT_URL
ARG DEBIAN_REPO_PATH
ARG NPM_VIRTUAL_PATH
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

WORKDIR /workspace

RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt

RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/install-npm-tool.sh eslint

ENTRYPOINT ["eslint"]
CMD ["--help"]
