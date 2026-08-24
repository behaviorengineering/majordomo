# SA tool image — mypy (Python type checker)
# Triggered by: Changes to this Dockerfile
#
# Runs as: docker run --rm -v <workspace>:/workspace sa-mypy:<tag> [file...]
#
# Config detection order (evaluated at runtime inside the container):
#   1. /workspace/mypy.ini          — repo-level config (strict, project-specific)
#   2. /workspace/pyproject.toml    — if it contains a [tool.mypy] section
#   3. /defaults/mypy.ini           — baked-in fallback (lenient, ignore_missing_imports)
#
# Source root detection: if /workspace/src exists, MYPYPATH is set to /workspace/src
# automatically so that src-layout packages resolve correctly.
#
# Base image pulled via Artifactory pull-through cache to avoid Docker Hub rate limits

ARG BASE_IMAGE=a01a0f-met-docker-snapshot-dependencies.artifactory.srv.westpac.com.au/python:3.12-slim

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

# Install mypy via Artifactory PyPI mirror.
RUN --mount=type=secret,id=salary_id \
    --mount=type=secret,id=jfrog_token \
    --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/scripts,target=/tmp/sa-scripts,ro \
    /bin/sh /tmp/sa-scripts/install-pypi-tool.sh mypy

ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV PYTHONIOENCODING=utf-8
ENV PYTHONUTF8=1
# Redirect mypy cache to /tmp — /workspace is mounted read-only so mypy cannot
# write to /workspace/.mypy_cache. The container is ephemeral (--rm) so the
# cache provides no benefit; /tmp is always writable.
ENV MYPY_CACHE_DIR=/tmp/.mypy_cache

# Baked-in fallback config — used when the repo has no mypy.ini or pyproject.toml [tool.mypy].
# Lenient by design: ignore_missing_imports avoids noise from unstubbed third-party packages.
# Repos with their own mypy.ini override this entirely.
RUN --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/mypy/mypy-default.ini,target=/tmp/mypy-default.ini,ro /bin/sh -ec 'mkdir -p /defaults; cp /tmp/mypy-default.ini /defaults/mypy.ini'

# Entrypoint: detects repo config, sets MYPYPATH for src-layout, then runs mypy.
# Receives the list of changed files as positional arguments from run-sa-tool.sh.
RUN --mount=type=bind,source=.majordomo/dockerfiles/sa-tools/mypy/mypy-entrypoint.sh,target=/tmp/mypy-entrypoint.sh,ro /bin/sh -ec 'cp /tmp/mypy-entrypoint.sh /usr/local/bin/mypy-entrypoint.sh; chmod +x /usr/local/bin/mypy-entrypoint.sh'

ENTRYPOINT ["/usr/local/bin/mypy-entrypoint.sh"]
CMD ["--help"]