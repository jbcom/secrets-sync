#!/usr/bin/env bash
# Install gopy using a module graph that is compatible with current Go.

set -euo pipefail

GOPY_VERSION="${GOPY_VERSION:-v0.4.10}"
X_TOOLS_VERSION="${X_TOOLS_VERSION:-v0.47.0}"
GOBIN="${GOBIN:-$(go env GOPATH)/bin}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

cd "$tmpdir"
go mod init secrets-sync-gopy-bootstrap >/dev/null
go get \
  "github.com/go-python/gopy@${GOPY_VERSION}" \
  "golang.org/x/tools@${X_TOOLS_VERSION}" \
  "golang.org/x/tools/cmd/goimports@${X_TOOLS_VERSION}" >/dev/null

gopy_src="$(go list -m -f '{{.Dir}}' github.com/go-python/gopy)"
cp -R "$gopy_src" "$tmpdir/gopy-src"
chmod -R u+w "$tmpdir/gopy-src"

# Upstream gopy v0.4.10 hard-codes -Ofast for the CPython extension build.
# Modern Clang deprecates -Ofast and can fail cgo's runtime/cgo build when
# warnings are treated as errors. LLVM documents -O3 -ffast-math as the
# replacement, so patch the generated tool during bootstrap.
old='cflags = append(cflags, "-fPIC", "-Ofast")'
new='cflags = append(cflags, "-fPIC", "-O3", "-ffast-math")'
path="gopy-src/cmd_build.go"
if grep -Fq "$new" "$path"; then
  echo "gopy CFLAGS are already patched"
elif ! grep -Fq "$old" "$path"; then
  echo "expected gopy CFLAGS line not found in $path" >&2
  exit 1
else
  perl -0pi -e 's/\Q'"$old"'\E/'"$new"'/g' "$path"
fi

(
  cd "$tmpdir/gopy-src"
  go get "golang.org/x/tools@${X_TOOLS_VERSION}" "golang.org/x/tools/cmd/goimports@${X_TOOLS_VERSION}" >/dev/null
  GOBIN="$GOBIN" go install .
)
GOBIN="$GOBIN" go install golang.org/x/tools/cmd/goimports

echo "Installed patched gopy ${GOPY_VERSION} with golang.org/x/tools ${X_TOOLS_VERSION} into ${GOBIN}"
