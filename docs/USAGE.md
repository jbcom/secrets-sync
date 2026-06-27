# Usage

SecretSync is a pipeline runner. It reads configured sources, merges secret
material into a merge store, and syncs the resulting bundles into AWS Secrets
Manager targets.

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
intermediate bundle store. `targets` define AWS accounts and the source or
target imports that feed them. A target imports another target by listing that
target name in `imports`.

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
