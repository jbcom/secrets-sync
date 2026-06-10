# Architecture Audit

This file records the current architecture status of the standalone
`jbcom/secrets-sync` repository. It replaces the earlier migration-era gap
analysis that referenced old monorepo paths such as `stores/vault/vault.go`.

## Current Shape

SecretSync is a standalone Go module with:

- CLI entry point in `cmd/secretsync`.
- Pipeline orchestration in `pkg/pipeline`.
- Vault and AWS clients in `pkg/client`.
- Diffing and exit-code behavior in `pkg/diff`.
- Circuit breaker, request context, and observability support in `pkg`.
- Docker action metadata in `action.yml`.
- Optional Python binding sources under `python/secretssync`.
- Kubernetes API types under `api/v1alpha1`.

The main runtime path is the two-phase pipeline:

1. Merge source secrets into a deterministic merge-store bundle.
2. Sync the bundle into destination stores, primarily AWS Secrets Manager.

## Implemented Parity Items

| Area | Current status | Primary implementation |
| --- | --- | --- |
| Deep merge semantics | Implemented: maps merge recursively, lists append, scalar and type conflicts override. | `pkg/utils/deepmerge.go`, `pkg/pipeline/merge.go`, `pkg/client/vault/vault.go` |
| Vault KV2 traversal | Implemented with breadth-first recursive listing, path validation, depth limits, secret-count limits, and queue compaction. | `pkg/client/vault/vault.go` |
| AWS planned-deletion filtering | Implemented with `IncludePlannedDeletion: aws.Bool(false)`. | `pkg/client/aws/aws.go` |
| Empty AWS secret filtering | Implemented through `NoEmptySecrets`. | `pkg/client/aws/aws.go` |
| Path conflict handling | Implemented for `/foo` versus `foo` before writes. | `pkg/client/aws/aws.go` |
| JSON-aware idempotency | Implemented through `SkipUnchanged` and JSON-normalized comparison. | `pkg/client/aws/aws.go`, `pkg/utils/deepmerge.go` |
| Target inheritance | Implemented with cycle detection and merge-store source paths. | `pkg/pipeline/inheritance.go`, `pkg/pipeline/graph.go` |
| Diff exit codes | Implemented and tested: no changes, changes, and error states map to stable exit codes. | `pkg/diff`, `pkg/pipeline/diff_integration_test.go` |
| Stable pipeline result output | Implemented for machine-readable action and CLI use. | `cmd/secretsync/cmd`, `pkg/pipeline` |

## Release And Action Status

- CI and release workflows are SHA-pinned to current stable action releases.
- Release-please owns the `secrets-sync-vX.Y.Z` component tag shape.
- GoReleaser builds binary release artifacts from release-created tags.
- The Docker action image tag remains `jbcom/secretssync:v1` until digest
  refresh can be automated.

## Known Remaining Work

- The Marketplace and action docs should continue to use the component release
  tag placeholder until the first standalone repository release exists.
- The Docker action should eventually move to a digest-pinned image reference
  once release automation can update that digest as part of publication.
- Optional Python binding and Kubernetes API surfaces need separate release
  contracts if they become first-class artifacts.

## Development Rule

Prefer visible breakage over compatibility shims. This repository is a clean
standalone line, so stale monorepo assumptions should be removed or made to fail
in tests rather than silently accepted.
