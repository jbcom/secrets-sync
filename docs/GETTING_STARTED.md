# Getting Started with SecretSync

This guide will walk you through setting up SecretSync from scratch to sync secrets from HashiCorp Vault to AWS Secrets Manager.

## Prerequisites

Before you begin, ensure you have:

- **HashiCorp Vault** with KV2 secrets engine enabled
- **AWS Account** with Secrets Manager access
- **Vault credentials** (AppRole recommended)
- **AWS credentials** (IAM role or access keys)

## Step 1: Installation

Choose your preferred installation method:

### Option A: Download Binary

```bash
# Download latest release
curl -LO https://github.com/extended-data-library/secretssync/releases/latest/download/secretsync-linux-amd64

# Make executable and move to PATH
chmod +x secretsync-linux-amd64
sudo mv secretsync-linux-amd64 /usr/local/bin/secretsync

# Verify installation
secretsync version
```

### Option B: Go Install

```bash
go install github.com/extended-data-library/secretssync/cmd/secretsync@latest
```

### Option C: Docker

```bash
# Pull image
docker pull extended-data-library/secretssync:latest

# Create alias for easier usage
alias secretsync='docker run --rm -v $(pwd):/workspace extended-data-library/secretssync'
```

## Step 2: Basic Configuration

Create a configuration file `config.yaml`:

```yaml
# Basic SecretSync configuration
vault:
  address: "https://your-vault.example.com"
  namespace: "admin"  # Optional: if using Vault namespaces
  auth:
    approle:
      role_id: "${VAULT_ROLE_ID}"
      secret_id: "${VAULT_SECRET_ID}"

aws:
  region: "us-east-1"
  # Optional: role to assume for cross-account access
  # role_arn: "arn:aws:iam::123456789012:role/SecretSyncRole"

# Define where to read secrets from
sources:
  app-secrets:
    vault:
      path: "secret/data/myapp"  # KV2 path

# Define where to write secrets to
targets:
  production:
    aws_secretsmanager:
      region: "us-east-1"
      # Optional: prefix for secret names
      prefix: "myapp/"
    imports:
      - app-secrets
```

## Step 3: Set Environment Variables

```bash
# Vault credentials
export VAULT_ROLE_ID="your-role-id"
export VAULT_SECRET_ID="your-secret-id"

# AWS credentials (if not using IAM roles)
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
```

## Step 4: Validate Configuration

Before running, validate your configuration:

```bash
secretsync validate --config config.yaml
```

This will check:
- Configuration syntax
- Vault connectivity
- AWS permissions
- Source/target accessibility

## Step 5: Dry Run

Perform a dry run to see what changes would be made:

```bash
secretsync pipeline --config config.yaml --dry-run
```

You should see output like:
```
Pipeline Diff Summary
=====================
  Added:     3 secrets
  Modified:  0 secrets
  Deleted:   0 secrets

⚠️  CHANGES DETECTED

Target: production
  + myapp/database-password
  + myapp/api-key
  + myapp/jwt-secret
```

## Step 6: Execute Sync

If the dry run looks correct, execute the actual sync:

```bash
secretsync pipeline --config config.yaml
```

## Step 7: Verify Results

Check AWS Secrets Manager to confirm your secrets were created:

```bash
# Using AWS CLI
aws secretsmanager list-secrets --query 'SecretList[?starts_with(Name, `myapp/`)]'

# Or check in AWS Console
# Navigate to AWS Secrets Manager in your region
```

## Next Steps

### Enable Advanced Features

#### 1. Add Observability (v1.1.0)

```bash
# Run with metrics endpoint
secretsync pipeline --config config.yaml --metrics-port 9090

# In another terminal, check metrics
curl http://localhost:9090/metrics
curl http://localhost:9090/health
```

#### 2. Enhanced Diff Output (v1.2.0)

```bash
# Side-by-side comparison
secretsync pipeline --config config.yaml --dry-run --format side-by-side

# JSON output for automation
secretsync pipeline --config config.yaml --dry-run --format json
```

#### 3. Secret Versioning (v1.2.0)

Add to your config:
```yaml
versioning:
  enabled: true
  s3_bucket: "my-secretsync-versions"
  retention_days: 90
```

#### 4. AWS Organizations Discovery (v1.2.0)

```yaml
discovery:
  aws_organizations:
    enabled: true
    tag_filters:
      - key: "Environment"
        values: ["production", "staging"]
        operator: "equals"
    cache_ttl: "1h"
```

### Set Up CI/CD

#### GitHub Actions

Create `.github/workflows/secretsync.yml`:

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
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

## Common Patterns

### Multi-Environment Setup

```yaml
sources:
  base-secrets:
    vault:
      path: "secret/data/base"
  
  prod-secrets:
    vault:
      path: "secret/data/production"

targets:
  staging:
    aws_secretsmanager:
      region: "us-east-1"
    imports:
      - base-secrets
  
  production:
    aws_secretsmanager:
      region: "us-east-1"
    imports:
      - base-secrets
      - prod-secrets  # Production-specific overrides
```

### Cross-Account Sync

```yaml
targets:
  dev-account:
    aws_secretsmanager:
      region: "us-east-1"
      role_arn: "arn:aws:iam::111111111111:role/SecretSyncRole"
    imports:
      - dev-secrets
  
  prod-account:
    aws_secretsmanager:
      region: "us-east-1"
      role_arn: "arn:aws:iam::222222222222:role/SecretSyncRole"
    imports:
      - prod-secrets
```

### Merge Store Pattern

```yaml
# Use S3 as merge store for complex inheritance
merge_store:
  s3:
    bucket: "my-secretsync-merge-store"
    prefix: "merged/"
    region: "us-east-1"

targets:
  staging:
    imports: [base-secrets]
  
  production:
    inherits: staging  # Inherit from staging's merged output
    imports: [prod-overrides]
```

## Troubleshooting

### Common Issues

#### "Vault authentication failed"
- Verify `VAULT_ROLE_ID` and `VAULT_SECRET_ID` are correct
- Check Vault policies allow access to specified paths
- Ensure Vault address is reachable

#### "AWS access denied"
- Verify AWS credentials are configured
- Check IAM permissions for Secrets Manager
- Ensure region is correct

#### "Secret not found"
- Verify Vault path exists and is accessible
- Check KV2 engine is enabled at the mount
- Ensure path format is correct (`secret/data/path` for KV2)

### Debug Mode

Enable debug logging for more details:

```bash
secretsync pipeline --config config.yaml --log-level debug
```

### Validate Permissions

Test individual components:

```bash
# Test Vault connectivity
vault auth -method=approle role_id=$VAULT_ROLE_ID secret_id=$VAULT_SECRET_ID
vault kv list secret/

# Test AWS connectivity
aws secretsmanager list-secrets --region us-east-1
```

## Getting Help

- **Documentation**: [Full docs](https://github.com/extended-data-library/secretssync/tree/main/docs)
- **Examples**: [Configuration examples](https://github.com/extended-data-library/secretssync/tree/main/examples)
- **Issues**: [GitHub Issues](https://github.com/extended-data-library/secretssync/issues)
- **Discussions**: [GitHub Discussions](https://github.com/extended-data-library/secretssync/discussions)

## What's Next?

- Explore [advanced configuration options](./PIPELINE.md)
- Set up [monitoring and observability](./OBSERVABILITY.md)
- Learn about [deployment patterns](./DEPLOYMENT.md)
- Integrate with [GitHub Actions](./GITHUB_ACTIONS.md)

Welcome to SecretSync! 🚀