# Usage

SecretSync is a pipeline runner. It reads configured sources, merges secret
material into a merge store, and syncs the resulting bundles into one or more
target backends. AWS Secrets Manager is the default target; Azure Key Vault,
GCP Secret Manager, Kubernetes Secrets, a generic HTTP/webhook store, and Vault
are also supported.

## Configuration

Use the current pipeline configuration shape:

```yaml
vault:
  address: https://vault.example.com/
  namespace: platform/secrets
  auth:
    approle:
      mount: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

aws:
  region: us-east-1

sources:
  shared:
    vault:
      mount: shared
      paths:
        - app/*
  production-overrides:
    vault:
      mount: production

merge_store:
  s3:
    bucket: secrets-sync-merge-store
    prefix: merged/
    kms_key_id: alias/secrets-sync-merge-store

targets:
  staging:
    account_id: "111111111111"
    region: us-east-1
    secret_prefix: /staging/
    imports:
      - shared
  production:
    account_id: "222222222222"
    region: us-east-1
    secret_prefix: /production/
    imports:
      - staging
      - production-overrides

dynamic_targets:
  analytics_sandboxes:
    discovery:
      organizations:
        ous:
          - ou-xxxx-sandbox
        recursive: true
        tag_filters:
          - key: Team
            values:
              - analytics
            operator: equals
    imports:
      - shared
    secret_prefix: /sandbox/

pipeline:
  merge:
    parallel: 4
  sync:
    parallel: 4
    delete_orphans: false
  dry_run: false
  continue_on_error: true
```

`sources` define where secrets are read from. `merge_store` defines the
intermediate bundle store. `targets` define sync destinations and the source or
target imports that feed them. A target imports another target by listing that
target name in `imports`.

## Target backends

A target syncs to AWS Secrets Manager by default. To sync to a different store,
add a `backend:` block selecting the driver. The legacy AWS fields
(`account_id`, `role_arn`, `region`) apply only to the default AWS backend.

| Driver | Store | Required config |
|--------|-------|-----------------|
| `aws` (default) | AWS Secrets Manager | `account_id` / cross-account `role_arn` |
| `azure` | Azure Key Vault | `options.vault_url` |
| `gcp` | GCP Secret Manager | `path` (project ID) |
| `kubernetes` | Kubernetes Secrets | `path` (namespace); `options.secret_type` |
| `http` | Generic HTTP/webhook store | `options.base_url`; auth via `bearer_token` / headers / mTLS |
| `vault` | HashiCorp Vault KV2 | `path` (mount) |

```yaml
targets:
  azure-vault:
    imports:
      - shared
    backend:
      driver: azure
      options:
        vault_url: https://my-keyvault.vault.azure.net/
  gcp-project:
    imports:
      - shared
    backend:
      driver: gcp
      path: my-gcp-project
  k8s-namespace:
    imports:
      - shared
    backend:
      driver: kubernetes
      path: team-platform
      options:
        secret_type: Opaque
```

Authentication is environment-native per provider: AWS uses the standard
credential chain plus cross-account role assumption; Azure uses
DefaultAzureCredential (service principal, managed identity, workload identity
federation); GCP uses Application Default Credentials; Kubernetes uses
in-cluster config, `KUBECONFIG`, or `~/.kube/config`; the HTTP store uses a
bearer token, custom headers, or mTLS client certificates.

A full multi-provider example lives at
[`examples/multi-provider-targets.yaml`](../examples/multi-provider-targets.yaml).

## Sync policies (policy as code)

Gate which sources may sync to which targets with declarative allow/deny rules.
Policies are validated during `secrets-sync validate` (invalid regexes or
actions fail there) and enforced before the sync phase writes anything — a
denied target is reported in `--dry-run` too.

```yaml
policy:
  default_action: allow   # allow (opt-in) | deny (allowlist)
  rules:
    - name: keep-prod-secrets-out-of-dev
      source: "^prod-"
      target: "^dev-"
      action: deny
    - name: only-shared-to-sandbox
      source: "^shared$"
      target: "^sandbox"
      action: allow
```

`source` and `target` are regular expressions matched against the source and
target names; an empty pattern matches anything. Rules are evaluated in order
and the first match wins; if none match, `default_action` applies. An empty
policy permits everything.

## Distributed tracing

Enable OpenTelemetry tracing with an `observability.tracing` block. Spans cover
each merge/sync phase per target and each backend fetch, with attributes for
phase, target, operation, and driver. Jaeger is reached via its native OTLP
endpoint.

```yaml
observability:
  tracing:
    enabled: true
    exporter: otlp-grpc        # otlp-grpc | otlp-http | zipkin | stdout
    endpoint: localhost:4317   # honors OTEL_EXPORTER_* env vars when empty
    insecure: true
    sample_ratio: 1.0          # 0 = never, 1 = always (parent-based)
    service_name: secrets-sync
```

When `enabled` is false (the default), tracing installs a no-op provider and
adds no overhead.

## Validate

```bash
secrets-sync validate --config config.yaml
```

Validation checks required targets, account ID formats, merge store settings,
dynamic discovery settings, and target inheritance cycles.

## Plan

```bash
secrets-sync pipeline --config config.yaml --dry-run --diff --output json
```

Dry runs load the same configuration and compute the same target graph as an
apply run, but skip writes to destination stores.

## Apply

```bash
secrets-sync pipeline --config config.yaml --diff --output human
```

Use `--targets staging,production` to limit a run to selected targets and their
dependencies. Use `--merge-only` or `--sync-only` when an operational workflow
needs to split the two phases.

## CI/CD

```bash
secrets-sync pipeline --config config.yaml --dry-run --diff --output github --exit-code
```

`--exit-code` returns `1` when a diff is detected, which lets a workflow require
review before applying changes.

## Kubernetes

The supported Kubernetes deployment model is a scheduled pipeline runner. Use
the Helm chart in `deploy/charts/secrets-sync` or render an equivalent CronJob
that runs:

```bash
secrets-sync pipeline --config /config/config.yaml --diff --output json
```
