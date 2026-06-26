# Ownership Map

`secrets-sync` owns the SecretSync product surface: the Go CLI, GitHub Action,
deployment artifacts, documentation site, and the Python bridge package that
invokes the CLI or native Go bindings.

## In This Repository

| Surface | Current owner |
| --- | --- |
| Go CLI | `cmd/secrets-sync` |
| Go pipeline packages | `pkg/*` |
| GitHub Action | `action.yml` |
| Helm chart | `deploy/charts/secrets-sync` |
| Python bridge distribution | `packages/secrets-sync-bridge` |
| Python import package | `secrets_sync` |
| Native gopy output package | `secrets_sync_native` |
| Python binding source | `python/secrets_sync` |

## Outside This Repository

| Surface | Current repository | Install target |
| --- | --- | --- |
| Base data primitives, inputs, logging, and workflows | `jbcom/extended-data` | `extended-data` |
| Vendor API connectors | `jbcom/vendor-connectors` | `vendor-connectors[...]` |
| SecretSync agent framework tools | `jbcom/agentic-crew` | `agentic-crew[secrets-sync]` |

The bridge package has no optional feature extras. It provides backend
selection for the CLI and native runtime only; framework wrappers belong in the
agentic layer.
