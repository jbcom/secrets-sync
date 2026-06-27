#!/usr/bin/env bash
# Resolve the public Python binding distribution version.

set -euo pipefail

binding_version="${PYTHON_BINDING_VERSION:-${VERSION:-}}"
if [[ -z "${binding_version}" ]]; then
  if binding_version="$(git describe --tags --exact-match 2>/dev/null)"; then
    :
  else
    binding_version="0.0.0.dev0"
  fi
fi

binding_version="${binding_version#secrets-sync-v}"
binding_version="${binding_version#v}"
printf '%s\n' "${binding_version}"
