# Ownership Map

`secrets-sync` owns the SecretSync product surface: the Go CLI, GitHub Action,
deployment artifacts, and documentation site.

## In This Repository

| Surface | Current owner |
| --- | --- |
| Go CLI | `cmd/secrets-sync` |
| Go pipeline packages | `pkg/*` |
| GitHub Action | `action.yml` |
| Helm chart | `deploy/charts/secrets-sync` |

## Outside This Repository

| Surface | Current repository | Install target |
| --- | --- | --- |
| Base data primitives, inputs, logging, and workflows | `jbcom/extended-data` | `extended-data` |
| Vendor API connectors and Python-native SecretSync | `jbcom/vendor-fabric` | `vendor-fabric[secrets-sync]` |
| SecretSync agent framework tools | `jbcom/vendor-fabric` | `vendor-fabric[ai,secrets-sync]` |

The Python package and generated binding source were retired from this
repository. Python applications should use `vendor_fabric.secrets_sync`, which
ports the pipeline concepts into Python and composes the Extended Data and
vendor connector layers directly.
