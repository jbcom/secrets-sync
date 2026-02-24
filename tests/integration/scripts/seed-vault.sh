#!/bin/sh
# Seed Vault with test fixtures from secrets_seed.json
# This script runs inside the vault-seeder container

set -e

echo "=== Starting Vault seeding ==="

# Wait for Vault to be fully ready
until vault status > /dev/null 2>&1; do
  echo "Waiting for Vault to be ready..."
  sleep 2
done

echo "Vault is ready, proceeding with seeding..."

# Enable KV v2 secrets engine at 'secret/'
echo "Enabling KV v2 secrets engine..."
vault secrets enable -version=2 -path=secret kv || echo "Secret engine already enabled"

# Parse JSON and seed secrets
# Using jq to extract and iterate over sources
if [ ! -f /testdata/secrets_seed.json ]; then
  echo "ERROR: /testdata/secrets_seed.json not found"
  exit 1
fi

echo "Seeding secrets from secrets_seed.json..."

# Extract all source mounts
SOURCES=$(cat /testdata/secrets_seed.json | jq -r '.sources | keys[]')

for source in $SOURCES; do
  echo "Processing source: $source"
  
  # Get all secret paths for this source
  PATHS=$(cat /testdata/secrets_seed.json | jq -r ".sources[\"$source\"] | keys[]")
  
  for path in $PATHS; do
    echo "  Writing secret: $source/$path"
    
    # Extract secret data and write to Vault
    SECRET_DATA=$(cat /testdata/secrets_seed.json | jq -c ".sources[\"$source\"][\"$path\"]")
    
    # Write to Vault KV v2 (note: 'data' in path for KV v2)
    vault kv put "secret/$source/$path" data="$SECRET_DATA" > /dev/null 2>&1 || {
      echo "    WARNING: Failed to write $source/$path"
    }
  done
done

# Create some nested paths to test recursive listing
echo "Creating nested test paths..."
for i in 1 2 3; do
  for j in 1 2; do
    vault kv put "secret/nested/level$i/sublevel$j/config" \
      data="{\"level\": $i, \"sublevel\": $j, \"test\": \"recursive-listing\"}" > /dev/null 2>&1
  done
done

# Create edge case paths
echo "Creating edge case paths..."
vault kv put "secret/edge-cases/trailing-slash-test/data" \
  data='{"note": "tests path normalization"}' > /dev/null 2>&1
vault kv put "secret/edge-cases/special chars with spaces/data" \
  data='{"note": "tests special character handling"}' > /dev/null 2>&1

# Verify some secrets were created
echo "Verifying seed data..."
SECRET_COUNT=$(vault kv list -format=json secret/ 2>/dev/null | jq '. | length' || echo "0")
echo "Created $SECRET_COUNT top-level secret paths"

echo "=== Vault seeding completed successfully ==="
exit 0
