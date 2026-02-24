# GitHub Actions Usage Guide

This guide shows how to use SecretSync as a GitHub Action for automated secrets synchronization in your CI/CD pipelines.

## Quick Start

### Basic Usage

```yaml
name: Sync Secrets
on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
  workflow_dispatch:

jobs:
  sync:
    runs-on: ubuntu-latest
    permissions:
      id-token: write  # Required for OIDC
      contents: read
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      
      - name: Sync Secrets
        uses: extended-data-library/secretssync@v1
        with:
          config: config.yaml
```

## Input Parameters

All inputs correspond to CLI flags and are optional:

| Input | Description | Default | CLI Flag |
|-------|-------------|---------|----------|
| `config` | Path to SecretSync configuration file | `config.yaml` | `--config` |
| `targets` | Comma-separated list of targets | `""` (all) | `--targets` |
| `dry-run` | Run without making changes | `false` | `--dry-run` |
| `merge-only` | Only run merge phase | `false` | `--merge-only` |
| `sync-only` | Only run sync phase | `false` | `--sync-only` |
| `discover` | Enable dynamic target discovery | `false` | `--discover` |
| `output-format` | Output format (human, json, github, compact) | `github` | `--output` |
| `compute-diff` | Show diff even without dry-run | `false` | `--diff` |
| `exit-code` | Use exit codes for CI/CD | `false` | `--exit-code` |
| `log-level` | Logging level (debug, info, warn, error) | `info` | `--log-level` |
| `log-format` | Log format (text, json) | `text` | `--log-format` |

## Usage Examples

### 1. Dry Run with Pull Request Validation

Validate configuration changes in pull requests:

```yaml
name: Validate Secrets Config

on:
  pull_request:
    paths:
      - 'config.yaml'
      - 'secrets/**'

jobs:
  validate:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
      pull-requests: write  # For PR comments
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      
      - name: Validate Changes (Dry Run)
        uses: extended-data-library/secretssync@v1
        with:
          config: config.yaml
          dry-run: 'true'
          output-format: 'github'
          exit-code: 'true'
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

### 2. Manual Workflow with Target Selection

Allow manual execution with specific targets:

```yaml
name: Sync Secrets (Manual)

on:
  workflow_dispatch:
    inputs:
      targets:
        description: 'Targets to sync (comma-separated or "all")'
        required: false
        default: 'all'
      dry_run:
        description: 'Dry run mode'
        type: boolean
        default: false

jobs:
  sync:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      
      - name: Sync Secrets
        uses: extended-data-library/secretssync@v1
        with:
          config: config.yaml
          targets: ${{ github.event.inputs.targets != 'all' && github.event.inputs.targets || '' }}
          dry-run: ${{ github.event.inputs.dry_run }}
          output-format: 'github'
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

### 3. Scheduled Sync with Merge Only

Merge secrets from sources without syncing to AWS (useful for testing):

```yaml
name: Merge Secrets Only

on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM UTC

jobs:
  merge:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Merge Secrets
        uses: extended-data-library/secretssync@v1
        with:
          config: config.yaml
          merge-only: 'true'
          log-level: 'debug'
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

### 4. Dynamic Target Discovery

Automatically discover and sync to accounts from AWS Organizations:

```yaml
name: Sync with Dynamic Discovery

on:
  schedule:
    - cron: '0 */4 * * *'  # Every 4 hours

jobs:
  sync:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      
      - name: Sync with Discovery
        uses: extended-data-library/secretssync@v1
        with:
          config: config.yaml
          discover: 'true'
          output-format: 'github'
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

### 5. CI/CD with Exit Codes

Use exit codes to control pipeline behavior:

```yaml
name: Secrets Pipeline

on:
  push:
    branches: [main]
    paths:
      - 'config.yaml'

jobs:
  check:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      
      - name: Check for Changes
        id: check
        uses: extended-data-library/secretssync@v1
        with:
          config: config.yaml
          dry-run: 'true'
          exit-code: 'true'
          output-format: 'compact'
        continue-on-error: true
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
      
      - name: Apply Changes
        if: steps.check.outcome == 'failure'
        uses: extended-data-library/secretssync@v1
        with:
          config: config.yaml
          output-format: 'github'
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

### 6. Multiple Environments

Sync different configurations for different environments:

```yaml
name: Multi-Environment Sync

on:
  workflow_dispatch:
    inputs:
      environment:
        description: 'Environment to sync'
        required: true
        type: choice
        options:
          - development
          - staging
          - production

jobs:
  sync:
    runs-on: ubuntu-latest
    environment: ${{ github.event.inputs.environment }}
    permissions:
      id-token: write
      contents: read
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      
      - name: Sync Secrets
        uses: extended-data-library/secretssync@v1
        with:
          config: configs/${{ github.event.inputs.environment }}.yaml
          output-format: 'github'
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

## Configuration File

Your repository should include a SecretSync configuration file. Example:

```yaml
# config.yaml
vault:
  address: https://vault.example.com
  namespace: admin
  auth:
    approle:
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

aws:
  region: us-east-1
  control_tower:
    enabled: true
    execution_role:
      name: AWSControlTowerExecution

sources:
  analytics:
    vault:
      mount: analytics
      paths: ['*']

merge_store:
  vault:
    mount: merged-secrets

targets:
  Staging:
    account_id: "111111111111"
    imports:
      - analytics
  
  Production:
    account_id: "222222222222"
    imports:
      - Staging
```

## Security Best Practices

### 1. Use OIDC Instead of Long-Lived Credentials

