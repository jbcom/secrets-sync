# Changelog

> **Note:** Historical links reference pre-monorepo repositories.

All notable changes to SecretSync will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2025-12-09

### Added - v1.2.0 Advanced Features
- **AWS Organizations Discovery Enhancement**
  - Multiple tag filters with wildcard support (`*`, `?`) and contains matching
  - Configurable AND/OR logic for tag combinations
  - Multiple organizational unit (OU) filtering with nested traversal
  - Account status filtering (exclude SUSPENDED/CLOSED accounts)
  - In-memory caching with TTL for discovered accounts (1 hour default)
- **AWS Identity Center Integration**
  - Permission set discovery with ARN to name mapping
  - Account assignment mapping with principal tracking
  - Cross-region support with automatic instance ARN discovery
  - Caching for assignments (30 minute TTL)
- **Secret Versioning System**
  - Version tracking in diff engine with metadata storage
  - S3-based version storage with retention policies
  - Version rollback capability via CLI `--version` flag
  - Version transitions displayed in diff output (v1 → v2)
- **Enhanced Diff Output**
  - Side-by-side comparison format with aligned columns
  - Intelligent value masking for sensitive patterns (API keys, passwords, tokens)
  - Multiple output formats: human, JSON, GitHub Actions, compact
  - Summary statistics with added/modified/deleted counts and timing

### Added - v1.1.0 Observability & Reliability
- **Prometheus Metrics Integration**
  - `/metrics` endpoint with comprehensive metrics for Vault, AWS, Pipeline, S3
  - `/health` endpoint for health checks
  - CLI flags: `--metrics-port` and `--metrics-addr`
  - Automatic metrics server startup when port configured
- **Circuit Breaker Pattern**
  - Independent circuit breakers for Vault and AWS clients
  - Configurable failure thresholds and recovery timeouts
  - State transition logging with observability integration
- **Enhanced Error Context**
  - Request ID tracking throughout pipeline execution
  - Duration tracking for all operations
  - Structured error messages with contextual information
- **Queue Compaction Configuration**
  - Configurable thresholds with adaptive defaults
  - Formula: `min(1000, maxSecretsPerMount/100)`
  - YAML configuration support
- **Race Condition Prevention**
  - Proper mutex protection for concurrent map access
  - All tests pass with `-race` detector
- **Docker Image Version Pinning**
  - All test images pinned to specific versions for reproducible builds

### Added - v1.0 Core Features
- **SecretSync 1.0 Release** - Complete rebranding and architecture overhaul
- Recursive Vault KV2 listing with BFS traversal
- Target inheritance with circular dependency detection
- Deepmerge support (list append, dict merge, scalar override)
- AWS Organizations dynamic account discovery
- Fuzzy name matching for AWS accounts
- S3 merge store support
- TTL-based caching for AWS ListSecrets
- Enhanced path validation (directory traversal, null byte injection protection)
- LogicalClient interface for testability
- Comprehensive test suite (150+ test functions)

### Changed
- **PROJECT RENAME**: vault-secret-sync → SecretSync
- CLI renamed from `vss` to `secretsync`
- Docker images published to `docker.io/extended-data-library/secretssync`
- Helm charts published to `oci://registry-1.docker.io/extended-data-library/secretssync`
- Simplified pipeline architecture (removed legacy operator complexity)
- Environment variable prefix changed from `VSS_` to `SECRETSYNC_`

### Removed
- Legacy Kubernetes operator architecture (~13k lines)
- Backend packages (kube, file)
- Queue packages (redis, nats, sqs, memory)
- Notification packages (webhook, slack, email)
- GCP/GitHub/Doppler/HTTP store implementations
- Event processing system

### Security
- Race condition fixes in AWS client with mutex protection
- Cache invalidation on writes to prevent stale data
- Path traversal attack prevention
- Type-safe Vault API response parsing
- Safe type assertions throughout codebase
- Intelligent value masking in diff output to prevent credential exposure

### Fixed
- Cache invalidation after WriteSecret and DeleteSecret operations
- Structured logging consistency
- Error context in vault traversal with depth and count info
- Critical lint errors (copylocks, staticcheck QF1003)
- Integration test coverage in CI/CD pipeline

---

## Ownership & Attribution

### Current Maintainer
- **Organization**: jbcom
- **Repository**: [extended-data-library/secretssync](https://github.com/extended-data-library/secretssync)

### Original Source
- **Author**: Robert Lestak
- **Repository**: [robertlestak/vault-secret-sync](https://github.com/robertlestak/vault-secret-sync)
- **License**: MIT

### Fork Rationale

This project is a complete rebranding and reimplementation of vault-secret-sync,
focused on providing a streamlined, pipeline-based secret synchronization tool.

**Key differences from upstream:**
- Pipeline-driven architecture instead of Kubernetes operator
- Support for dynamic AWS Organizations discovery
- Enhanced merge strategies matching Python's deepmerge
- Simplified configuration and deployment
- GitHub Marketplace Action support

### License

MIT License - see [LICENSE](LICENSE) for details.

Original work Copyright (c) Robert Lestak
Modified work Copyright (c) 2025 jbcom
