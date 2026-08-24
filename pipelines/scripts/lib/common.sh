#!/bin/bash
# Shared bash utilities for pipelines/scripts/
# Source this file: source "${SCRIPT_DIR}/lib/common.sh"

log_info() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $*"
}

log_warn() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [WARN] $*" >&2
}

log_error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $*" >&2
}

log_debug() {
    if [ "${DEBUG:-false}" = "true" ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [DEBUG] $*"
    fi
}

log_header() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] ========== $* =========="
}

require_args() {
    local script_name=$1
    local required_count=$2
    local actual_count=$3
    if [ "${actual_count}" -lt "${required_count}" ]; then
        log_error "Usage: ${script_name} requires ${required_count} arguments, got ${actual_count}"
        exit 1
    fi
}

require_command() {
    local cmd=$1
    if ! command -v "${cmd}" &> /dev/null; then
        log_error "Required command not found: ${cmd}"
        exit 1
    fi
}

setup_error_trap() {
    trap 'rc=$?; log_error "Script failed at line ${LINENO} — exit code ${rc}"' ERR
}
