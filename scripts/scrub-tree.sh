#!/bin/sh
# Scrub corporate-specific strings from all text files in the working tree.
# Prefer scripts/scrub-history.sh for full history rewrite (inlines this logic).

find . -type f \
  ! -path './.git/*' \
  ! -path './scripts/scrub-*' \
  \( \
    -name '*.md' -o -name '*.sh' -o -name '*.py' -o -name '*.go' -o \
    -name '*.yml' -o -name '*.yaml' -o -name '*.json' -o -name '*.groovy' -o \
    -name 'Jenkinsfile' -o -name '*.Jenkinsfile' -o -name '*Jenkinsfile*' -o \
    -name '*.Dockerfile' -o -name 'Dockerfile' -o -name '*.ini' -o -name '*.toml' -o \
    -name '*.txt' -o -name '*.agent.md' -o -name '*.persona.md' -o -name '*.conf' \
  \) -print0 \
| xargs -0 perl -pi -e '
  s/a01a0f-met-docker-snapshot-dependencies\.artifactory\.srv\.westpac\.com\.au/example-docker-snapshot-dependencies.packages.example.com/g;
  s/a01a0f-met-docker-snapshot-local\.artifactory\.srv\.westpac\.com\.au/example-docker-snapshot-local.packages.example.com/g;
  s/a01a0f-met-pypi-snapshot-dependencies/example-pypi-snapshot-dependencies/g;
  s/wbcorp-pr-proxy0\.westpac\.com\.au/proxy.example.com/g;
  s/wbcorp-pr-proxy0\.example\.com/proxy.example.com/g;
  s/wbcorp-pr-proxy0/proxy/g;
  s|/a01a0f/|/example-project/|g;
  s/a01a0f-met-/example-/g;
  s/a01a0f/example-project/g;
  s/artifactory\.srv\.westpac\.com\.au/packages.example.com/g;
  s/jenkins\.srv\.westpac\.com\.au/jenkins.example.com/g;
  s/bitbucket\.srv\.westpac\.com\.au/bitbucket.example.com/g;
  s/westpac\.ghe\.com/github.example.com/g;
  s/westpacgroup\.com/example.com/g;
  s/\.westpac\.com\.au/.example.com/g;
  s/westpac\.com\.au/example.com/g;
  s/Westpac/corporate/g;
  s/westpac/corporate/g;
  s/L212278/ci-user/g;
  s/l212278/ci-user/g;
  s/met-app-docker-creds/example-docker-creds/g;
  s/met-app-artifactory-token/example-registry-token/g;
  s/edp_obm_lnx_shared/linux-shared-agent/g;
  s/wdp-001_npm_virtual/example-npm-virtual/g;
  s/wdp-001_generic/example-generic/g;
  s/wdp-001/example-org/g;
  s/read_artifactory_user_sanitized/read_registry_user_sanitized/g;
  s/setup-artifactory-apt\.sh/setup-corp-apt.sh/g;
  s/artifactory-user\.sh/registry-user.sh/g;
  s/ARTIFACTORY_USER_SANITIZED/REGISTRY_USER_SANITIZED/g;
  s/ARTIFACTORY_USER/REGISTRY_USER/g;
  s/JFROG_ID_TOKEN/REGISTRY_TOKEN/g;
  s/JFROG_TOKEN/REGISTRY_TOKEN/g;
  s/SALARY_ID/REGISTRY_USER/g;
  s/id=salary_id/id=username/g;
  s/id=jfrog_token/id=token/g;
  s|/run/secrets/salary_id|/run/secrets/username|g;
  s|/run/secrets/jfrog_token|/run/secrets/token|g;
  s/salary_id/username/g;
  s/jfrog_token/token/g;
  s/westpac-ca\.crt/corp-ca.crt/g;
  s/au\/com\/westpac\/security/example\/security/g;
  s/Artifactory/package registry/g;
  s/artifactory/package-registry/g;
'
