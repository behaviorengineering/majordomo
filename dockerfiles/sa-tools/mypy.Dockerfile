# SA tool image — mypy (Python type checker)
#
# GitHub CI / open internet:
#   docker build --target public -t sa-mypy -f mypy.Dockerfile .
#
# Corp (default stage): pass PACKAGE_REGISTRY_* build-args + BuildKit secrets.
#
# Config detection (runtime): mypy.ini → pyproject.toml [tool.mypy] → /defaults/mypy.ini

ARG BASE_IMAGE=python:3.12-slim

FROM python:3.12-slim AS public

WORKDIR /workspace

RUN pip install --no-cache-dir mypy

ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
ENV MYPY_CACHE_DIR=/tmp/.mypy_cache

COPY mypy/mypy-default.ini /defaults/mypy.ini
COPY mypy/mypy-entrypoint.sh /usr/local/bin/mypy-entrypoint.sh
RUN chmod +x /usr/local/bin/mypy-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/mypy-entrypoint.sh"]
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
    /bin/sh /tmp/sa-scripts/install-pypi-tool.sh mypy

ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
ENV MYPY_CACHE_DIR=/tmp/.mypy_cache

COPY mypy/mypy-default.ini /defaults/mypy.ini
COPY mypy/mypy-entrypoint.sh /usr/local/bin/mypy-entrypoint.sh
RUN chmod +x /usr/local/bin/mypy-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/mypy-entrypoint.sh"]
CMD ["--help"]
