# Integration Testing

This directory contains end-to-end integration tests for SecretSync that validate the complete merge+sync pipeline with real Vault and AWS Secrets Manager instances (via LocalStack).

## Test Infrastructure

### Docker Compose Stack

The `docker-compose.test.yml` file in the project root defines a complete integration test environment with:

1. **LocalStack** - AWS Secrets Manager emulation
   - Runs on port 4566
   - Includes S3 for merge store testing
   - Automatically seeded with test data

2. **Vault** - HashiCorp Vault in dev mode
   - Runs on port 8200
   - Root token: `test-root-token`
   - KV v2 secrets engine enabled
   - Automatically seeded with test fixtures

3. **Seeders** - Initialization containers
   - `vault-seeder`: Populates Vault with test secrets from `testdata/secrets_seed.json`
   - `aws-seeder`: Populates LocalStack with initial AWS secrets

4. **Test Runner** - Executes Go integration tests
   - Waits for seeders to complete
   - Runs with proper environment variables
   - Uses Go 1.25.5

### Test Data

- **testdata/secrets_seed.json** - Comprehensive test fixtures covering:
  - Multi-source merge scenarios (analytics, data-engineers, shared)
  - Deepmerge patterns (scalar override, list append, dict merge)
  - Nested path hierarchies (tests recursive listing)
  - Expected merge results for validation

- **testdata/accounts.json** - AWS Organizations test data:
  - Multiple AWS accounts with varying name formats
  - Tests fuzzy account name matching
  - Organizational units structure

### Seed Scripts

- **scripts/seed-vault.sh** - Vault initialization
  - Enables KV v2 secrets engine
  - Creates secrets from JSON fixtures
  - Adds nested paths for recursive listing tests
  - Creates edge cases (trailing slashes, special characters)

- **scripts/seed-aws.sh** - LocalStack initialization
  - Creates S3 bucket for merge store
  - Seeds initial AWS Secrets Manager secrets
  - Creates 25+ secrets for pagination testing
  - Tests path normalization patterns

## Running Tests

### Quick Start (Recommended)

```bash
# Run complete test suite with docker-compose
make test-integration-docker
```

This command:
1. Tears down any existing containers
2. Builds and starts all services
3. Seeds test data automatically
4. Runs integration tests
5. Cleans up containers

### Manual Testing

```bash
# Start the test environment
make test-env-up

# Export environment variables (shown in output)
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=test-root-token
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_REGION=us-east-1

# Run tests
go test -v -tags=integration ./tests/integration/...

# Cleanup
make test-env-down
```

### CI/CD Testing

The GitHub Actions CI workflow uses the docker-compose stack for integration tests:

```yaml
- name: Run integration tests
  run: make test-integration-docker
```

## Test Coverage

### Current Tests

1. **pipeline_test.go** - Core merge+sync pipeline
   - Seeds Vault with test secrets
   - Validates recursive listing
   - Tests deepmerge semantics
   - Syncs to AWS Secrets Manager
   - Validates final output

2. **fsc_compatibility_test.go** - FlipsideCrypto compatibility
   - Multi-tier target inheritance (Staging → Production → Demo)
   - AWS Organizations account discovery
   - Fuzzy account name matching
   - S3 and Vault merge stores
   - Complex merge patterns

### Test Patterns Covered

✅ **Vault Operations**
- Recursive KV2 listing with nested paths
- Path normalization (trailing slashes, special chars)
- Metadata path conversion
- Type-safe response parsing

✅ **Deepmerge Semantics**
- Scalar override (last wins)
- List append (preserves order, allows duplicates)
- Dictionary merge (recursive, last wins on conflicts)
- 3+ level deep nesting

✅ **AWS Operations**
- Secrets Manager CRUD operations
- Pagination (>100 secrets)
- Empty secret filtering
- Path format preservation (/foo vs foo)
- S3 merge store read/write

✅ **Target Inheritance**
- Single-level imports
- Multi-level chains (A → B → C)
- Source vs target detection
- GetSourcePath resolution

✅ **Edge Cases**
- Path with/without leading slashes
- Special characters in paths
- Empty secrets
- Missing secrets (404 handling)
- Concurrent operations

## Fixtures and Seed Data

### Vault Seed Structure

```
secret/
├── analytics/
│   ├── config/database
│   ├── config/api
│   ├── credentials/service_account
│   └── nested/deep/level1/level2/config
├── data-engineers/
│   ├── config/database
│   ├── config/tools
│   ├── credentials/snowflake
│   └── team/members
├── shared/
│   ├── config/common
│   ├── credentials/aws
│   └── certificates/root_ca
├── nested/
│   ├── level1/sublevel1/config
│   ├── level1/sublevel2/config
│   ├── level2/sublevel1/config
│   └── level2/sublevel2/config
└── edge-cases/
    ├── trailing-slash-test/data
    └── special chars with spaces/data
```

### AWS Seed Structure

```
AWS Secrets Manager:
├── /test/simple-secret
├── /test/db-credentials
├── /test/api-keys
├── /test/empty-secret
├── test/no-leading-slash
└── /test/pagination/
    ├── secret-001
    ├── secret-002
    └── ... (25 total)

S3 Buckets:
└── merged-secrets/
```

## Troubleshooting

### Services Not Healthy

If containers fail health checks:

```bash
# Check container logs
docker-compose -f docker-compose.test.yml logs localstack
docker-compose -f docker-compose.test.yml logs vault

# Restart with fresh state
docker-compose -f docker-compose.test.yml down -v
docker-compose -f docker-compose.test.yml up
```

### Seeding Failures

Check seeder container logs:

```bash
docker-compose -f docker-compose.test.yml logs vault-seeder
docker-compose -f docker-compose.test.yml logs aws-seeder
```

### Test Failures

Run tests with verbose output:

```bash
go test -v -timeout=10m -tags=integration ./tests/integration/...
```

Enable debug logging:

```bash
export VAULT_LOG_LEVEL=debug
export DEBUG=1
go test -v -tags=integration ./tests/integration/...
```

## Adding New Tests

1. **Add test fixtures** to `testdata/secrets_seed.json`
2. **Update seed scripts** if needed (scripts/seed-*.sh)
3. **Write test function** in appropriate *_test.go file
4. **Run locally** with `make test-integration-docker`
5. **Verify CI passes** in PR

## Performance

Typical test execution times:
- Docker compose startup: ~15-20 seconds
- Vault/AWS seeding: ~5-10 seconds
- Test execution: ~10-30 seconds
- **Total**: ~30-60 seconds

The docker-compose stack is designed for fast iteration with:
- Health checks for service readiness
- Parallel service startup
- Efficient seeding scripts
- Automatic cleanup

## References

- [LocalStack Documentation](https://docs.localstack.cloud/)
- [Vault Dev Mode](https://developer.hashicorp.com/vault/docs/concepts/dev-server)
- [AWS SDK for Go V2](https://aws.github.io/aws-sdk-go-v2/docs/)
- [testify/assert](https://pkg.go.dev/github.com/stretchr/testify/assert)
