# SA tool image — ruff (Python linter/formatter)
# Triggered by: Changes to this Dockerfile
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-ruff:<tag> check --output-format=concise /workspace
#
# Base image pulled via package registry pull-through cache to avoid Docker Hub rate limits

ARG BASE_IMAGE=example-docker-snapshot-dependencies.packages.example.com/python:3.12-slim

FROM ${BASE_IMAGE}

# Build arguments for proxy configuration
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY=localhost,127.0.0.1,packages.example.com

WORKDIR /workspace

# Configure package registry apt mirror, install CA cert — shared script keeps this DRY across all SA tool images.
# Uses BuildKit secrets so credentials are never baked into image layers.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh

# Install ruff via package registry PyPI mirror.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/install-pypi-tool.sh ruff

ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
# Redirect ruff cache to /tmp — /workspace is mounted read-only so ruff cannot
# write to /workspace/.ruff_cache. The container is ephemeral (--rm) so the
# cache provides no benefit; /tmp is always writable.
ENV RUFF_CACHE_DIR=/tmp/.ruff_cache

ENTRYPOINT ["ruff"]
CMD ["--help"]
