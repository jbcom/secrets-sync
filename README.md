<div align="center">

# SecretSync

**Enterprise-Grade Secret Synchronization Pipeline**

[![⭐ Star on GitHub](https://img.shields.io/github/stars/jbcom/secrets-sync?style=social)](https://github.com/jbcom/secrets-sync/stargazers)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release](https://img.shields.io/github/release/jbcom/secrets-sync.svg)](https://github.com/jbcom/secrets-sync/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/jbcom/secrets-sync)](https://goreportcard.com/report/github.com/jbcom/secrets-sync)
[![Python Binding](https://img.shields.io/badge/python-secrets__sync-blue.svg)](./docs/PYTHON_BINDINGS.md)

[Quick Start](#quick-start) • [Repo Docs](./docs/) • [Python Integration](#python-integration) • [Examples](./examples/) • [GitHub Action](./docs/GITHUB_ACTIONS.md)

</div>

---

SecretSync provides **fully automated, enterprise-grade secret synchronization** across multiple cloud providers and secret stores. Built for scale with a **two-phase pipeline architecture** (merge → sync), it supports inheritance, dynamic target discovery, and CI/CD-friendly diff reporting.

## 🏢 Independent Go Runtime, Python Facade Above It

SecretSync is an independent [jbcom/secrets-sync](https://github.com/jbcom/secrets-sync) repository and MIT-licensed release artifact for secret synchronization workflows.

**🐍 Python Integration**: this repository owns the gopy binding source and
publishes the `secrets-sync-python-binding` distribution, imported as
`secrets_sync`. Downstream packages such as vendor-fabric can wrap it with
credential handoff, provider coordination, redaction, and Extended Data
composition.

**🚀 Perfect for:** Multi-account AWS environments, Kubernetes deployments, CI/CD pipelines, and enterprise secret management at scale.

## 🤔 Why SecretSync?

| Feature | SecretSync | Alternatives |
|---------|------------|--------------|
| **Two-Phase Pipeline** | ✅ Merge → Sync with inheritance | ❌ Simple 1:1 sync only |
| **AWS Organizations** | ✅ Dynamic discovery with tag filtering | ❌ Manual account management |
| **Secret Versioning** | ✅ Complete audit trail with rollback | ❌ No version tracking |
| **Enhanced Diff** | ✅ Side-by-side with intelligent masking | ❌ Basic text diff |
| **Enterprise Scale** | ✅ 1000+ accounts, circuit breakers | ❌ Limited scalability |
| **CI/CD Integration** | ✅ GitHub Action + exit codes | ❌ Manual scripting required |

## ✨ Key Features

### 🔍 **Advanced Discovery**
- **AWS Organizations Integration**: Discover accounts with tag filtering, wildcards, and OU-based selection
- **AWS Identity Center**: Permission set discovery and account assignment mapping
- **Smart Caching**: Multi-level caching for optimal performance at scale

### 📚 **Secret Versioning**
- **Complete Audit Trail**: Track every secret change with metadata
- **S3-Based Storage**: Reliable, scalable version history
- **Rollback Capability**: CLI support for version rollback
- **Retention Policies**: Configurable cleanup of old versions

### 🎨 **Enhanced Diff Output**
- **Side-by-Side Comparison**: Visual diff with aligned columns and color coding
- **Intelligent Masking**: Automatic detection and masking of sensitive values
- **Multiple Formats**: Human, JSON, GitHub Actions, and compact outputs
- **Rich Statistics**: Detailed change counts, sizes, and timing

### 🛡️ **Enterprise Reliability**
- **Circuit Breakers**: Automatic failure detection and recovery
- **Prometheus Metrics**: Production-ready observability with `/metrics` endpoint
- **Request Tracking**: Unique request IDs and duration tracking
- **Race-Free Operations**: Thread-safe with comprehensive testing

### 🏗️ **Pipeline Architecture**
- **Two-Phase Design**: Merge → Sync for complex inheritance scenarios
- **DeepMerge Support**: List append, dict merge, scalar override
- **Target Inheritance**: Hierarchical configuration with circular dependency detection
- **Dynamic Discovery**: AWS Organizations, Identity Center, and fuzzy matching

## Attribution

SecretSync originated as a fork of [robertlestak/vault-secret-sync](https://github.com/robertlestak/vault-secret-sync) (MIT License). We thank **Robert Lestak** for creating the original codebase.

**SecretSync is an independent product** with its own roadmap and development direction. It has been substantially rewritten with:
- Two-phase pipeline architecture (merge → sync)
- S3 merge store support  
- Dynamic target discovery (AWS Organizations, Identity Center)
- Comprehensive diff/dry-run system with CI/CD integration
- DeepMerge semantics for secret aggregation
- Kubernetes CronJob, `CredentialSynchronization` controller, and Helm
  deployment paths

## Supported Secret Stores

| Store | Source | Sync Target | Merge Store |
|-------|--------|-------------|-------------|
| HashiCorp Vault (KV2) | ✅ | ❌ | ✅ |
| AWS Secrets Manager | ✅ | ✅ | ❌ |
| AWS S3 | ❌ | ❌ | ✅ |
| AWS Organizations | Discovery | ❌ | ❌ |
| AWS Identity Center | Discovery | ❌ | ❌ |

## Two-Phase Pipeline Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    MERGE PHASE (Optional)                        │
│  Source1 ──┐                                                     │
│  Source2 ──┼──▶ Merge Store (Vault/S3) ──▶ Aggregated Secrets   │
│  Source3 ──┘    (deepmerge, inheritance)                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        SYNC PHASE                                │
│  Merge Store ──┬──▶ AWS Account 1 (via STS AssumeRole)          │
│  (or Source)   ├──▶ AWS Account 2                                │
│                └──▶ AWS Account 3                                │
└─────────────────────────────────────────────────────────────────┘
```

See [Two-Phase Architecture](./docs/TWO_PHASE_ARCHITECTURE.md) for detailed documentation.

## Quick Start

### Installation

```bash
# Go install
go install github.com/jbcom/secrets-sync/cmd/secrets-sync@latest

# Or build from a local checkout
brew install just # macOS; use apt/dnf/pacman equivalent on Linux
git clone https://github.com/jbcom/secrets-sync.git
cd secrets-sync
just build
```

## Python Integration

SecretSync owns the gopy binding contract for Python consumers.

- PyPI distribution: `secrets-sync-python-binding`
- Python import/module: `secrets_sync`
- Binding source: `python/secrets_sync/secrets_sync.go`

The repo-owned application surface is the `secrets_sync` binding:

```bash
pip install secrets-sync-python-binding
```

Downstream facades should consume this binding rather than reimplementing the
merge/sync engine. They can either delegate authentication to `secrets-sync` or
own the provider handshake and pass
authenticated session material through `ProviderSession`.

Direct binding consumers can install the generated wheel:

```bash
pip install secrets-sync-python-binding
```

```python
import secrets_sync

opts = secrets_sync.DefaultSyncOptions()
validation = secrets_sync.ValidateConfig("pipeline.yaml")
if not validation.Valid:
    raise RuntimeError(validation.ErrorMessage)

result = secrets_sync.RunPipeline("pipeline.yaml", opts)
```

### AI Agent Integration

Agent framework wrappers belong in `agentic-fabric`. That layer should consume
vendor-fabric capabilities rather than adding LangChain, CrewAI, LangGraph,
Strands, or MCP adapters here.

### Basic Usage

```bash
# Validate configuration
secrets-sync validate --config pipeline.yaml

# Dry run with enhanced diff output
secrets-sync pipeline --config pipeline.yaml --dry-run --output side-by-side

# Full pipeline execution with metrics
secrets-sync pipeline --config pipeline.yaml --metrics-port 9090

# Stable machine-readable CLI contract
secrets-sync pipeline --config pipeline.yaml --output json

# CI/CD mode (exit codes: 0=no changes, 1=changes, 2=errors)
secrets-sync pipeline --config pipeline.yaml --dry-run --diff --output json --exit-code

# Inspect dependency order
secrets-sync graph --config pipeline.yaml
```

### Example Configuration

```yaml
vault:
  address: https://vault.example.com/
  namespace: admin
  auth:
    approle:
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

aws:
  region: us-east-1
  execution_context:
    type: delegated_admin
    account_id: "123456789012"
  control_tower:
    enabled: true
    execution_role:
      name: AWSControlTowerExecution

merge_store:
  s3:
    bucket: company-secrets-sync-merge-store
    prefix: merged/
    versioning:
      enabled: true
      retain_versions: 90

sources:
  api-keys:
    vault:
      mount: secret
      paths: [api-keys]
  database:
    vault:
      mount: secret
      paths: [database]

targets:
  staging:
    account_id: "111111111111"
    imports: [api-keys, database]

  production:
    account_id: "222222222222"
    imports: [staging, production-overrides]

dynamic_targets:
  production-accounts:
    discovery:
      organizations:
        ous: ["ou-production-12345"]
        tag_filters:
          - key: Environment
            values: ["production"]
            operator: equals
        recursive: true
    imports: [production]
    region: us-east-1
    secret_prefix: platform/
```

## GitHub Actions

SecretSync is available as a GitHub Action for seamless CI/CD integration:

```yaml
- name: Sync Secrets
  uses: jbcom/secrets-sync@vX.Y.Z
  with:
    config: config.yaml
    dry-run: 'false'
    output-format: 'github'
  env:
    VAULT_ROLE_ID: ${{ secrets.VAULT_ROLE_ID }}
    VAULT_SECRET_ID: ${{ secrets.VAULT_SECRET_ID }}
```

**Key Features:**
- 🔒 Native OIDC support for AWS authentication
- 📊 GitHub-native diff annotations in PRs
- 🎯 Exit codes for CI/CD control flow
- 🔄 Automatic Docker multi-arch builds
- ⚡ Zero configuration needed beyond config file

**Quick Start:**
1. Add `config.yaml` to your repository
2. Configure AWS OIDC and Vault secrets
3. Use the action in your workflow

See [GitHub Actions documentation](./docs/GITHUB_ACTIONS.md) for complete usage guide and examples.

## CI/CD Integration (CLI)

### GitHub Actions (CLI)

```yaml
- name: Validate secrets pipeline
  run: |
    secrets-sync pipeline --config pipeline.yaml --dry-run --output github --exit-code
  
- name: Apply secrets (on merge to main)
  if: github.ref == 'refs/heads/main'
  run: |
    secrets-sync pipeline --config pipeline.yaml
```

### Output Formats

| Format | Use Case | Features |
|--------|----------|----------|
| `human` | Interactive terminal output | Color coding, readable layout |
| `side-by-side` | Visual comparison | Aligned columns, intelligent masking |
| `json` | Machine parsing, logging | Structured data with metadata |
| `github` | GitHub Actions annotations | PR comments, file annotations |
| `compact` | One-line CI status | Minimal output for scripts |

**Value Masking**: Sensitive values are automatically masked by default. Use `--show-values` flag to display actual values (use with caution in CI/CD).

## 📚 Documentation

### Getting Started
- [🚀 Getting Started Guide](./docs/GETTING_STARTED.md) - Step-by-step setup tutorial
- [❓ FAQ](./docs/FAQ.md) - Frequently asked questions
- [📋 Examples](./examples/) - Complete configuration examples

### Core Documentation
- [🏗️ Architecture Overview](./docs/ARCHITECTURE.md) - System design and components
- [🔎 Architecture Audit](./docs/ARCHITECTURE_AUDIT.md) - Current implementation and release-contract status
- [🔄 Two-Phase Pipeline](./docs/TWO_PHASE_ARCHITECTURE.md) - Merge → Sync architecture
- [⚙️ Pipeline Configuration](./docs/PIPELINE.md) - Configuration reference
- [🚀 Deployment Guide](./docs/DEPLOYMENT.md) - Production deployment patterns

### Advanced Topics
- [🔒 Security Configuration](./docs/SECURITY.md) - Security best practices
- [📊 Observability](./docs/OBSERVABILITY.md) - Monitoring and metrics
- [🎯 GitHub Actions](./docs/GITHUB_ACTIONS.md) - CI/CD integration guide
- [📖 Usage Reference](./docs/USAGE.md) - Complete CLI reference

### Community
- [🗺️ Roadmap](./docs/ROADMAP.md) - Future development plans
- [🤝 Contributing](./CONTRIBUTING.md) - How to contribute
- [🛡️ Security Policy](./SECURITY.md) - Security reporting
- [📜 Code of Conduct](./CODE_OF_CONDUCT.md) - Community guidelines

## Kubernetes

Run SecretSync as a scheduled pipeline runner or install the
`CredentialSynchronization` controller. See
[docs/DEPLOYMENT.md](./docs/DEPLOYMENT.md) for complete CronJob and controller
examples.

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: secrets-sync
spec:
  schedule: "*/30 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
            - name: secrets-sync
              image: ghcr.io/jbcom/secrets-sync:v2.2.0
              args: ["pipeline", "--config", "/config/config.yaml", "--diff", "--output", "json"]
```

## Docker

```bash
# Run with config file
docker run -v $(pwd)/config.yaml:/config.yaml \
  ghcr.io/jbcom/secrets-sync:v2.2.0 pipeline --config /config.yaml

# Release image: ghcr.io/jbcom/secrets-sync:v2.2.0
```

The published image is a Google Distroless static runtime containing both
`secrets-sync` and `secrets-sync-controller`.

## Lambda And Kubernetes API

The compiled Go runtime also ships as:

- A Lambda archive built from `cmd/secrets-sync-lambda`, published by GoReleaser.
- A Kubernetes CRD schema at
  `deploy/crds/secrets-sync.jbcom.dev_credentialsynchronizations.yaml`.
- A Kubernetes controller at `cmd/secrets-sync-controller` with direct manifests
  under `deploy/controller`.
- A Helm chart at `deploy/charts/secrets-sync` for direct CronJob or controller
  installs.

The CRD kind is `CredentialSynchronization`. The controller reconciles those
resources into managed CronJobs that run the same `secrets-sync pipeline`
command used by the CLI, Action, Lambda, and Docker surfaces.

## Observability

SecretSync exposes Prometheus metrics for production monitoring and debugging.

### Enabling Metrics

```bash
# Enable metrics server on port 9090
secrets-sync pipeline --config config.yaml --metrics-port 9090

# Custom address and port
secrets-sync pipeline --config config.yaml --metrics-addr 0.0.0.0 --metrics-port 9090
```

### Available Metrics

**Vault Metrics:**
- `secrets_sync_vault_api_call_duration_seconds` - Vault API call latency
- `secrets_sync_vault_secrets_listed_total` - Total secrets listed from Vault
- `secrets_sync_vault_traversal_depth` - BFS traversal depth reached
- `secrets_sync_vault_queue_size` - Current traversal queue size
- `secrets_sync_vault_errors_total` - Vault error count by operation/type

**AWS Metrics:**
- `secrets_sync_aws_api_call_duration_seconds` - AWS API call latency
- `secrets_sync_aws_pagination_pages` - Number of pagination pages processed
- `secrets_sync_aws_cache_hits_total` - Cache hit count
- `secrets_sync_aws_cache_misses_total` - Cache miss count
- `secrets_sync_aws_secrets_operations_total` - Secret operations (create/update/delete)

**Pipeline Metrics:**
- `secrets_sync_pipeline_execution_duration_seconds` - Pipeline phase duration
- `secrets_sync_pipeline_targets_processed_total` - Targets processed by phase
- `secrets_sync_pipeline_parallel_workers` - Active parallel workers
- `secrets_sync_pipeline_errors_total` - Pipeline error count

**S3 Metrics:**
- `secrets_sync_s3_operation_duration_seconds` - S3 operation latency
- `secrets_sync_s3_object_size_bytes` - S3 object sizes

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'secrets-sync'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/metrics'
```

### Health Check

The metrics server also exposes a `/health` endpoint:

```bash
curl http://localhost:9090/health
# Returns: OK
```

## Development

```bash
# Clone
git clone https://github.com/jbcom/secrets-sync.git
cd secrets-sync

# Build
go build ./...

# Vulnerability scan
go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...

# Unit tests
go test ./...

# Integration tests (requires Docker)
just test-integration-docker

# Lint
golangci-lint run
```

### Integration Testing

SecretSync includes comprehensive integration tests that validate the complete pipeline with real Vault and AWS Secrets Manager instances (via LocalStack).

**Quick Start:**
```bash
# Run complete integration test suite
just test-integration-docker
```

This command:
- Starts Vault and LocalStack in Docker containers
- Seeds test data automatically
- Runs all integration tests
- Cleans up containers

**Manual Testing:**
```bash
# Start test environment
just test-env-up

# Export environment variables (shown in output)
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=test-root-token
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test

# Run tests
go test -v -tags=integration ./tests/integration/...

# Cleanup
just test-env-down
```

For detailed documentation, see [tests/integration/README.md](./tests/integration/README.md).

## 🌟 Community & Support

### Getting Help
- **📚 Documentation**: Start with the repo-local [docs folder](./docs/)
- **🐛 GitHub Issues**: Questions, bug reports, and feature requests
- **🔒 Security**: Private security vulnerability reporting

### Contributing
We welcome contributions! See our [Contributing Guide](./CONTRIBUTING.md) for:
- 🛠️ Development setup
- 📝 Code style guidelines  
- 🧪 Testing requirements
- 📋 Pull request process

### Community
- **⭐ Star the repo** to show your support
- **🐦 Follow updates** on GitHub
- **📢 Share** your success stories
- **🤝 Contribute** code, docs, or feedback

## 📄 License

[MIT License](./LICENSE) - Free for commercial and personal use

## 🙏 Attribution

SecretSync originated as a fork of [vault-secret-sync](https://github.com/robertlestak/vault-secret-sync) by **Robert Lestak**. We thank Robert for creating the original foundation.

SecretSync has evolved into an independent project with its own architecture, features, and roadmap, while maintaining the same MIT license and open-source spirit.

**Current Maintainer**: [jbcom](https://github.com/jbcom)
