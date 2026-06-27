# SecretSync Roadmap

This roadmap outlines the planned development direction for SecretSync. It's a
living document that evolves based on community feedback and changing
requirements.

## Current Status

SecretSync v2.3.1 is the current release. The shipped surface includes:

- Go CLI with `pipeline`, `validate`, `graph`, `context`, `migrate`, and
  `version` subcommands
- Structured JSON output with CI-friendly exit codes
- GitHub Action (`action.yml`) with `--output github` diff reporting
- GHCR distroless image (`ghcr.io/jbcom/secrets-sync`)
- Helm chart (CronJob runner + optional controller)
- Kubernetes `CredentialSynchronization` CRD + controller
- AWS Lambda entrypoint (inline, S3-hosted, or packaged config)
- `secrets_sync` gopy binding published as `secrets-sync-python-binding`
- Vault KV2 source + merge store
- AWS Secrets Manager source + sync target
- S3 merge store with secret versioning
- AWS Organizations + Identity Center dynamic target discovery
- Prometheus metrics (`/metrics` + `/health`)
- Two-phase merge → sync pipeline with inheritance, diff, and dry-run

### Repository scope boundary

`secrets-sync` owns the canonical Go runtime, CLI, pipeline semantics, GHCR
image, GitHub Action, Helm chart, Kubernetes CRD/controller, Lambda
entrypoint, documentation, and gopy binding. The following belong in
downstream repos (`vendor-fabric`, `agentic-fabric`) and are **not** planned
for this repository:

- Agent framework adapters and runtime wrappers
- Web dashboard / management UI
- Mobile apps
- REST/GraphQL API servers
- Plugin architecture (runtime plugins belong in `agentic-fabric`; provider
  plugins belong in `vendor-fabric`)

## Upcoming Releases

### v2.4.0 - Additional Secret Stores

**Theme**: Expand the provider matrix beyond Vault and AWS.

#### Azure Key Vault
- Full read/write support as both source and sync target
- Azure AD authentication (service principal, managed identity, workload
  identity federation)
- Cross-tenant sync via Azure RBAC role assignments

#### Google Cloud Secret Manager
- Full read/write support as both source and sync target
- Service account and workload identity authentication
- Project-level secret isolation

#### Kubernetes Secrets
- Direct sync to Kubernetes clusters as a target backend
- Namespace-scoped writes with RBAC
- Support for `Dockerconfigjson`, `TLS`, and Opaque secret types

#### Generic HTTP Store
- Webhook-based integration for custom secret stores
- Configurable auth (bearer token, mTLS, custom headers)
- Retry and circuit-breaker integration

### v2.5.0 - Observability & Tracing

**Theme**: Production-grade observability for complex pipelines.

#### OpenTelemetry Distributed Tracing
- Trace correlation across Vault, AWS, and pipeline operations
- Span attributes for target, source, operation, and phase
- Jaeger, Zipkin, and OTLP exporter support
- Configurable sampling rates

#### Enhanced Metrics
- User-defined custom metrics via config
- Pre-built Prometheus alerting rules
- Official Grafana dashboard templates
- Advanced health check endpoints (dependency probes)

#### Performance Optimizations
- Discovery result caching (Organizations, Identity Center)
- Configurable AWS retry/backoff settings
- Bulk batch operations for large secret sets
- Concurrent source reads with configurable limits

### v2.6.0 - Enterprise Governance

**Theme**: Security, compliance, and operational safety.

#### Policy as Code
- Declarative sync policies in config (allow/deny rules per target/source)
- Policy validation during `secrets-sync validate`
- Pre-sync policy enforcement with dry-run preview

#### Audit Logging
- Structured audit log for every secret read/write/delete
- Tamper-evident log chaining (hash chain)
- Configurable log destinations (file, CloudWatch, S3)

#### Client-Side Encryption
- Optional encryption-at-rest for the S3 merge store
- KMS-managed or user-supplied encryption keys
- Zero-knowledge mode: secrets encrypted before reaching the merge store

