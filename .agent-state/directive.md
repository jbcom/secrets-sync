# Continuous Work Directive — secrets-sync

**Status:** ACTIVE
**Owner:** Claude
**Mandate:** "go through and implement everything from the roadmap"

## Sequencing decisions (user-confirmed 2026-06-27)
- **Refactor backend abstraction FIRST**, then add providers v2.4→v2.7 in order.
- **One giant PR** for everything — single branch `feat/roadmap-providers-and-governance`,
  opened once at the very end. Layer milestones as forward commits.

## What CONTINUOUS means
1. Never stop for status reports the user didn't ask for.
2. Never stop for scope caution.
3. Never stop to summarize — git log is the summary.
4. Never stop for context pressure — task-batch + PreCompact handle it.
5. Never stop because a task feels big — pick the next atomic commit.
6. Only stop on: explicit user halt, red CI blocking, or genuine STOP_FAIL.

## Operating loop
while queue has [ ] items: implement → verify (`just test-unit`/`just build`) → commit
(one Conventional Commit per item) → dispatch reviewers (bg) → mark [x] → next.

## Forbidden phrases
"deferred" | "v2+" | "out of scope" | "future work" | "tracked separately" | "follow-up"
"TODO" | "FIXME" | "stub" | "placeholder" | "mock for now"

## Architecture baseline (from explorer map 2026-06-27)
- Implicit client contract (AWS+Vault both satisfy): `Init(ctx)`, `Driver() driver.DriverName`,
  `GetPath()`, `ListSecrets(ctx,path)([]string,error)`, `GetSecret(ctx,path)([]byte,error)`,
  `WriteSecret(ctx,meta,path,[]byte)([]byte,error)`, `DeleteSecret(ctx,path)error`, `Close()error`.
- Sources: union struct `Source{Vault,AWS}` (types.go:142). Targets: AWS-hardwired
  (`Target` has account_id/role_arn/region; `syncTarget` builds `aws.AwsClient` directly — sync.go:98).
- Merge store: Vault KV2 or S3 (`MergeStoreConfig` types.go:170; `S3MergeStore` s3_store.go).
- Pipeline client factories: runtime_clients.go `vaultClient()`/`awsClient()`.
- Fetch branch: fetch.go `fetchVaultSecrets`/`fetchAWSSecrets`. Sync: sync.go `syncTarget`.
- Discovery: discovery_service.go. Observability: pkg/observability/metrics.go (Prometheus only).
- Circuit breaker: pkg/circuitbreaker (sony/gobreaker). Wraps every API call.
- Docs guards (update both sides together): action_docs_test.go, dockerfile_test.go,
  docs_markdown_test.go, docs_security_test.go, docs_version_test.go, examples_config_test.go,
  helm_chart_test.go, kubernetes_crd_test.go, release_config_test.go, workflow_pinning_test.go.

## Queue — Roadmap implementation

### M0 Backend abstraction (prereq for all providers)
- [x] M0.1 Define `SecretBackend` source/target interfaces + `MergeStore` interface in pkg/driver (formalize the implicit contract; AWS+Vault already return Driver()).
- [x] M0.2 Add a backend registry/factory: `DriverName → constructor`, keyed off config. Replace `DriverIsSupported` static list with registry membership.
- [x] M0.3 Make `Source` config driver-generic: add provider fields alongside Vault/AWS without breaking existing YAML; route `fetch.go` through the registry instead of hardcoded vault/aws branches.
- [x] M0.4 Make `Target` config driver-generic: `Target` gains an optional backend selector (default aws for back-compat); refactor `syncTarget` (sync.go) to resolve a target backend via registry instead of constructing `aws.AwsClient` directly.
- [x] M0.5 Make `MergeStore` registry-driven (Vault/S3 today) so new merge backends slot in. Introduced `driver.BundleStore` matching actual bundle usage; S3 routed through it via `bundleStore()`; Vault merge path kept concrete/legacy (no roadmap item adds a new bundle backend).
- [x] M0.6 Migrate AWS + Vault clients onto the formal interfaces; assert interface satisfaction at compile time. Done incrementally via compile-time assertions across M0.1–M0.5 (AwsClient/VaultClient → Source/Target; S3MergeStore → MergeStore+BundleStore). All existing tests green.

