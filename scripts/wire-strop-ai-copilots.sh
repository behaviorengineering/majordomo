#!/usr/bin/env bash
# Wire Cursor discovery to strop's in-module ai-copilots (see strop BOOTSTRAP.md).
# Re-run after bumping github.com/behaviorengineering/strop.
# Optional: STROP_MOD=/path/to/strop checkout overrides go list (and ../strop sibling when present).
set -euo pipefail
cd "$(dirname "$0")/.."

resolve_strop_mod() {
	if [[ -n "${STROP_MOD:-}" ]] && [[ -d "${STROP_MOD}/ai-copilots/skills" ]]; then
		cd "$STROP_MOD" && pwd
		return 0
	fi
	local repo_root
	repo_root="$(pwd)"
	if [[ -d "${repo_root}/../strop/ai-copilots/skills" ]]; then
		cd "${repo_root}/../strop" && pwd
		return 0
	fi
	go list -m -f '{{.Dir}}' github.com/behaviorengineering/strop
}

MOD="$(resolve_strop_mod)"
test -d "$MOD/ai-copilots/skills" || {
	echo "missing ai-copilots under $MOD" >&2
	exit 1
}
mkdir -p .cursor/skills
for name in strop-pipeline-pattern strop-orchestration strop-human-review inference-pace; do
	ln -snf "$MOD/ai-copilots/skills/${name}" ".cursor/skills/${name}"
	test -f ".cursor/skills/${name}/SKILL.md"
done
echo "wired strop ai-copilots from $MOD"
