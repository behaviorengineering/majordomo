#!/bin/sh
# Shared package-registry username sanitizer.
# Input may be either bare login (ci-user) or email (ci-user@example.com).
# Output is always the login/local-part.

read_registry_user_sanitized() {
    secret_path="$1"
    raw_value=$(tr -d '\r\n' < "$secret_path")
    printf '%s' "${raw_value%%@*}"
}
