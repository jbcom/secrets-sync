# Frequently Asked Questions

## General Questions

### What is SecretSync?

SecretSync is an enterprise-grade secret synchronization pipeline that automates the process of syncing secrets from HashiCorp Vault to AWS Secrets Manager and other secret stores. It features a two-phase architecture (merge → sync) with inheritance, dynamic discovery, and comprehensive CI/CD integration.

### How is SecretSync different from other secret sync tools?

| Feature | SecretSync | Alternatives |
|---------|------------|--------------|
| **Two-Phase Pipeline** | ✅ Merge → Sync with inheritance | ❌ Simple 1:1 sync only |
| **AWS Organizations** | ✅ Dynamic discovery with tag filtering | ❌ Manual account management |
| **Secret Versioning** | ✅ Complete audit trail with rollback | ❌ No version tracking |
| **Enhanced Diff** | ✅ Side-by-side with intelligent masking | ❌ Basic text diff |
| **Enterprise Scale** | ✅ 1000+ accounts, circuit breakers | ❌ Limited scalability |
| **CI/CD Integration** | ✅ GitHub Action + exit codes | ❌ Manual scripting required |

### Is SecretSync production-ready?

Yes! SecretSync v1.2.0 is production-ready with:
- 150+ comprehensive tests
- Full CI/CD pipeline with integration tests
- Circuit breakers and error handling
- Prometheus metrics for monitoring
- Used in production environments

## Installation & Setup

### What are the system requirements?

- **Go**: 1.21+ (if building from source)
- **Operating System**: Linux, macOS, or Windows
- **Memory**: 256MB minimum, 512MB recommended
- **Network**: HTTPS access to Vault and AWS APIs

### How do I install SecretSync?

Multiple installation options:
```bash
# Binary download
curl -LO https://github.com/extended-data-library/secretssync/releases/latest/download/secretsync-linux-amd64

# Go install
go install github.com/extended-data-library/secretssync/cmd/secretsync@latest

# Docker
docker pull extended-data-library/secretssync:latest

# Helm
helm install secretsync oci://registry-1.docker.io/extended-data-library/secretssync
```

### What permissions does SecretSync need?

**Vault Permissions:**
```hcl
# Read access to secret paths
path "secret/data/*" {
  capabilities = ["read", "list"]
}
path "secret/metadata/*" {
  capabilities = ["list"]
}
```

**AWS Permissions:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:ListSecrets",
        "secretsmanager:GetSecretValue",
        "secretsmanager:CreateSecret",
        "secretsmanager:UpdateSecret"
      ],
      "Resource": "*"
    }
  ]
}
```

## Configuration

### What's the difference between sources and targets?

- **Sources**: Where SecretSync reads secrets from (e.g., Vault paths)
- **Targets**: Where SecretSync writes secrets to (e.g., AWS Secrets Manager)
- **Merge Store**: Optional intermediate storage for complex inheritance

### How does inheritance work?

Inheritance allows targets to import configuration from other targets:

```yaml
targets:
  base:
    imports: [common-secrets]
  
  staging:
    inherits: base  # Gets everything from base
    imports: [staging-overrides]  # Plus staging-specific secrets
  
  production:
    inherits: staging  # Gets base + staging
    imports: [prod-overrides]  # Plus production-specific secrets
```

### Can I sync to multiple AWS accounts?

Yes! Use cross-account IAM roles:

```yaml
targets:
  dev-account:
    aws_secretsmanager:
      role_arn: "arn:aws:iam::111111111111:role/SecretSyncRole"
      region: "us-east-1"
  
  prod-account:
    aws_secretsmanager:
      role_arn: "arn:aws:iam::222222222222:role/SecretSyncRole"
      region: "us-east-1"
```

### How do I handle different environments?

Use separate configuration files or conditional imports:

```yaml
# Option 1: Separate configs
# config-dev.yaml, config-staging.yaml, config-prod.yaml

