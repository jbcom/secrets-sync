# SecretSync GitHub Action - Quick Reference

## Installation

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
```

## Minimal Example

```yaml
- name: Sync Secrets
  uses: jbcom/secrets-sync@vX.Y.Z
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
| `output-format` | `github` | Output format (human, json, github, compact, side-by-side) |
| `compute-diff` | `false` | Show diff even without dry-run |
| `exit-code` | `false` | Use exit codes (0=no changes, 1=changes, 2=errors) |
| `continue-on-error` | `true` | Continue processing remaining targets after an error |
| `parallelism` | `0` | Maximum concurrent target operations (`0` uses config/default) |
| `metrics-addr` | `0.0.0.0` | Metrics server bind address |
| `metrics-port` | `0` | Metrics server port (`0` disables metrics) |
| `log-level` | `info` | Log level (debug, info, warn, error) |
| `log-format` | `text` | Log format (text, json) |

## Common Patterns

### Dry Run (PR Validation)

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
  with:
    config: config.yaml
    dry-run: 'true'
    output-format: 'github'
```

### Specific Targets

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
  with:
    config: config.yaml
    targets: 'Staging,Production'
```

### Merge Only

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
  with:
    config: config.yaml
    merge-only: 'true'
```

### With Exit Codes

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
  with:
    config: config.yaml
    dry-run: 'true'
    exit-code: 'true'
  continue-on-error: true
```

### Debug Mode

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
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
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
      
      - name: Configure AWS
        uses: aws-actions/configure-aws-credentials@e7f100cf4c008499ea8adda475de1042d6975c7b # v6.2.0
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1
      
      - name: Sync Secrets
        uses: jbcom/secrets-sync@vX.Y.Z
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
  uses: aws-actions/configure-aws-credentials@e7f100cf4c008499ea8adda475de1042d6975c7b # v6.2.0
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

Shows GitHub Actions annotations in workflow logs and writes these action
outputs when diff computation is enabled:

| Output | Description |
| --- | --- |
| `changes` | Total added, removed, and modified secrets |
| `added` | Secrets that would be added or were added |
| `removed` | Secrets that would be removed or were removed |
| `modified` | Secrets that would be modified or were modified |
| `unchanged` | Secrets with no detected changes |
| `zero_sum` | `true` when no changes are detected |

### `json`

Machine-readable pipeline result envelope. Diff details are nested under
`diff` and `diff_output` when diff computation is enabled.

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
  uses: jbcom/secrets-sync@vX.Y.Z
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
- uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
- uses: jbcom/secrets-sync@vX.Y.Z
  with:
    config: path/to/config.yaml  # Relative to repo root
```

### Authentication Errors

Check that secrets are set in repository settings and environment variables are passed correctly.

### AWS AssumeRole Fails

Ensure OIDC is configured correctly and trust policy allows your repository.

## Resources

- **Full Documentation**: [docs/GITHUB_ACTIONS.md](./GITHUB_ACTIONS.md)
- **Examples**: [examples/](https://github.com/jbcom/secrets-sync/tree/main/examples)
- **Support**: [docs/SUPPORT.md](./SUPPORT.md)
- **Security**: [docs/SECURITY.md](./SECURITY.md)
- **Marketplace**: [docs/MARKETPLACE.md](./MARKETPLACE.md)

## Version Pinning

```yaml
# Recommended: Pin to a package release tag
uses: jbcom/secrets-sync@vX.Y.Z

# Not recommended: Track the branch tip
uses: jbcom/secrets-sync@main
```

## License

MIT - See [LICENSE](../LICENSE)
