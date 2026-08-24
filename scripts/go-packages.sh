#!/usr/bin/env bash
# List source Go packages while excluding generated build output.

set -euo pipefail

mapfile -t package_dirs < <(
  find . -type f -name '*.go' \
    ! -path './.git/*' \
    ! -path './.tools/*' \
    ! -path './bin/*' \
    ! -path './dist/*' \
    ! -path '*/node_modules/*' \
    ! -path './docs/dist/*' \
    ! -path './python/build/*' \
    -exec dirname {} \; \
    | sort -u
)

if [[ "${#package_dirs[@]}" -eq 0 ]]; then
  exit 0
fi

GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.6}" go list "${package_dirs[@]}"
