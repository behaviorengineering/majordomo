# SA tool image — hadolint (Dockerfile linter)
# Triggered by: Changes to this Dockerfile
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-hadolint:<tag> --no-color /workspace/path/to/Dockerfile
#
# Base image pulled via Artifactory pull-through cache to avoid Docker Hub rate limits
# hadolint is a statically linked Haskell binary — copy from its official image into debian-slim.

ARG BASE_IMAGE=a01a0f-met-docker-snapshot-dependencies.artifactory.srv.westpac.com.au/debian:bookworm-slim
ARG HADOLINT_IMAGE=a01a0f-met-docker-snapshot-dependencies.artifactory.srv.westpac.com.au/hadolint/hadolint:latest-debian

FROM ${HADOLINT_IMAGE} AS hadolint-bin

FROM ${BASE_IMAGE}

# Build arguments for proxy configuration
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY=localhost,127.0.0.1,artifactory.srv.westpac.com.au

WORKDIR /workspace

# Copy hadolint binary from official image
COPY --from=hadolint-bin /bin/hadolint /usr/local/bin/hadolint

# Configure Artifactory apt mirror, install CA cert — shared script keeps this DRY across all SA tool images.
# Uses BuildKit secrets so credentials are never baked into image layers.
RUN --mount=type=secret,id=salary_id \
    --mount=type=secret,id=jfrog_token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-artifactory-apt.sh

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["hadolint"]
CMD ["--help"]