# Option 2: Environment-specific sources
sources:
  base-secrets:
    vault:
      path: "secret/data/base"
  
  dev-secrets:
    vault:
      path: "secret/data/dev"
  
  prod-secrets:
    vault:
      path: "secret/data/prod"

targets:
  development:
    imports: [base-secrets, dev-secrets]
  
  production:
    imports: [base-secrets, prod-secrets]
```

## Features

### What's new in v1.2.0?

Major new features:
- **Enhanced AWS Organizations Discovery** with tag filtering and wildcards
- **AWS Identity Center Integration** for permission set discovery
- **Secret Versioning System** with S3-based storage and rollback
- **Enhanced Diff Output** with side-by-side comparison and value masking

### How does secret versioning work?

SecretSync tracks every secret change with metadata:

```bash
# View version history
secretsync versions --secret-path "app/database/password"

# Rollback to specific version
secretsync sync --version 5 --target production

# Configure retention
versioning:
  enabled: true
  s3_bucket: "my-secretsync-versions"
  retention_days: 90
```

### What is the merge store?

The merge store is intermediate storage (Vault or S3) that holds merged secrets for complex inheritance scenarios:

```yaml
merge_store:
  s3:
    bucket: "my-merge-store"
    prefix: "merged/"

targets:
  staging:
    imports: [base-secrets]
  
  production:
    inherits: staging  # Reads from merge store
    imports: [prod-overrides]
```

### How does AWS Organizations discovery work?

Automatically discover and sync to accounts based on tags and OUs:

```yaml
discovery:
  aws_organizations:
    enabled: true
    tag_filters:
      - key: "Environment"
        values: ["production", "staging"]
        operator: "equals"
      - key: "Team"
        values: ["platform*"]
        operator: "contains"
    organizational_units:
      - "ou-production-12345"
    tag_logic: "AND"
```

## Operations

### How do I run SecretSync in CI/CD?

Use the GitHub Action:

```yaml
- name: Sync Secrets
  uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
    dry-run: 'false'
  env:
    VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
    VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

Or use exit codes for pipeline control:

```bash
# Check for changes (exit code 1 if changes detected)
secretsync pipeline --config config.yaml --dry-run --exit-code

# Apply changes only if needed
if [ $? -eq 1 ]; then
  secretsync pipeline --config config.yaml
fi
```

### How do I monitor SecretSync?

Enable Prometheus metrics:

```bash
secretsync pipeline --config config.yaml --metrics-port 9090
```

Available metrics:
- `secretsync_vault_api_call_duration_seconds`
- `secretsync_aws_api_call_duration_seconds`
- `secretsync_pipeline_execution_duration_seconds`
- `secretsync_pipeline_errors_total`

### What happens if SecretSync fails?

SecretSync includes several reliability features:
- **Circuit Breakers**: Prevent cascade failures
- **Retry Logic**: Automatic retry with exponential backoff
- **Error Context**: Detailed error messages with request IDs
- **Partial Failure Handling**: Continue processing other targets if one fails

### How do I handle secrets that are too large?

AWS Secrets Manager has a 64KB limit per secret. For larger secrets:

1. **Split into multiple secrets**:
   ```yaml
   # In Vault: secret/app/config
   # In AWS: app/config/part1, app/config/part2
   ```

2. **Use S3 for large files**:
   ```yaml
   # Store large files in S3, reference in secrets
   ```

3. **Compress data**:
   ```yaml
   # Use gzip compression for text-based secrets
   ```

## Troubleshooting

### "Vault authentication failed"

Common causes and solutions:

1. **Incorrect credentials**:
   ```bash
   # Verify environment variables
   echo $VAULT_ROLE_ID
   echo $VAULT_SECRET_ID
   ```

2. **Network connectivity**:
   ```bash
   # Test Vault connectivity
   curl -k $VAULT_ADDR/v1/sys/health
   ```

