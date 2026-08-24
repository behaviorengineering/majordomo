#!/bin/sh
# Shared Artifactory username sanitizer.
# Input may be either bare login (l212278) or email (l212278@westpac.com.au).
# Output is always the login/local-part.

read_artifactory_user_sanitized() {
    secret_path="$1"
    raw_value=$(tr -d '\r\n' < "$secret_path")
    printf '%s' "${raw_value%%@*}"
}
