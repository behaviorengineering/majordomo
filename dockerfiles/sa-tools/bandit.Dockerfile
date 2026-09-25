# SA tool image — bandit (Python security analyser)
#
# GitHub CI / open internet:
#   docker build --target public -t sa-bandit -f bandit.Dockerfile .
#
# Corp (default stage): pass PACKAGE_REGISTRY_* build-args + BuildKit secrets.
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-bandit -f txt -ll /workspace/path/to/file.py

ARG BASE_IMAGE=python:3.12-slim

FROM python:3.12-slim AS public

WORKDIR /workspace

RUN pip install --no-cache-dir bandit

ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1

ENTRYPOINT ["bandit"]
CMD ["--help"]

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
    /bin/sh /tmp/sa-scripts/install-pypi-tool.sh bandit

ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1

ENTRYPOINT ["bandit"]
CMD ["--help"]