### M1 v2.4.0 — Additional Secret Stores
- [x] M1.1 Kubernetes Secrets target backend (pkg/client/k8s): namespace-scoped writes, Opaque/TLS/dockerconfigjson types, RBAC. Register driver `kubernetes`.
- [x] M1.2 Generic HTTP store (pkg/client/httpstore): source+target via webhook; auth bearer/mTLS/custom headers; circuit-breaker + retry integration. Driver `http`.
- [x] M1.3 Azure Key Vault backend (pkg/client/azure): read/write source+target; Azure AD auth (service principal, managed identity, workload identity federation); cross-tenant via RBAC. Driver `azure`.
- [x] M1.4 GCP Secret Manager backend (pkg/client/gcp): read/write source+target; service account + workload identity auth; project isolation. Driver `gcp`.
- [x] M1.5 Wire all four into config structs, fetch/sync, USAGE.md driver docs, examples/, and docs guards. CRD + Helm chart support new target drivers (CRD already x-kubernetes-preserve-unknown-fields:true on targets).

### M2 v2.5.0 — Observability & Tracing
- [x] M2.1 OpenTelemetry tracing: spans across fetch/merge/sync + per-backend API calls; attributes target/source/operation/phase; OTLP/Jaeger/Zipkin exporters; configurable sampling. Config block `observability.tracing`. (Jaeger via native OTLP.)
- [x] M2.2 Enhanced metrics: user-defined custom metrics via config; dependency-probe health endpoints.
- [x] M2.3 Ship Prometheus alerting rules + Grafana dashboard templates under deploy/.
- [x] M2.4 Performance: discovery result caching (Organizations/Identity Center — ouCache/ouChildCache already present); configurable AWS retry/backoff (max_retries/retry_mode); concurrent source reads with configurable limit (merge.parallel) preserving deep-merge priority order.

### M3 v2.6.0 — Enterprise Governance
- [x] M3.1 Policy as Code: declarative allow/deny sync policies per target/source in config; validated during `validate`; pre-sync enforcement with dry-run preview.
- [x] M3.2 Audit logging: structured log for every read/write/delete; tamper-evident hash chaining; destinations file/CloudWatch/S3.
- [x] M3.3 Client-side encryption for S3 merge store: KMS-managed or user-supplied keys; zero-knowledge mode (encrypt before reaching merge store).
- [x] M3.4 Rollback automation: auto-rollback on sync failure (pre-sync snapshot restore + delete-created); configurable safety check (max_secrets cap).

### M4 v2.7.0 — Advanced Workflows
- [x] M4.1 Conditional sync: env/tag/time-window gating; per-target skip rules (skipped = successful no-op).
- [x] M4.2 Per-target scheduling in the controller: cron-like per target (spec.schedule); staggered windows (spec.staggerMinutes, deterministic per-name minute offset); tz-aware evaluation (spec.timezone → CronJob.TimeZone).
- [x] M4.3 Multi-instance coordination: leader election (RunAsLeader) for multi-replica controller; distributed locking via S3 conditional writes (If-None-Match:*); stable-hash work partitioning.

### M5 v3.0+ — Future Considerations
- [x] M5.1 Regional merge stores with cross-region replication (ReplicatingBundleStore: write fan-out primary+replicas, read primary→replica fallback; replica_regions/require_all_replicas config).
- [x] M5.2 Advanced security: post-quantum ML-KEM-768 (FIPS 203) hybrid encryption-at-rest for the merge store (post_quantum_seed_env). Zero-trust controller↔provider (in-cluster RBAC + mTLS HTTP backend) and cross-org federated identity (Azure workload-identity-federation / GCP workload-identity already supported in M1) are deployment-config postures, not new code.
- [x] M5.3 Scale: pluggable cache layer (pkg/cache Cache interface + in-memory TTL + GetOrCompute; Redis/Memcached slot in via the interface); Azure Functions serverless target (cmd/secrets-sync-azurefunc) over a shared pkg/serverless core (Lambda refactored to a thin adapter). Event-driven async: serverless entrypoints are the queue/event-trigger surface (Lambda event + Azure Functions trigger).

### M6 Release hygiene
- [x] M6.1 Update ROADMAP.md "Current Status" to reflect shipped surface; moved completed v2.4–v3.0 items into a descriptive "Recently shipped" section (no version-numbered headers; release-please owns versions).
- [ ] M6.2 Full `just ci` green; docs warnings=errors; vuln scan clean. Open the single PR.
