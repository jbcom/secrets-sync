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
cmd/secrets-sync
cmd/secrets-sync-controller
cmd/secrets-sync-lambda
cmd/secrets-sync/cmd
pkg/circuitbreaker
pkg/client/aws
pkg/client/vault
pkg/context
pkg/diff
pkg/discovery/identitycenter
pkg/driver
pkg/kubernetes
pkg/observability
pkg/pipeline
pkg/utils
python/secrets_sync
```
