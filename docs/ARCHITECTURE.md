# Architecture

SecretSync is a compiled Go pipeline runtime. The universal cross-language
surface is the `secrets-sync` CLI with structured JSON output. GitHub Actions,
the GHCR image, Helm CronJob runner, Kubernetes controller, Lambda entrypoint,
and Python gopy binding all consume the same Go pipeline packages.

See [Architecture Audit](./ARCHITECTURE_AUDIT.md) for the current
implementation-status checklist and release-contract notes.

## Runtime Shape

```text
pipeline.yaml
    |
    v
secrets-sync pipeline --config pipeline.yaml
    |
    +--> merge phase: source secrets -> merge store
    |
    +--> sync phase: merged/source secrets -> target stores
    |
    +--> result envelope: success, counts, per-target results, optional diff
```

The pipeline reads one YAML configuration file, resolves source and target
inheritance, optionally writes a merge store, then syncs destination stores. The
same command can run a dry-run with diff output, a merge-only operation, a
sync-only operation, or the full merge-plus-sync pipeline.

## Core Components

- **CLI entrypoint**: `cmd/secrets-sync` exposes `validate`, `pipeline`, and
  graph-related commands for local, CI, and scheduled execution.
- **Pipeline package**: `pkg/pipeline` owns config loading, validation,
  inheritance resolution, discovery, merge, sync, diff integration, and result
  envelopes.
- **Diff package**: `pkg/diff` builds masked human, JSON, GitHub Actions,
  compact, and side-by-side diff output.
- **Observability package**: `pkg/observability` exposes metrics for pipeline
  runs that opt into the metrics endpoint.
- **GitHub Action**: `action.yml` packages the CLI contract for CI/CD workflows.
- **GHCR image**: `Dockerfile` publishes a distroless image with the CLI and
  Kubernetes controller binaries as `ghcr.io/jbcom/secrets-sync`.
- **Helm chart**: the chart renders a Kubernetes `CronJob` plus ConfigMap or
  existing config mount for scheduled pipeline execution, and can optionally
  install the controller.
- **Kubernetes CRD and controller**: `deploy/crds` defines the
  `CredentialSynchronization` object schema and `cmd/secrets-sync-controller`
  reconciles those resources into managed CronJobs.
- **Lambda entrypoint**: `cmd/secrets-sync-lambda` runs the pipeline from
  inline, S3-hosted, or packaged config and returns structured JSON.
- **Python binding**: `python/secrets_sync` owns the gopy binding source for
  the Go runtime and publishes as `secrets-sync-python-binding`.

## Deployment Models

### Local Or CI Execution

Run the CLI directly when an operator or engineer controls the execution
environment:

```bash
secrets-sync validate --config pipeline.yaml
secrets-sync pipeline --config pipeline.yaml --dry-run --diff --output json
secrets-sync pipeline --config pipeline.yaml --output json
```

GitHub Actions uses the same contract through the published action. The action
does not own a separate API surface; it validates inputs, executes the pipeline,
and reports outputs suitable for CI workflows.

### Kubernetes Scheduled Execution

For Kubernetes, run SecretSync as a direct `CronJob` or install the
`CredentialSynchronization` controller. Both paths mount the pipeline
configuration from a ConfigMap or Secret and provide cloud credentials through
the cluster identity model.

```text
kind: CronJob
  -> Pod
    -> secrets-sync pipeline --config /config/config.yaml
      -> Vault / AWS Secrets Manager / S3 / AWS discovery APIs
```

The Helm chart supports both a direct runner CronJob and the
`secrets-sync-controller` Deployment. The controller watches
`CredentialSynchronization` resources and reconciles them into managed CronJobs
that execute the same `secrets-sync pipeline` command.

## Integration Boundaries

SecretSync owns the Go CLI, pipeline packages, release artifacts, GHCR image,
Docker action, Helm runner chart, Kubernetes controller, CRD schema, Lambda entrypoint, and
`secrets_sync` gopy binding. Python applications should use
`vendor_fabric.secrets_sync`, which wraps that binding with Extended Data
primitives, vendor connectors, optional `ProviderSession` handoff, and
redaction.

The stable cross-language contract is:

```bash
secrets-sync pipeline --config pipeline.yaml --output json
```

The JSON result envelope contains pipeline success, target count, secret change
counts, duration, per-target results, and optional diff output. SecretSync
redacts common bearer tokens, password or token assignments, API key
assignments, client secrets, and matching URL query parameters from top-level
and per-target error strings before serializing this envelope. Consumers should
still treat diff and error fields as operationally sensitive and apply their own
policy before writing logs, CI comments, or chat responses.
