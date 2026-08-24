#!/bin/sh
# Rewrite git history to remove corporate-specific strings from all commits.
# Requires: git, perl, xargs. Run from repo root.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Error: commit or stash working tree changes before scrubbing history." >&2
  exit 1
fi

export FILTER_BRANCH_SQUELCH_WARNING=1

SCRUB='
find . -type f \
  ! -path "./.git/*" \
  \( \
    -name "*.md" -o -name "*.sh" -o -name "*.py" -o -name "*.go" -o \
    -name "*.yml" -o -name "*.yaml" -o -name "*.json" -o -name "*.groovy" -o \
    -name "Jenkinsfile" -o -name "*.Dockerfile" -o -name "*.ini" -o -name "*.toml" -o \
    -name "*.txt" -o -name "*.agent.md" -o -name "*.persona.md" \
  \) -print0 \
| xargs -0 perl -pi -e "
  s/a01a0f-met-docker-snapshot-dependencies\.artifactory\.srv\.westpac\.com\.au/example-docker-snapshot-dependencies.packages.example.com/g;
  s/a01a0f-met-docker-snapshot-local\.artifactory\.srv\.westpac\.com\.au/example-docker-snapshot-local.packages.example.com/g;
  s/example-pypi-snapshot-dependencies/example-pypi-snapshot-dependencies/g;
  s/artifactory\.srv\.westpac\.com\.au/packages.example.com/g;
  s/jenkins\.srv\.westpac\.com\.au/jenkins.example.com/g;
  s/bitbucket\.srv\.westpac\.com\.au/bitbucket.example.com/g;
  s/westpac\.ghe\.com/github.example.com/g;
  s/westpac\.com\.au/example.com/g;
  s/ci-user\@example\.com/ci-user\@example.com/g;
  s/ci-user/ci-user/g;
  s/example-docker-creds/example-docker-creds/g;
  s/example-registry-token/example-registry-token/g;
  s/linux-shared-agent/linux-shared-agent/g;
  s/example-npm-virtual/example-npm-virtual/g;
  s/example-generic/example-generic/g;
  s/example-org/example-org/g;
  s/read_registry_user_sanitized/read_registry_user_sanitized/g;
  s/setup-artifactory-apt\.sh/setup-corp-apt.sh/g;
  s/artifactory-user\.sh/registry-user.sh/g;
  s/REGISTRY_USER_SANITIZED/REGISTRY_USER_SANITIZED/g;
  s/REGISTRY_TOKEN/REGISTRY_TOKEN/g;
  s/REGISTRY_TOKEN/REGISTRY_TOKEN/g;
  s/REGISTRY_USER/REGISTRY_USER/g;
  s/westpac-ca\.crt/corp-ca.crt/g;
  s/au\/com\/westpac\/security/example\/security/g;
" 2>/dev/null || true
'

echo "Rewriting git history (this may take a minute)..."
git filter-branch -f --tree-filter "$SCRUB" -- --all

echo "Expiring reflog and garbage-collecting..."
git for-each-ref --format='delete %(refname)' refs/original 2>/dev/null | git update-ref --stdin || true
git reflog expire --expire=now --all
git gc --prune=now --aggressive

echo "Done. Verify with: git log -p -S westpac -- . | head"
