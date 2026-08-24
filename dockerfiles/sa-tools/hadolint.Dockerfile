# SA tool image — hadolint (Dockerfile linter)
# Triggered by: Changes to this Dockerfile
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-hadolint:<tag> --no-color /workspace/path/to/Dockerfile
#
# Base image pulled via package registry pull-through cache to avoid Docker Hub rate limits
# hadolint is a statically linked Haskell binary — copy from its official image into debian-slim.

ARG BASE_IMAGE=example-docker-snapshot-dependencies.packages.example.com/debian:bookworm-slim
ARG HADOLINT_IMAGE=example-docker-snapshot-dependencies.packages.example.com/hadolint/hadolint:latest-debian

FROM ${HADOLINT_IMAGE} AS hadolint-bin

FROM ${BASE_IMAGE}

# Build arguments for proxy configuration
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY=localhost,127.0.0.1,packages.example.com

WORKDIR /workspace

# Copy hadolint binary from official image
COPY --from=hadolint-bin /bin/hadolint /usr/local/bin/hadolint

# Configure package registry apt mirror, install CA cert — shared script keeps this DRY across all SA tool images.
# Uses BuildKit secrets so credentials are never baked into image layers.
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-corp-apt.sh

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["hadolint"]
CMD ["--help"]
