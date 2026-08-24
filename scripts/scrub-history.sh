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

echo "Rewriting git history (this may take a minute)..."
git filter-branch -f --tree-filter 'sh scripts/scrub-tree.sh' -- --all

echo "Expiring reflog and garbage-collecting..."
git for-each-ref --format='delete %(refname)' refs/original | git update-ref --stdin
git reflog expire --expire=now --all
git gc --prune=now --aggressive

echo "Done. Verify with: git log -p -S westpac -- . | head"
