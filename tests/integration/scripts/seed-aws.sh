#!/bin/sh
# Seed AWS LocalStack with test fixtures
# This script runs inside the aws-seeder container

set -e

echo "=== Starting AWS LocalStack seeding ==="

# Wait for LocalStack to be fully ready
until aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager list-secrets > /dev/null 2>&1; do
  echo "Waiting for LocalStack to be ready..."
  sleep 2
done

echo "LocalStack is ready, proceeding with seeding..."

# Create S3 bucket for merge store testing
echo "Creating S3 bucket for merge store..."
aws --endpoint-url="$AWS_ENDPOINT_URL" s3 mb s3://merged-secrets || echo "Bucket already exists"

# Create some initial secrets to test sync phase
echo "Creating initial AWS Secrets Manager secrets..."

# Test secret 1: Simple key-value
aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager create-secret \
  --name "/test/simple-secret" \
  --description "Simple test secret" \
  --secret-string '{"key": "initial-value", "created_by": "seed-script"}' \
  > /dev/null 2>&1 || echo "Secret already exists: /test/simple-secret"

# Test secret 2: Database credentials pattern
aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager create-secret \
  --name "/test/db-credentials" \
  --description "Database credentials test" \
  --secret-string '{"username": "test_user", "password": "initial_pass", "host": "localhost"}' \
  > /dev/null 2>&1 || echo "Secret already exists: /test/db-credentials"

# Test secret 3: API keys pattern
aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager create-secret \
  --name "/test/api-keys" \
  --description "API keys test" \
  --secret-string '{"api_key": "test_key_12345", "api_secret": "test_secret_67890"}' \
  > /dev/null 2>&1 || echo "Secret already exists: /test/api-keys"

# Test secret 4: Empty secret for filtering tests
aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager create-secret \
  --name "/test/empty-secret" \
  --description "Empty secret for NoEmptySecrets testing" \
  --secret-string '{}' \
  > /dev/null 2>&1 || echo "Secret already exists: /test/empty-secret"

# Test secret 5: Path with leading slash
aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager create-secret \
  --name "test/no-leading-slash" \
  --description "Tests path normalization" \
  --secret-string '{"path_format": "no_leading_slash"}' \
  > /dev/null 2>&1 || echo "Secret already exists: test/no-leading-slash"

# Create secrets for pagination testing (>100 secrets)
echo "Creating secrets for pagination testing..."
for i in $(seq 1 25); do
  aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager create-secret \
    --name "/test/pagination/secret-$(printf "%03d" $i)" \
    --description "Pagination test secret $i" \
    --secret-string "{\"index\": $i, \"batch\": \"pagination-test\"}" \
    > /dev/null 2>&1 || true
done

# Verify secrets were created
echo "Verifying seed data..."
SECRET_COUNT=$(aws --endpoint-url="$AWS_ENDPOINT_URL" secretsmanager list-secrets \
  --query 'length(SecretList)' --output text 2>/dev/null || echo "0")
echo "Created/verified $SECRET_COUNT AWS Secrets Manager secrets"

# List S3 buckets to verify
BUCKET_COUNT=$(aws --endpoint-url="$AWS_ENDPOINT_URL" s3 ls | wc -l)
echo "S3 buckets available: $BUCKET_COUNT"

echo "=== AWS LocalStack seeding completed successfully ==="
exit 0