#### Rollback Automation
- Automatic rollback on sync failure detection
- Version-aware rollback using the S3 version store
- Configurable rollback windows and safety checks

### v2.7.0 - Advanced Workflows

**Theme**: Flexible pipeline orchestration.

#### Conditional Sync
- Condition-based sync gating (environment, tag, time window)
- Configurable sync triggers and filters
- Skip rules for specific source/target combinations

#### Per-Target Scheduling
- Cron-like scheduling per target via the Kubernetes controller
- Staggered sync windows to avoid provider rate limits
- Time-zone-aware schedule evaluation

#### Multi-Instance Coordination
- Leader election for multi-replica controller deployments
- Distributed locking via S3 conditional writes
- Work partitioning across concurrent pipeline instances

## Future Considerations (v3.0+)

### Edge Computing
- Edge deployment patterns for global secret distribution
- Regional merge stores with cross-region replication

### Advanced Security
- Post-quantum cryptographic algorithms for encryption-at-rest
- Zero-trust security model for controller-to-provider communication
- Cross-organization federated identity for multi-org discovery

### Scale
- Horizontal scaling with Redis/Memcached caching layer
- Event-driven async processing with message queues
- Additional serverless targets (Azure Functions) beyond AWS Lambda

## Community Priorities

Based on community feedback, we're prioritizing:

1. **Azure Key Vault Support** (high demand from enterprise users) — v2.4.0
2. **Enhanced Kubernetes Integration** (DevOps community priority) — v2.4.0
3. **Distributed Tracing** (observability gap) — v2.5.0
4. **Policy as Code** (security team requirements) — v2.6.0

## How to Influence the Roadmap

### Community Input
- **GitHub Issues**: Share your use cases and requirements
- **Feature Requests**: Create detailed feature requests with business
  justification
- **User Surveys**: Participate in periodic user surveys

### Contributions
- **Code Contributions**: Implement features you need
- **Documentation**: Improve docs and examples
- **Testing**: Help test beta features
- **Feedback**: Provide feedback on proposed features

### Enterprise Partnerships
- **Design Partnerships**: Work with us to design enterprise features
- **Beta Testing**: Early access to enterprise features
- **Sponsored Development**: Fund development of specific features

## Release Schedule

### Regular Releases
- **Major Releases**: Every 6 months (x.0.0)
- **Minor Releases**: Every 2 months (x.y.0)
- **Patch Releases**: As needed (x.y.z)
- **Security Releases**: Immediate (x.y.z)

### Beta Program
- **Alpha Releases**: 4 weeks before minor releases
- **Beta Releases**: 2 weeks before minor releases
- **Release Candidates**: 1 week before major releases

## Clean Break Policy

- **Configuration**: Prefer one current shape over compatibility aliases.
- **API**: Breaking changes are acceptable when they keep the implementation
  honest.
- **CLI**: Removed flags and fields should fail loudly with clear replacement
  guidance.
- **Migration Docs**: Document replacement configuration rather than carrying
  shims.

## Success Metrics

### Technical Metrics
- **Performance**: <100ms p95 latency for secret operations
- **Reliability**: 99.9% uptime for production deployments
- **Scale**: Support for 10,000+ secrets and 1,000+ targets
- **Security**: Zero critical security vulnerabilities

### Community Metrics
- **Adoption**: 10,000+ GitHub stars by end of 2026
- **Contributors**: 100+ community contributors
- **Deployments**: 1,000+ production deployments

---

## Questions?

- **Roadmap Feedback**: [GitHub Issues](https://github.com/jbcom/secrets-sync/issues)
- **Feature Requests**: [GitHub Issues](https://github.com/jbcom/secrets-sync/issues)
- **Enterprise Inquiries**: Contact us through GitHub Issues

**This roadmap is a living document and will evolve based on community needs
and feedback.**
