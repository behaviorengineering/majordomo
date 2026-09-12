#!/usr/bin/env bash
# Wire Cursor discovery to strop's in-module ai-copilots (see strop BOOTSTRAP.md).
# Re-run after bumping github.com/behaviorengineering/strop.
set -euo pipefail
cd "$(dirname "$0")/.."
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/strop)"
test -d "$MOD/ai-copilots/skills" || {
	echo "missing ai-copilots under $MOD" >&2
	exit 1
}
mkdir -p .cursor/skills
for name in strop-pipeline-pattern strop-orchestration strop-human-review; do
	ln -snf "$MOD/ai-copilots/skills/${name}" ".cursor/skills/${name}"
	test -f ".cursor/skills/${name}/SKILL.md"
done
echo "wired strop ai-copilots from $MOD"
