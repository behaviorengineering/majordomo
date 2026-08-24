# SA tool image — shellcheck (shell script analyser)
# Triggered by: Changes to this Dockerfile
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-shellcheck:<tag> -S warning -f gcc /workspace/path/to/file.sh
#
# Base image pulled via package registry pull-through cache to avoid Docker Hub rate limits

ARG BASE_IMAGE=example-docker-snapshot-dependencies.packages.example.com/debian:bookworm-slim

FROM ${BASE_IMAGE}

# Build arguments for proxy configuration
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY=localhost,127.0.0.1,packages.example.com

WORKDIR /workspace

# Configure package registry apt mirror, install CA cert and shellcheck — shared script keeps this DRY across all SA tool images.
# Extra apt packages passed as positional args (shellcheck here).
# Uses BuildKit secrets so credentials are never baked into image layers.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh shellcheck

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["shellcheck"]
CMD ["--help"]
