# SecretSync GitHub Action - Quick Reference

## Installation

```yaml
- uses: extended-data-library/secretssync@v1
```

## Minimal Example

```yaml
- name: Sync Secrets
  uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
  env:
    VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
    VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

## All Inputs

| Input | Default | Description |
|-------|---------|-------------|
| `config` | `config.yaml` | Path to configuration file |
| `targets` | `""` | Comma-separated target list |
| `dry-run` | `false` | Run without making changes |
| `merge-only` | `false` | Only run merge phase |
| `sync-only` | `false` | Only run sync phase |
| `discover` | `false` | Enable dynamic discovery |
| `output-format` | `github` | Output format (human, json, github, compact) |
| `compute-diff` | `false` | Show diff even without dry-run |
| `exit-code` | `false` | Use exit codes (0=no changes, 1=changes, 2=errors) |
| `log-level` | `info` | Log level (debug, info, warn, error) |
| `log-format` | `text` | Log format (text, json) |

## Common Patterns

### Dry Run (PR Validation)

```yaml
- uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
    dry-run: 'true'
    output-format: 'github'
```

### Specific Targets

```yaml
- uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
    targets: 'Staging,Production'
```

### Merge Only

```yaml
- uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
    merge-only: 'true'
```

### With Exit Codes

```yaml
- uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
    dry-run: 'true'
    exit-code: 'true'
  continue-on-error: true
```

### Debug Mode

```yaml
- uses: extended-data-library/secretssync@v1
  with:
    config: config.yaml
    log-level: 'debug'
    log-format: 'json'
```

## Complete Workflow

```yaml
name: Sync Secrets

on:
  schedule:
    - cron: '0 */6 * * *'
  workflow_dispatch:

jobs:
  sync:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS
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

## Environment Variables

SecretSync supports all environment variables from the CLI. Common ones:

- `VAULT_ADDR`: Vault address
- `VAULT_TOKEN`: Vault token (alternative to AppRole)
- `VAULT_ROLE_ID`: AppRole role ID
- `VAULT_SECRET_ID`: AppRole secret ID
- `VAULT_NAMESPACE`: Vault namespace
- `AWS_REGION`: AWS region
- `AWS_ACCESS_KEY_ID`: AWS access key (prefer OIDC)
- `AWS_SECRET_ACCESS_KEY`: AWS secret (prefer OIDC)

## AWS Authentication

### Recommended: OIDC

```yaml
- name: Configure AWS Credentials
  uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
    aws-region: us-east-1
```

### Alternative: Access Keys (Not Recommended)

```yaml
env:
  AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
  AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
```

## Vault Authentication

### AppRole (Recommended)

```yaml
vault:
  auth:
    approle:
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
```

```yaml
env:
  VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
  VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

### Token (Alternative)

```yaml
vault:
  auth:
    token:
      token: ${VAULT_TOKEN}
```

```yaml
env:
  VAULT_TOKEN: ${{ secrets.VAULT_TOKEN }}
```

## Output Formats

### `github` (Default for Action)

Shows GitHub Actions annotations in workflow logs.

### `json`

Machine-readable JSON output.

### `compact`

One-line summary (good for CI status).

### `human`

Colorful terminal output.

## Exit Codes

When `exit-code: 'true'`:

- `0`: No changes (success)
- `1`: Changes detected (considered "failure" for branching)
- `2`: Errors occurred (actual failure)

Use with `continue-on-error: true` to handle:

```yaml
- name: Check Changes
  id: check
  uses: extended-data-library/secretssync@v1
  with:
    dry-run: 'true'
    exit-code: 'true'
  continue-on-error: true

- name: Act on Changes
  if: steps.check.outcome == 'failure' && steps.check.conclusion == 'success'
  run: echo "Changes detected!"
```

## Troubleshooting

### Config File Not Found

```yaml
- uses: actions/checkout@v4  # Must checkout first!
- uses: extended-data-library/secretssync@v1
  with:
    config: path/to/config.yaml  # Relative to repo root
```

### Authentication Errors

Check that secrets are set in repository settings and environment variables are passed correctly.

### AWS AssumeRole Fails

Ensure OIDC is configured correctly and trust policy allows your repository.

## Resources

- **Full Documentation**: [docs/GITHUB_ACTIONS.md](./GITHUB_ACTIONS.md)
- **Examples**: [examples/](../examples/)
- **Support**: [docs/SUPPORT.md](./SUPPORT.md)
- **Security**: [docs/SECURITY.md](./SECURITY.md)
- **Marketplace**: [docs/MARKETPLACE.md](./MARKETPLACE.md)

## Version Pinning

```yaml
# Recommended: Pin to major version
uses: extended-data-library/secretssync@v1

# More stable: Pin to specific version
uses: extended-data-library/secretssync@v1.0.0

# Not recommended: Latest from main
uses: extended-data-library/secretssync@main
```

## License

MIT - See [LICENSE](../LICENSE)
