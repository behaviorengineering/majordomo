#!/bin/bash
# Shared utilities for Copilot CLI automation scripts.
# Source this file after common.sh:
#   source "${SCRIPT_DIR}/lib/common.sh"
#   source "${SCRIPT_DIR}/lib/copilot-common.sh"

# Populates ~/.copilot/agents/ with symlinks to built-in agents from the submodule,
# then applies per-agent overrides supplied by the caller.
#
# Usage: setup_workspace [overrides]
#   overrides — optional comma-separated list of name=/abs/path pairs,
#               e.g. "pr-review=/workspace/agents/my-pr-review.agent.md"
#               Each entry symlinks <name>.agent.md over the built-in default.
setup_workspace() {
    local overrides="${1:-}"
    local workspace
    workspace="$(pwd)"
    local agents_src="${workspace}/.majordomo/agents"
    local agents_dir="${HOME}/.copilot/agents"

    log_info "Setting up Copilot agents: ${agents_dir}"
    mkdir -p "${agents_dir}"

    # Symlink every built-in agent from the submodule
    for agent_file in "${agents_src}"/*.agent.md; do
        [ -e "${agent_file}" ] || continue
        local name
        name="$(basename "${agent_file}")"
        ln -sfn "${agent_file}" "${agents_dir}/${name}"
        log_info "  Built-in: ${name}"
    done

    # Apply caller-supplied overrides on top (name=/abs/path, comma-separated)
    if [ -n "${overrides}" ]; then
        IFS=',' read -ra pairs <<< "${overrides}"
        for pair in "${pairs[@]}"; do
            local agent_name="${pair%%=*}"
            local agent_path="${pair#*=}"
            ln -sfn "${agent_path}" "${agents_dir}/${agent_name}.agent.md"
            log_info "  Override:  ${agent_name} -> ${agent_path}"
        done
    fi
}
