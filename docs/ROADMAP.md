# SecretSync Roadmap

This roadmap outlines the planned development direction for SecretSync. It's a
living document that evolves based on community feedback and changing
requirements.

## Current Status

The shipped surface includes:

- Go CLI with `pipeline`, `validate`, `graph`, `context`, `migrate`, and
  `version` subcommands
- Structured JSON output with CI-friendly exit codes
- GitHub Action (`action.yml`) with `--output github` diff reporting
- GHCR distroless image (`ghcr.io/jbcom/secrets-sync`)
- Helm chart (CronJob runner + optional controller)
- Kubernetes `CredentialSynchronization` CRD + controller, with per-target
  timezone and stagger-window scheduling
- AWS Lambda and Azure Functions serverless entrypoints (inline, S3-hosted, or
  packaged config) over a shared serverless core
- `secrets_sync` gopy binding published as `secrets-sync-python-binding`
- A driver-generic backend abstraction (formal source/target/merge-store
  interfaces + registry) underpinning all providers
- Sources and sync targets: Vault KV2, AWS Secrets Manager, Azure Key Vault,
  GCP Secret Manager, Kubernetes Secrets, and a generic HTTP/webhook store
- S3 and Vault merge stores with secret versioning and cross-region replication
- AWS Organizations + Identity Center dynamic target discovery
- Prometheus metrics (`/metrics`), dependency-probe health/readiness endpoints,
  user-defined custom metrics, and a shipped Grafana dashboard + alerting rules
- OpenTelemetry distributed tracing (OTLP/Zipkin/stdout exporters)
- Governance: declarative allow/deny sync policies, tamper-evident hash-chained
  audit logging (file/S3/CloudWatch), client-side merge-store encryption
  (KMS / static key / post-quantum ML-KEM-768), and automatic rollback on
  sync failure
- Workflow controls: conditional sync (env/tag/time-window gating) and
  multi-replica coordination (S3 lease lock leader election + work partitioning)
- A pluggable cache layer (in-memory TTL; Redis/Memcached-ready interface)
- Two-phase merge → sync pipeline with inheritance, diff, dry-run, and
  configurable concurrency and AWS retry/backoff

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

## Recently shipped

The major roadmap themes below have all landed and are reflected in **Current
Status** above. Release numbers are assigned by release automation, not encoded
here.

### Additional secret stores
Azure Key Vault, GCP Secret Manager, Kubernetes Secrets, and a generic
HTTP/webhook store all work as sources and sync targets, alongside the original
Vault and AWS Secrets Manager backends — routed through a common backend
abstraction (formal interfaces + registry) so the pipeline orchestration is
provider-agnostic.

### Observability & tracing
OpenTelemetry distributed tracing (OTLP/Zipkin/stdout exporters, configurable
sampling, target/source/operation/phase span attributes); user-defined custom
metrics; dependency-probe health/readiness endpoints; a shipped Grafana
dashboard and Prometheus alerting rules; discovery caching, configurable AWS
retry/backoff, and concurrent source reads.

### Enterprise governance
Declarative allow/deny sync policies validated at `validate` time and enforced
pre-sync; tamper-evident hash-chained audit logging to file, S3, or CloudWatch;
client-side merge-store encryption (KMS, user-supplied key, or post-quantum
ML-KEM-768) in zero-knowledge mode; and automatic rollback on sync failure.

### Advanced workflows
Conditional sync gating (environment, tag, time window); per-target controller
scheduling with stagger windows and timezone awareness; and multi-replica
coordination via an S3 lease-lock leader election with work partitioning.

### Scale & distribution
Cross-region merge-store replication; a pluggable cache layer (in-memory TTL,
Redis/Memcached-ready); and an Azure Functions serverless entrypoint sharing a
common serverless core with AWS Lambda.

## Forward look

The shipped surface covers the planned enterprise feature set. Future direction
is driven by community needs — see below. Candidate areas include additional
provider backends, deeper edge/global-distribution patterns, and further
serverless runtimes.

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
