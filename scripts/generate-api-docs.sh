#!/usr/bin/env bash
# Generate Go API reference markdown via gomarkdoc into docs/api for the
# repo-root Sphinx site.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$REPO_ROOT/docs/api"

if ! command -v gomarkdoc >/dev/null 2>&1; then
  if [ -x "$HOME/go/bin/gomarkdoc" ]; then
    export PATH="$HOME/go/bin:$PATH"
  else
    echo "gomarkdoc not found. Installing with:" >&2
    echo "  go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest" >&2
    go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest
    export PATH="$HOME/go/bin:$PATH"
  fi
fi

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

cd "$REPO_ROOT"
package_dirs="$(go list -f '{{.Dir}}' ./cmd/... ./pkg/... ./python/...)"
while IFS= read -r pkg_dir; do
  [[ -z "$pkg_dir" ]] && continue
  rel_pkg="${pkg_dir#$REPO_ROOT/}"
  pkg="./$rel_pkg"
  if ! gomarkdoc \
    --output "$OUT_DIR/{{.Dir}}.md" \
    --repository.url "https://github.com/jbcom/secrets-sync" \
    --repository.default-branch main \
    --repository.path / \
    "$pkg" 2>&1 | sed "s#^#  $pkg: #"; then
    echo "gomarkdoc failed for $pkg" >&2
    exit 1
  fi
done <<< "$package_dirs"

find "$OUT_DIR" -name "*.md" | while read -r file; do
  pkg_name=$(awk '/^# /{print $2; exit}' "$file")
  rel_path="${file#$OUT_DIR/}"
  go_path="${rel_path%.md}"

  if ! head -1 "$file" | grep -q '^---$'; then
    tmp="$(mktemp)"
    {
      echo "---"
      echo "title: ${go_path}"
      echo "description: Go API reference for the ${pkg_name:-package} package."
      echo "---"
      echo ""
      cat "$file"
    } > "$tmp"
    mv "$tmp" "$file"
  fi
done

toc_entries=$(
  find "$OUT_DIR" -name "*.md" ! -name "index.md" -print \
    | sed "s#^$OUT_DIR/##" \
    | sed 's#\.md$##' \
    | sort
)

cat > "$OUT_DIR/index.md" <<'MDEOF'
---
title: Go API reference
description: Auto-generated Go package and binding documentation.
---

This section is **generated** from Go doc comments via
[gomarkdoc](https://github.com/princjef/gomarkdoc). Do not edit files under
`docs/api/` directly. Changes are overwritten on the next docs build.

To improve this reference, edit the doc comments in the corresponding `.go`
file and regenerate with `just docs-api` or `tox -e docs` from the repo root.

## Organization

- **cmd/secrets-sync/** - CLI entry points and subcommand handlers.
- **pkg/** - pipeline, diffing, observability, AWS/Vault clients, and helpers.
- **python/secrets_sync/** - gopy binding source for
  `secrets-sync-python-binding`.

```{toctree}
:hidden:
MDEOF

while IFS= read -r entry; do
  [[ -z "$entry" ]] && continue
  printf '%s\n' "$entry" >> "$OUT_DIR/index.md"
done <<< "$toc_entries"

cat >> "$OUT_DIR/index.md" <<'MDEOF'
```
MDEOF

count=$(find "$OUT_DIR" -name "*.md" | wc -l | tr -d ' ')
echo "Generated ${count} API reference page(s) in ${OUT_DIR#$REPO_ROOT/}"
