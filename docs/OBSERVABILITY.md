# Observability: Metrics and Monitoring

SecretSync provides comprehensive observability features including Prometheus metrics for production debugging and monitoring.

## Metrics Overview

SecretSync exposes Prometheus metrics for:
- **Vault operations**: API call latency, BFS traversal metrics, error rates
- **AWS Secrets Manager operations**: API call latency, pagination, cache performance
- **Pipeline execution**: Phase timings, target processing, parallel worker usage
- **S3 merge store**: Operation latency and object sizes

## Enabling Metrics

### Command Line

Start SecretSync with metrics enabled:

```bash
# Enable metrics on default port 9090
secrets-sync pipeline --config config.yaml --metrics-port 9090

# Custom address and port
secrets-sync pipeline --config config.yaml --metrics-addr 0.0.0.0 --metrics-port 8080
```

### Environment Variables

```bash
export SECRETS_SYNC_METRICS_PORT=9090
export SECRETS_SYNC_METRICS_ADDR=0.0.0.0
secrets-sync pipeline --config config.yaml
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secrets-sync
spec:
  template:
    spec:
      containers:
      - name: secrets-sync
        image: jbcom/secrets-sync:v1
        args:
          - pipeline
          - --config
          - /config/config.yaml
          - --metrics-port
          - "9090"
        ports:
        - containerPort: 9090
          name: metrics
          protocol: TCP
```

## Metrics Endpoint

Once enabled, metrics are exposed at:
- **Metrics**: `http://localhost:9090/metrics`
- **Health check**: `http://localhost:9090/health`

## Logging Safety

Logs and metrics are designed for operational visibility without exposing the
secret bytes being synchronized. SecretSync records request IDs, operation
names, durations, counts, paths, targets, account identifiers, and provider
error context. It must not log raw secret values, raw Vault secret payloads, raw
AWS secret payloads, or raw client structures.

Use JSON logs when shipping to a centralized platform:

```bash
secrets-sync pipeline --config config.yaml --log-format json --log-level info
```

Debug and trace logging can reveal more operational metadata and should be sent
only to secured sinks with appropriate retention. If a provider returns
credentials in an error string, treat that upstream behavior as a provider or
configuration issue and rotate the exposed credential.

Machine-readable `secrets-sync pipeline --output json` result envelopes redact
common secret-bearing fragments from top-level and per-target error strings
before serialization. Treat `error_message`, per-target `error`, and
`diff_output` as operationally sensitive when forwarding them to logs,
dashboards, CI comments, or chat systems.
GitHub Actions annotation output escapes workflow-command data in target names
and secret paths before writing groups, notices, or warnings.

## Available Metrics

### Vault Metrics

#### `secrets_sync_vault_api_call_duration_seconds`
**Type**: Histogram  
**Labels**: `operation`, `status`  
**Description**: Duration of Vault API calls in seconds

**Operations**:
- `list_secrets`: BFS traversal to list all secrets
- `get_secret`: Retrieve individual secret

**Status**: `success`, `error`

**Example**:
```text
secrets_sync_vault_api_call_duration_seconds_bucket{operation="list_secrets",status="success",le="0.1"} 42
secrets_sync_vault_api_call_duration_seconds_sum{operation="list_secrets",status="success"} 2.5
secrets_sync_vault_api_call_duration_seconds_count{operation="list_secrets",status="success"} 45
```

#### `secrets_sync_vault_secrets_listed_total`
**Type**: Counter  
**Labels**: `path`  
**Description**: Total number of secrets listed from Vault

**Example**:
```text
secrets_sync_vault_secrets_listed_total{path="kv/prod/app"} 150
```

#### `secrets_sync_vault_traversal_depth`
**Type**: Histogram  
**Labels**: `path`  
**Description**: Depth reached during BFS traversal

Useful for detecting deep directory structures that may impact performance.

#### `secrets_sync_vault_queue_size`
**Type**: Gauge  
**Labels**: `path`  
**Description**: Current size of the BFS traversal queue

Indicates how many paths are pending during recursive listing.

