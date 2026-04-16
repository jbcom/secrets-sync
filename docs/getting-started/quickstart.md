# Quickstart

This guide walks through a minimal SecretSync dry run using the Go CLI and one
of the package example configs.

## 1. Install the CLI

```bash
go install github.com/jbcom/extended-data-library/packages/secretssync/cmd/secretsync@latest
```

## 2. Start from an example config

```bash
git clone https://github.com/jbcom/extended-data-library.git
cd extended-data-library/packages/secretssync
cp examples/pipeline-config.yaml pipeline.yaml
```

Edit `pipeline.yaml` for your Vault address, AWS role pattern, source paths,
and targets.

## 3. Export credentials

```bash
export VAULT_ADDR="https://vault.example.com"
export VAULT_ROLE_ID="your-role-id"
export VAULT_SECRET_ID="your-secret-id"
export AWS_REGION="us-east-1"
```

If you are not using ambient AWS credentials or OIDC, also export
`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.

## 4. Validate before syncing

```bash
secretsync validate --config pipeline.yaml
```

## 5. Run a dry run

```bash
secretsync pipeline --config pipeline.yaml --dry-run --output compact
```

## 6. Apply the pipeline

```bash
secretsync pipeline --config pipeline.yaml
```

## Next Steps

- Read [../GETTING_STARTED.md](../GETTING_STARTED.md) for a longer walkthrough
- Review [../PIPELINE.md](../PIPELINE.md) for config details and advanced flags
- See [../development/contributing.md](../development/contributing.md) if you
  want to work on the package
