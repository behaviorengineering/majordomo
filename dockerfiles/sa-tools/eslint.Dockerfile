# SA tool image — eslint (JavaScript/TypeScript linter)
# Triggered by: Changes to this Dockerfile
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-eslint:<tag> --format unix /workspace/path/to/file.ts
#
# Note: eslint requires a config file (eslint.config.js / .eslintrc.*) in the project.
# If none is present, eslint exits with a config-not-found error (treated as no findings).
#
# Base image pulled via Artifactory pull-through cache to avoid Docker Hub rate limits

ARG BASE_IMAGE=a01a0f-met-docker-snapshot-dependencies.artifactory.srv.westpac.com.au/node:20-slim

FROM ${BASE_IMAGE}

# Build arguments for proxy configuration
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY=localhost,127.0.0.1,artifactory.srv.westpac.com.au

WORKDIR /workspace

# Configure Artifactory apt mirror, install CA cert — shared script keeps this DRY across all SA tool images.
# Uses BuildKit secrets so credentials are never baked into image layers.
RUN --mount=type=secret,id=salary_id \
    --mount=type=secret,id=jfrog_token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/setup-artifactory-apt.sh

ARG NPM_REGISTRY=https://artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/

# Install eslint globally via Artifactory npm registry
RUN --mount=type=secret,id=salary_id \
    --mount=type=secret,id=jfrog_token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    . /tmp/sa-scripts/artifactory-user.sh && \
    ARTIFACTORY_USER_SANITIZED=$(read_artifactory_user_sanitized /run/secrets/salary_id) && \
    JFROG_ID_TOKEN=$(cat /run/secrets/jfrog_token) && \
    export NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt && \
    npm config set cafile /etc/ssl/certs/ca-certificates.crt && \
    npm config set strict-ssl true && \
    npm install -g eslint \
        --registry "${NPM_REGISTRY}" \
        --//artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/:username="${ARTIFACTORY_USER_SANITIZED}" \
        --//artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/:_password="$(echo -n "${JFROG_ID_TOKEN}" | base64)" \
        --//artifactory.srv.westpac.com.au/artifactory/api/npm/wdp-001_npm_virtual/:always-auth=true

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["eslint"]
CMD ["--help"]