#### `secrets_sync_vault_errors_total`
**Type**: Counter  
**Labels**: `operation`, `error_type`  
**Description**: Total number of Vault errors

**Error types**:
- `list_path`: Failed to list path contents
- `access_denied`: 403/404 errors (expected for inaccessible paths)
- `api_error`: Other API errors
- `max_depth_exceeded`: Traversal depth limit hit
- `invalid_path`, `not_initialized`, `not_found`, `no_data`, `invalid_type`: Get secret errors

### AWS Metrics

#### `secrets_sync_aws_api_call_duration_seconds`
**Type**: Histogram  
**Labels**: `operation`, `region`, `status`  
**Description**: Duration of AWS API calls in seconds

**Operations**:
- `list_secrets`: List all secrets (with pagination)
- `write_secret`: Create or update secret
- `delete_secret`: Delete secret

**Example**:
```text
secrets_sync_aws_api_call_duration_seconds_bucket{operation="list_secrets",region="us-east-1",status="success",le="1"} 10
```

#### `secrets_sync_aws_pagination_pages`
**Type**: Histogram  
**Labels**: `operation`  
**Description**: Number of pagination pages processed

Tracks how many pages were required for list operations. High values may indicate performance opportunities.

#### `secrets_sync_aws_cache_hits_total` / `secrets_sync_aws_cache_misses_total`
**Type**: Counter  
**Labels**: `operation`  
**Description**: Cache hit/miss counters for AWS operations

Monitor cache effectiveness when `CacheTTL` is configured.

**Example**:
```text
# Cache hit rate calculation
rate(secrets_sync_aws_cache_hits_total[5m]) /
  (rate(secrets_sync_aws_cache_hits_total[5m]) + rate(secrets_sync_aws_cache_misses_total[5m]))
```

#### `secrets_sync_aws_secrets_operations_total`
**Type**: Counter  
**Labels**: `operation`, `status`  
**Description**: Total number of secrets operations

**Operations**: `create`, `update`, `skip`, `delete`  
**Status**: `success`, `error`

### Pipeline Metrics

#### `secrets_sync_pipeline_execution_duration_seconds`
**Type**: Histogram  
**Labels**: `phase`, `operation`  
**Description**: Duration of pipeline execution phases

**Phases**: `merge`, `sync`  
**Operations**: `merge`, `sync`, `pipeline`

**Example**:
```text
secrets_sync_pipeline_execution_duration_seconds_sum{phase="merge",operation="pipeline"} 45.2
secrets_sync_pipeline_execution_duration_seconds_count{phase="merge",operation="pipeline"} 1
```

#### `secrets_sync_pipeline_targets_processed_total`
**Type**: Counter  
**Labels**: `phase`, `status`  
**Description**: Total number of targets processed

**Example**:
```text
secrets_sync_pipeline_targets_processed_total{phase="merge",status="success"} 10
secrets_sync_pipeline_targets_processed_total{phase="sync",status="error"} 2
```

#### `secrets_sync_pipeline_parallel_workers`
**Type**: Gauge  
**Labels**: `phase`  
**Description**: Number of active parallel workers

Real-time view of parallelism during execution.

#### `secrets_sync_pipeline_errors_total`
**Type**: Counter  
**Labels**: `phase`, `error_type`  
**Description**: Total number of pipeline errors

### S3 Metrics

#### `secrets_sync_s3_operation_duration_seconds`
**Type**: Histogram  
**Labels**: `operation`, `status`  
**Description**: Duration of S3 operations

**Operations**: S3 read/write for merge store operations

#### `secrets_sync_s3_object_size_bytes`
**Type**: Histogram  
**Labels**: `operation`  
**Description**: Size of S3 objects in bytes

## Prometheus Configuration

### Scrape Configuration

Add SecretSync to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'secrets-sync'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:9090']
        labels:
          app: 'secrets-sync'
          env: 'production'
```

### Kubernetes Service Monitor

For Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: secrets-sync
  namespace: default
spec:
  selector:
    matchLabels:
      app: secrets-sync
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Useful Queries

### Performance Monitoring

```text
# Average Vault list operation duration
rate(secrets_sync_vault_api_call_duration_seconds_sum{operation="list_secrets"}[5m]) /
  rate(secrets_sync_vault_api_call_duration_seconds_count{operation="list_secrets"}[5m])