**Recommended:**
```yaml
- name: Configure AWS Credentials
  uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
    aws-region: us-east-1
```

**Not Recommended:**
```yaml
env:
  AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
  AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
```

### 2. Minimal Permissions

Use GitHub Actions permissions to grant only what's needed:

```yaml
permissions:
  id-token: write    # For OIDC
  contents: read     # To checkout code
  # Don't grant unnecessary permissions
```

### 3. Environment Protection Rules

Use GitHub Environments for production deployments:

```yaml
jobs:
  sync-production:
    environment: production  # Requires approval
    steps:
      - uses: extended-data-library/secretssync@v1
        with:
          config: production.yaml
```

### 4. Secrets Scoping

Store secrets at the appropriate scope:
- **Repository secrets**: For repository-specific credentials
- **Environment secrets**: For environment-specific credentials
- **Organization secrets**: For shared credentials across repos

### 5. Use Environment Variables for Sensitive Data

Never hardcode secrets in configuration:

```yaml
# ✅ Good - uses environment variables
vault:
  auth:
    approle:
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

# ❌ Bad - hardcoded credentials
vault:
  auth:
    approle:
      role_id: abc123
      secret_id: xyz789
```

## AWS IAM Role Setup for OIDC

To use OIDC authentication with AWS:

### 1. Create OIDC Provider in AWS

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

### 2. Create IAM Role with Trust Policy

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:your-org/your-repo:*"
        }
      }
    }
  ]
}
```

### 3. Attach Permissions Policy

The role needs permissions to:
- Assume roles in target accounts (for Control Tower pattern)
- Access Organizations API (for dynamic discovery)
- Access Identity Center API (for dynamic discovery)

Example policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sts:AssumeRole"
      ],
      "Resource": "arn:aws:iam::*:role/AWSControlTowerExecution"
    },
    {
      "Effect": "Allow",
      "Action": [
        "organizations:ListAccounts",
        "organizations:DescribeAccount",
        "organizations:ListAccountsForParent"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "sso:ListInstances",
        "sso:ListAccountAssignments",
        "identitystore:ListUsers",
        "identitystore:ListGroups"
      ],
      "Resource": "*"
    }
  ]
}
```

## Troubleshooting

### Action Fails with "Config file not found"

Ensure your config file is in the repository and the path is correct:

```yaml
- uses: actions/checkout@v4  # Required to access repository files

- uses: extended-data-library/secretssync@v1
  with:
    config: path/to/config.yaml  # Relative to repo root
```

### Vault Authentication Fails

Verify environment variables are set correctly:

```yaml
- uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
    log-level: debug  # Enable debug logging
  env:
    VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
    VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

### AWS AssumeRole Fails

Check:
1. OIDC provider is configured in AWS
2. Trust policy allows your repository
3. Role has necessary permissions
4. Control Tower role exists in target accounts

### GitHub Actions Annotations Not Showing

Ensure `output-format` is set to `github`:

```yaml
- uses: extended-data-library/secretssync@v1
  with:
    output-format: 'github'  # Enables GitHub Actions annotations
```

## Exit Codes

When `exit-code: 'true'` is enabled:

| Exit Code | Meaning | Use Case |
|-----------|---------|----------|
| `0` | No changes detected | CI passes, no action needed |
| `1` | Changes detected | Trigger downstream jobs |
| `2` | Errors occurred | Fail the pipeline |

Example using exit codes:

```yaml
- name: Check for Changes
  id: check
  uses: extended-data-library/secretssync@v1
  with:
    dry-run: 'true'
    exit-code: 'true'
  continue-on-error: true

- name: Notify on Changes
  if: steps.check.outcome == 'failure' && steps.check.conclusion == 'success'
  run: echo "Changes detected!"

- name: Notify on Errors
  if: steps.check.outcome == 'failure' && steps.check.conclusion == 'failure'
  run: echo "Errors occurred!"
```

## Advanced Configuration

### Using Matrix Strategy

Sync multiple configurations in parallel:

```yaml
jobs:
  sync:
    strategy:
      matrix:
        environment: [dev, staging, prod]
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets[format('AWS_ROLE_{0}', matrix.environment)] }}
          aws-region: us-east-1
      
      - name: Sync Secrets
        uses: extended-data-library/secretssync@v1
        with:
          config: configs/${{ matrix.environment }}.yaml
```

### Conditional Execution

Run only when configuration changes:

```yaml
on:
  push:
    paths:
      - 'config.yaml'
      - 'configs/**'

jobs:
  sync:
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: extended-data-library/secretssync@v1
```

### Composite Actions

Create reusable workflows:

```yaml
# .github/actions/secretsync/action.yml
name: 'SecretSync Wrapper'
description: 'Configured SecretSync action'
inputs:
  config:
    required: true

runs:
  using: composite
  steps:
    - uses: aws-actions/configure-aws-credentials@v4
      with:
        role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
        aws-region: us-east-1
    
    - uses: extended-data-library/secretssync@v1
      with:
        config: ${{ inputs.config }}
        output-format: 'github'
      env:
        VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
        VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

## Support

- **Documentation**: [Full docs](https://github.com/extended-data-library/secretssync/tree/main/docs)
- **Issues**: [GitHub Issues](https://github.com/extended-data-library/secretssync/issues)
- **Discussions**: [GitHub Discussions](https://github.com/extended-data-library/secretssync/discussions)

## License

MIT License - see [LICENSE](https://github.com/extended-data-library/secretssync/blob/main/LICENSE)
