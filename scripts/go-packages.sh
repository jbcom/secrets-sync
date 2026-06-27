#!/usr/bin/env bash
# List checked-in Go packages and exclude generated gopy build output.

set -euo pipefail

GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go list ./... \
  | grep -v '^github.com/jbcom/secrets-sync/python/build/'
