#!/bin/bash
set -euo pipefail

# --- Config detection ---
CONFIG_FILE=""
if [ -f /workspace/mypy.ini ]; then
    CONFIG_FILE=/workspace/mypy.ini
elif grep -qE '^\[tool\.mypy\]' /workspace/pyproject.toml 2>/dev/null; then
    CONFIG_FILE=/workspace/pyproject.toml
else
    CONFIG_FILE=/defaults/mypy.ini
fi

# --- Source root detection (src layout) ---
if [ -d /workspace/src ]; then
    export MYPYPATH=/workspace/src
fi

# Explicit package bases prevent mypy from collapsing sibling package trees
# (for example clients/loaniq and services/loaniq) into duplicate top-level modules.
exec mypy --config-file "${CONFIG_FILE}" --show-column-numbers --explicit-package-bases "$@"
