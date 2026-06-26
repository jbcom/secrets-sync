# Getting Started With SecretSync

This guide sets up a current SecretSync pipeline that reads secrets from Vault,
merges them into a merge store, and syncs selected targets into AWS Secrets
Manager.

## Prerequisites

- HashiCorp Vault with a KV v2 secrets engine.
- AWS credentials from an IAM role, workload identity, or access keys.
- Permission to read the configured Vault mounts.
- Permission to write the target AWS Secrets Manager secrets.

## Step 1: Install

Choose one runtime:

```bash
go install github.com/jbcom/secrets-sync/cmd/secrets-sync@latest
```

```bash
docker pull jbcom/secrets-sync:v1
alias secrets-sync='docker run --rm -v "$PWD":/workspace -w /workspace jbcom/secrets-sync:v1'
```

```bash
git clone https://github.com/jbcom/secrets-sync.git
cd secrets-sync
make build
./bin/secrets-sync version
```

## Step 2: Create A Pipeline Config

Create `config.yaml`:

```yaml
vault:
  address: https://vault.example.com/
  namespace: admin
  auth:
    approle:
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

aws:
  region: us-east-1
  execution_context:
    type: delegated_admin
    account_id: "123456789012"
  control_tower:
    enabled: true
    execution_role:
      name: AWSControlTowerExecution

sources:
  app-secrets:
    vault:
      mount: secret
      paths:
        - myapp

merge_store:
  vault:
    mount: merged-secrets

targets:
  production:
    account_id: "222222222222"
    region: us-east-1
    secret_prefix: myapp/
    imports:
      - app-secrets

pipeline:
  merge:
    parallel: 4
  sync:
    parallel: 4
    delete_orphans: false
  continue_on_error: true
```

Targets can import from sources or from other targets. Target-to-target imports
are how you model inheritance:

```yaml
targets:
  staging:
    account_id: "111111111111"
    imports:
      - app-secrets

  production:
    account_id: "222222222222"
    imports:
      - staging
```

## Step 3: Configure Credentials

```bash
export VAULT_ROLE_ID="your-role-id"
export VAULT_SECRET_ID="your-secret-id"
export AWS_REGION="us-east-1"
```

Prefer AWS role assumption or workload identity for deployed runners. Use static
AWS access keys only when your environment cannot provide an identity.

## Step 4: Validate And Inspect

```bash
secrets-sync validate --config config.yaml
secrets-sync graph --config config.yaml
```

Validation checks the config structure and dependency graph. Add `--check-aws`
when you want validation to test AWS credentials and access.

## Step 5: Dry Run

```bash
secrets-sync pipeline --config config.yaml --dry-run --diff --output json --exit-code
```

Exit codes are stable for automation:

- `0`: no changes
- `1`: changes detected
- `2`: errors

## Step 6: Apply

After reviewing the dry-run diff, run the apply path:

```bash
secrets-sync pipeline --config config.yaml --diff --output json
```

## Common Patterns

### Multi-Environment Inheritance

```yaml
sources:
  base-secrets:
    vault:
      mount: secret
      paths: [base]
  prod-secrets:
    vault:
      mount: secret
      paths: [production]

targets:
  staging:
    account_id: "111111111111"
    imports:
      - base-secrets

  production:
    account_id: "222222222222"
    imports:
      - staging
      - prod-secrets
```

### Cross-Account Sync

```yaml
targets:
  dev-account:
    account_id: "111111111111"
    region: us-east-1
    role_arn: arn:aws:iam::111111111111:role/SecretSyncRole
    imports:
      - dev-secrets

  prod-account:
    account_id: "222222222222"
    region: us-east-1
    role_arn: arn:aws:iam::222222222222:role/SecretSyncRole
    imports:
      - prod-secrets
```

### S3 Merge Store With Versioning

```yaml
merge_store:
  s3:
    bucket: my-secrets-sync-merge-store
    prefix: merged/
    kms_key_id: alias/secrets-sync
    versioning:
      enabled: true
      retain_versions: 90
```

### Dynamic AWS Organizations Targets

```yaml
dynamic_targets:
  production-accounts:
    discovery:
      organizations:
        ous:
          - ou-abcd-production
        tag_filters:
          - key: Environment
            values: ["production"]
            operator: equals
        recursive: true
    imports:
      - app-secrets
    region: us-east-1
    secret_prefix: myapp/
```

Run with discovery enabled:

```bash
secrets-sync pipeline --config config.yaml --discover --dry-run --diff
```

## GitHub Actions

```yaml
name: Sync Secrets
on:
  schedule:
    - cron: "0 */6 * * *"
  workflow_dispatch:

jobs:
  sync:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write
    steps:
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
      - uses: aws-actions/configure-aws-credentials@e7f100cf4c008499ea8adda475de1042d6975c7b # v6.2.0
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      - uses: jbcom/secrets-sync@secrets-sync-vX.Y.Z
        with:
          config: config.yaml
          dry-run: "true"
          compute-diff: "true"
          output-format: json
          exit-code: "true"
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

## Troubleshooting

### Vault authentication failed

- Verify `VAULT_ROLE_ID` and `VAULT_SECRET_ID`.
- Confirm the AppRole can read the configured mounts and paths.
- Confirm Vault address and namespace are reachable from the runner.

### AWS access denied

- Confirm the runner identity can assume the configured target role.
- Check Secrets Manager create, update, list, and delete permissions.
- Confirm the configured region and account IDs.

### No changes detected unexpectedly

- Run with `--output side-by-side` for a human diff.
- Check target imports and source path spelling.
- Run `secrets-sync graph --config config.yaml` to verify dependency order.

## Next Steps

- Read [PIPELINE.md](./PIPELINE.md) for the full configuration reference.
- Read [DEPLOYMENT.md](./DEPLOYMENT.md) for production deployment patterns.
- Read [OBSERVABILITY.md](./OBSERVABILITY.md) for metrics and logging.
- Read [GITHUB_ACTIONS.md](./GITHUB_ACTIONS.md) for CI integration.
