# SA tool image — hadolint (Dockerfile linter)
#
# GitHub CI / open internet:
#   docker build --target public -t sa-hadolint -f hadolint.Dockerfile .
#
# Corp (default stage): pass BASE_IMAGE, HADOLINT_IMAGE, PACKAGE_REGISTRY_* + secrets.
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-hadolint --no-color /workspace/path/to/Dockerfile

ARG BASE_IMAGE=debian:bookworm-slim
ARG HADOLINT_IMAGE=hadolint/hadolint:latest-debian

FROM hadolint/hadolint:latest-debian AS hadolint-bin-public

FROM debian:bookworm-slim AS public

WORKDIR /workspace

COPY --from=hadolint-bin-public /bin/hadolint /usr/local/bin/hadolint

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["hadolint"]
CMD ["--help"]

FROM ${HADOLINT_IMAGE} AS hadolint-bin-corp

FROM ${BASE_IMAGE} AS corp

ARG PACKAGE_REGISTRY_HOST
ARG CORP_CA_CERT_URL
ARG DEBIAN_REPO_PATH
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

WORKDIR /workspace

COPY --from=hadolint-bin-corp /bin/hadolint /usr/local/bin/hadolint

RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["hadolint"]
CMD ["--help"]
