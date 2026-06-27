# Ownership Map

`secrets-sync` owns the standalone SecretSync runtime and release surface.

## In This Repository

| Surface | Owner |
| --- | --- |
| Go CLI | `cmd/secrets-sync` |
| Go pipeline packages | `pkg/*` |
| gopy binding source | `python/secrets_sync` |
| Python binding distribution | `secrets-sync-python-binding` |
| GitHub Action | `action.yml` |
| GHCR image | `Dockerfile`, `.github/workflows/cd.yml` |
| Helm chart | `deploy/charts/secrets-sync` |
| Kubernetes CRD schema | `deploy/crds` |
| Kubernetes controller | `cmd/secrets-sync-controller`, `pkg/kubernetes`, `deploy/controller` |
| Lambda entrypoint/archive | `cmd/secrets-sync-lambda`, `deploy/lambda` |
| Documentation site | `docs/` |

## Outside This Repository

| Surface | Repository | Install target |
| --- | --- | --- |
| Base data primitives, inputs, logging, and workflows | `jbcom/extended-data` | `extended-data` |
| Vendor provider coordination and SecretSync Python facade | `jbcom/vendor-fabric` | `vendor-fabric[secrets-sync]` |
| Agent/runtime framework tools | `jbcom/agentic-fabric` | `agentic-fabric[...]` |

## Boundary Rule

This repository is canonical for merge, validate, diff, and sync behavior.
Downstream Python should call the `secrets_sync` binding rather than re-creating
pipeline semantics or shelling out through a Python CLI wrapper. Vendor-fabric
may adapt credentials, providers, redaction, and ExtendedData composition, and
may pass upstream-owned provider session material through `ProviderSession`.
Agentic-fabric may adapt those capabilities into agent runtimes.
