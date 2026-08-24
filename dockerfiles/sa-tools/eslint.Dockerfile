# SA tool image — eslint (JavaScript/TypeScript linter)
# Triggered by: Changes to this Dockerfile
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-eslint:<tag> --format unix /workspace/path/to/file.ts
#
# Note: eslint requires a config file (eslint.config.js / .eslintrc.*) in the project.
# If none is present, eslint exits with a config-not-found error (treated as no findings).
#
# Base image pulled via package registry pull-through cache to avoid Docker Hub rate limits

ARG BASE_IMAGE=example-docker-snapshot-dependencies.packages.example.com/node:20-slim

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

ARG NPM_REGISTRY=https://packages.example.com/package-registry/api/npm/example-npm-virtual/

# Install eslint globally via package registry npm registry
RUN --mount=type=secret,id=username \
    --mount=type=secret,id=token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/registry-user.sh && \
    REGISTRY_USER_SANITIZED=$(read_registry_user_sanitized /run/secrets/username) && \
    REGISTRY_TOKEN=$(cat /run/secrets/token) && \
    export NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt && \
    npm config set cafile /etc/ssl/certs/ca-certificates.crt && \
    npm config set strict-ssl true && \
    npm install -g eslint \
        --registry "${NPM_REGISTRY}" \
        --//packages.example.com/package-registry/api/npm/example-npm-virtual/:username="${REGISTRY_USER_SANITIZED}" \
        --//packages.example.com/package-registry/api/npm/example-npm-virtual/:_password="$(echo -n "${REGISTRY_TOKEN}" | base64)" \
        --//packages.example.com/package-registry/api/npm/example-npm-virtual/:always-auth=true

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["eslint"]
CMD ["--help"]