3. **Policy issues**:
   ```bash
   # Check Vault policies
   vault auth -method=approle role_id=$VAULT_ROLE_ID secret_id=$VAULT_SECRET_ID
   vault token lookup
   ```

### "AWS access denied"

Common causes and solutions:

1. **Missing permissions**:
   ```bash
   # Test AWS connectivity
   aws secretsmanager list-secrets --region us-east-1
   ```

2. **Role assumption issues**:
   ```bash
   # Test role assumption
   aws sts assume-role --role-arn arn:aws:iam::123456789012:role/SecretSyncRole --role-session-name test
   ```

3. **Region mismatch**:
   ```yaml
   # Ensure regions match
   aws:
     region: "us-east-1"
   targets:
     production:
       aws_secretsmanager:
         region: "us-east-1"  # Must match
   ```

### "Secret not found in Vault"

1. **Check path format**:
   ```yaml
   # KV2 engine uses /data/ in path
   sources:
     app-secrets:
       vault:
         path: "secret/data/myapp"  # Not "secret/myapp"
   ```

2. **Verify secret exists**:
   ```bash
   vault kv list secret/
   vault kv get secret/myapp
   ```

3. **Check permissions**:
   ```bash
   vault kv get secret/myapp  # Should work with your token
   ```

### Performance is slow

1. **Enable caching**:
   ```yaml
   discovery:
     aws_organizations:
       cache_ttl: "1h"  # Cache discovery results
   ```

2. **Reduce parallelism**:
   ```yaml
   # If hitting rate limits
   aws:
     max_retries: 5
     retry_delay: "1s"
   ```

3. **Use merge store**:
   ```yaml
   # Reduces Vault calls for complex inheritance
   merge_store:
     s3:
       bucket: "my-merge-store"
   ```

## Security

### Are secrets logged?

No, SecretSync never logs secret values. It only logs:
- Secret paths and names
- Operation metadata (duration, status)
- Error messages (without secret content)

### How are secrets handled in memory?

- Secrets are cleared from memory after use
- No secrets are written to disk unencrypted
- Memory dumps could potentially expose secrets (run in secure environments)

### What about secrets in diff output?

Secrets are masked by default in diff output:

```bash
# Masked (default)
secretsync pipeline --config config.yaml --dry-run

# Show values (use with caution)
secretsync pipeline --config config.yaml --dry-run --show-values
```

### How do I report security issues?

Use [GitHub Security Advisories](https://github.com/extended-data-library/secretssync/security/advisories) to report security vulnerabilities privately.

## Development

### How do I contribute?

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Submit a pull request

See [CONTRIBUTING.md](../CONTRIBUTING.md) for detailed guidelines.

### How do I add a new secret store?

1. Implement the `Store` interface
2. Add configuration schema
3. Write comprehensive tests
4. Update documentation

See the [development guide](../CONTRIBUTING.md#adding-a-new-secret-store) for details.

### How do I run tests?

```bash
# Unit tests
go test ./...

# Integration tests (requires Docker)
make test-integration-docker

# With coverage
go test -cover ./...

# With race detection
go test -race ./...
```

## Support

### Where can I get help?

- **Documentation**: [docs/](https://github.com/extended-data-library/secretssync/tree/main/docs)
- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and community support
- **Examples**: [examples/](https://github.com/extended-data-library/secretssync/tree/main/examples)

### How do I request a feature?

1. Check existing [issues](https://github.com/extended-data-library/secretssync/issues) and [discussions](https://github.com/extended-data-library/secretssync/discussions)
2. Create a [feature request](https://github.com/extended-data-library/secretssync/issues/new/choose)
3. Provide detailed use case and requirements

### Is there a roadmap?

See [ROADMAP.md](./ROADMAP.md) for planned features and timeline.

---

**Didn't find your question?** [Ask in GitHub Discussions](https://github.com/extended-data-library/secretssync/discussions) or [create an issue](https://github.com/extended-data-library/secretssync/issues/new/choose).