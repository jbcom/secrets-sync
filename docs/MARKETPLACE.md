# GitHub Marketplace Guide

This guide describes the GitHub Marketplace surface for the standalone
`jbcom/secrets-sync` action.

## Listing Summary

| Field | Value |
| --- | --- |
| Name | `SecretSync` |
| Repository | `jbcom/secrets-sync` |
| Category | Deployment, Continuous Integration, Security |
| License | MIT |
| Pricing | Free |

SecretSync synchronizes secrets from HashiCorp Vault into AWS Secrets Manager
with a two-phase merge and sync pipeline. It is designed for multi-account AWS
environments, AWS Organizations discovery, and CI/CD validation workflows.

## Supported Runtime Surface

- GitHub Action using `action.yml`.
- Docker image `ghcr.io/jbcom/secrets-sync:v2.3.1`.
- Go CLI `secrets-sync`.
- Kubernetes CronJob or `CredentialSynchronization` controller using the same
  GHCR image.
- AWS Lambda archive for scheduled or event-driven pipeline runs.
- Python binding distribution `secrets-sync-python-binding` for direct gopy
  integration.
- Vault KV2 sources.
- AWS Secrets Manager targets.
- Vault or S3 merge stores.
- GitHub-native output for PR validation and CI logs.

Keep Marketplace examples limited to this shipped runtime surface and the
currently supported store matrix.

## Release Tags

Release-please manages releases for the root package named `secrets-sync`.
Marketplace examples should therefore use plain semver release tags:

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
```

Do not document old monorepo package tags using the `secrets-sync-v...` shape.

`@main` may be useful for development testing, but it is not a stable
Marketplace recommendation. Moving major aliases such as `@v1` should only be
documented if the repository intentionally creates and maintains those aliases.

## Marketplace Requirements

- Repository is public.
- `action.yml` exists in the repository root.
- Action metadata has name, description, author, inputs, branding, and Docker
  image reference.
- README includes a working action example.
- `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, privacy docs, and support docs
  are present.
- Workflow examples use least-privilege permissions.
- Third-party actions in maintained examples are pinned to exact commit SHAs.

## Publication Flow

1. Merge normal changes to `main` using Conventional Commit prefixes.
2. Let release-please open or update the release PR.
3. Merge the release PR after review.
4. Confirm the release workflow created a `vX.Y.Z` GitHub release.
5. Confirm GoReleaser uploaded binary assets and `checksums.txt`.
6. In the GitHub release UI, publish that release to Marketplace.
7. Verify the Marketplace page renders the README and action metadata correctly.

Do not create a manual release or manual version tag during normal publication.

## Recommended Usage Snippet

```yaml
name: Sync Secrets

on:
  workflow_dispatch:

permissions:
  id-token: write
  contents: read

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3

      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@e7f100cf4c008499ea8adda475de1042d6975c7b # v6.2.0
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: us-east-1

      - name: Sync Secrets
        uses: jbcom/secrets-sync@vX.Y.Z
        with:
          config: config.yaml
          output-format: github
        env:
          VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
          VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

## Metadata

`action.yml` should remain the source of truth for action metadata:

```yaml
name: "SecretSync"
author: "jbcom"
description: "Sync secrets from HashiCorp Vault to AWS Secrets Manager across multiple accounts"
branding:
  icon: "lock"
  color: "blue"
```

## Listing Copy

Use concise copy that matches the implementation:

> SecretSync syncs HashiCorp Vault secrets into AWS Secrets Manager across
> multiple AWS accounts. It supports merge-first pipelines, AWS Organizations
> discovery, dry-run diff output, and GitHub-native CI feedback.

## Verification

After publication:

```bash
gh release view vX.Y.Z --repo jbcom/secrets-sync
gh workflow run ci.yml --repo jbcom/secrets-sync
```

Also check:

- Marketplace page links to the standalone repository.
- The README usage example references `vX.Y.Z`.
- Inputs shown by Marketplace match `action.yml`.
- No docs mention old `secrets-sync-v...` monorepo package tags.

## FAQ

### Can users pin `@main`?

They can, but documentation should recommend a stable semver release tag because
`main` is mutable.

### Should we publish a `v1` alias?

Only if maintainers decide to update that alias intentionally for every
supported release. Release-please will not maintain it automatically.

### Does the Marketplace release publish binary assets?

No. GoReleaser publishes binary archives and checksums from the release
workflow. Marketplace publication exposes the action metadata from the GitHub
release.

### Does the action send data to jbcom?

No. The action runs in the user's GitHub Actions environment and talks directly
to the configured Vault and AWS accounts. See `docs/PRIVACY.md`.
