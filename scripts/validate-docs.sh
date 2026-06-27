#!/usr/bin/env bash
# Lightweight documentation sanity checks for generated and hand-written docs.

set -euo pipefail

if grep -RIn --exclude-dir=_build --exclude-dir=api '<<<<<<<\|=======\|>>>>>>>' docs README.md HANDOFF.md SECRETS_SYNC_ALIGNMENT.md AGENTIC_REINFORCEMENT.md; then
  echo "Documentation contains merge conflict markers." >&2
  exit 1
fi

if grep -RInE --exclude-dir=_build --exclude-dir=api 'secretssync|\bsecretsync\b|cmd/secretsync|deploy/charts/secretsync|SECRETSYNC|vaultsecretsync|VaultSecretSync' docs README.md HANDOFF.md SECRETS_SYNC_ALIGNMENT.md AGENTIC_REINFORCEMENT.md; then
  echo "Documentation contains legacy SecretSync spellings." >&2
  exit 1
fi
