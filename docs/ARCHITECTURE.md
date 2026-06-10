# Architecture

SecretSync is a pipeline runner. The supported runtime is the `secretsync` CLI
executing a configured merge, sync, or full pipeline operation. Kubernetes and
GitHub Actions deployments wrap that same CLI contract instead of introducing a
separate controller API.

## Runtime Shape

```text
pipeline.yaml
    |
    v
secretsync pipeline --config pipeline.yaml
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

- **CLI entrypoint**: `cmd/secretsync` exposes `validate`, `pipeline`, and
  graph-related commands for local, CI, and scheduled execution.
- **Pipeline package**: `pkg/pipeline` owns config loading, validation,
  inheritance resolution, discovery, merge, sync, diff integration, and result
  envelopes.
- **Diff package**: `pkg/diff` builds masked human, JSON, GitHub Actions,
  compact, and side-by-side diff output.
- **Observability package**: `pkg/observability` exposes metrics for pipeline
  runs that opt into the metrics endpoint.
- **GitHub Action**: `action.yml` packages the CLI contract for CI/CD workflows.
- **Helm chart**: the chart renders a Kubernetes `CronJob` plus ConfigMap or
  existing config mount for scheduled pipeline execution.
- **Python integration**: `extended-data[secrets]` calls the supported CLI and
  consumes the JSON result envelope as mapping-style Python data.

## Deployment Models

### Local Or CI Execution

Run the CLI directly when an operator or engineer controls the execution
environment:

```bash
secretsync validate --config pipeline.yaml
secretsync pipeline --config pipeline.yaml --dry-run --diff --output json
secretsync pipeline --config pipeline.yaml --output json
```

GitHub Actions uses the same contract through the published action. The action
does not own a separate API surface; it validates inputs, executes the pipeline,
and reports outputs suitable for CI workflows.

### Kubernetes Scheduled Execution

For Kubernetes, run SecretSync as a `CronJob`. Mount the pipeline configuration
from a ConfigMap or Secret and provide cloud credentials through the cluster
identity model.

```text
kind: CronJob
  -> Pod
    -> secretsync pipeline --config /config/config.yaml
      -> Vault / AWS Secrets Manager / S3 / AWS discovery APIs
```

The Helm chart is intentionally a runner chart. It should not grow a custom
resource, reconciler, or sidecar service unless those components are owned as a
new public runtime contract.

## Integration Boundaries

SecretSync owns the Go CLI, pipeline packages, release artifact, Docker action,
and Helm runner chart. Python applications should use the `extended-data`
connector unless they are explicitly experimenting with the local gopy binding
sources in this repository.

The stable cross-language contract is:

```bash
secretsync pipeline --config pipeline.yaml --output json
```

The JSON result envelope contains pipeline success, target count, secret change
counts, duration, per-target results, and optional diff output. Consumers should
treat diff and error fields as potentially sensitive and redact or suppress
them before writing logs, CI comments, or chat responses.
