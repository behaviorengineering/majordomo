#!/bin/bash
# Compatibility shim — Copilot CLI dispatch was replaced by OpenCode (agent-dispatch.sh).
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/agent-dispatch.sh" "$@"