# AWS API call error rate
rate(secrets_sync_aws_api_call_duration_seconds_count{status="error"}[5m])

# Pipeline execution time (full pipeline)
secrets_sync_pipeline_execution_duration_seconds_sum{operation="pipeline"}

# Current parallel workers
secrets_sync_pipeline_parallel_workers
```

### Cache Performance

```text
# AWS cache hit rate
rate(secrets_sync_aws_cache_hits_total[5m]) /
  (rate(secrets_sync_aws_cache_hits_total[5m]) + rate(secrets_sync_aws_cache_misses_total[5m]))
```

### Error Monitoring

```text
# Vault error rate
rate(secrets_sync_vault_errors_total[5m])

# Pipeline target failures
rate(secrets_sync_pipeline_targets_processed_total{status="error"}[5m])

# Recent AWS write errors
increase(secrets_sync_aws_secrets_operations_total{status="error"}[1h])
```

### Capacity Planning

```text
# Secrets per mount
secrets_sync_vault_secrets_listed_total

# Pagination overhead
avg(secrets_sync_aws_pagination_pages)

# BFS traversal depth
histogram_quantile(0.95, rate(secrets_sync_vault_traversal_depth_bucket[5m]))
```

## Alerting Rules

Example Prometheus alerting rules:

```yaml
groups:
- name: secrets-sync
  rules:
  - alert: SecretSyncHighErrorRate
    expr: rate(secrets_sync_vault_errors_total[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate in SecretSync Vault operations"
      description: "{{ $labels.operation }} error rate is {{ $value }}/sec"

  - alert: SecretSyncPipelineFailures
    expr: increase(secrets_sync_pipeline_targets_processed_total{status="error"}[30m]) > 5
    labels:
      severity: critical
    annotations:
      summary: "SecretSync pipeline has failed targets"
      description: "{{ $value }} targets failed in the last 30 minutes"

  - alert: SecretSyncSlowVaultOperations
    expr: |
      histogram_quantile(0.95,
        rate(secrets_sync_vault_api_call_duration_seconds_bucket[5m])
      ) > 10
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "SecretSync Vault operations are slow"
      description: "P95 latency is {{ $value }}s"
```

## Grafana Dashboards

### Example Dashboard Panels

**Pipeline Execution Time**:
```text
secrets_sync_pipeline_execution_duration_seconds_sum{phase="merge"}
```

**Secrets Throughput**:
```text
rate(secrets_sync_vault_secrets_listed_total[5m])
```

**Active Workers**:
```text
secrets_sync_pipeline_parallel_workers
```

**Error Rate by Component**:
```text
sum by (operation) (rate(secrets_sync_vault_errors_total[5m]))
```

## Best Practices

1. **Scrape Interval**: Use 15-30 second intervals for production monitoring
2. **Retention**: Keep at least 30 days of metrics for trend analysis
3. **Alerting**: Set up alerts for error rates, not just failures
4. **Labels**: Use consistent labeling across environments (dev, staging, prod)
5. **Cardinality**: Monitor metric cardinality if using dynamic path labels

## Troubleshooting

### Metrics Not Appearing

1. Check that `--metrics-port` flag is set
2. Verify firewall allows access to metrics port
3. Check logs for "Starting metrics server" message
4. Test endpoint: `curl http://localhost:9090/metrics`

### High Cardinality

If you see performance issues:
- Review unique label combinations
- Vault `path` labels can create high cardinality with many mounts
- Consider aggregating metrics or using recording rules

### Missing Metrics

- Metrics are only emitted when operations occur
- Run a pipeline execution to generate metrics
- Some metrics (like cache hits) only appear when caching is enabled

## Future Enhancements

Planned observability improvements:
- Distributed tracing with OpenTelemetry (optional)
- Custom exporters (CloudWatch, Datadog)
- Metric sampling for very high-volume environments
- SLI/SLO tracking dashboards
