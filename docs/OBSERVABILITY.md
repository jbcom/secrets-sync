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
secretsync pipeline --config config.yaml --metrics-port 9090

# Custom address and port
secretsync pipeline --config config.yaml --metrics-addr 0.0.0.0 --metrics-port 8080
```

### Environment Variables

```bash
export SECRETSYNC_METRICS_PORT=9090
export SECRETSYNC_METRICS_ADDR=0.0.0.0
secretsync pipeline --config config.yaml
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secretsync
spec:
  template:
    spec:
      containers:
      - name: secretsync
        image: extended-data-library/secretssync:latest
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

## Available Metrics

### Vault Metrics

#### `secretsync_vault_api_call_duration_seconds`
**Type**: Histogram  
**Labels**: `operation`, `status`  
**Description**: Duration of Vault API calls in seconds

**Operations**:
- `list_secrets`: BFS traversal to list all secrets
- `get_secret`: Retrieve individual secret

**Status**: `success`, `error`

**Example**:
```prometheus
secretsync_vault_api_call_duration_seconds_bucket{operation="list_secrets",status="success",le="0.1"} 42
secretsync_vault_api_call_duration_seconds_sum{operation="list_secrets",status="success"} 2.5
secretsync_vault_api_call_duration_seconds_count{operation="list_secrets",status="success"} 45
```

#### `secretsync_vault_secrets_listed_total`
**Type**: Counter  
**Labels**: `path`  
**Description**: Total number of secrets listed from Vault

**Example**:
```prometheus
secretsync_vault_secrets_listed_total{path="kv/prod/app"} 150
```

#### `secretsync_vault_traversal_depth`
**Type**: Histogram  
**Labels**: `path`  
**Description**: Depth reached during BFS traversal

Useful for detecting deep directory structures that may impact performance.

#### `secretsync_vault_queue_size`
**Type**: Gauge  
**Labels**: `path`  
**Description**: Current size of the BFS traversal queue

Indicates how many paths are pending during recursive listing.

#### `secretsync_vault_errors_total`
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

#### `secretsync_aws_api_call_duration_seconds`
**Type**: Histogram  
**Labels**: `operation`, `region`, `status`  
**Description**: Duration of AWS API calls in seconds

**Operations**:
- `list_secrets`: List all secrets (with pagination)
- `write_secret`: Create or update secret
- `delete_secret`: Delete secret

**Example**:
```prometheus
secretsync_aws_api_call_duration_seconds_bucket{operation="list_secrets",region="us-east-1",status="success",le="1"} 10
```

#### `secretsync_aws_pagination_pages`
**Type**: Histogram  
**Labels**: `operation`  
**Description**: Number of pagination pages processed

Tracks how many pages were required for list operations. High values may indicate performance opportunities.

#### `secretsync_aws_cache_hits_total` / `secretsync_aws_cache_misses_total`
**Type**: Counter  
**Labels**: `operation`  
**Description**: Cache hit/miss counters for AWS operations

Monitor cache effectiveness when `CacheTTL` is configured.

**Example**:
```prometheus
# Cache hit rate calculation
rate(secretsync_aws_cache_hits_total[5m]) / 
  (rate(secretsync_aws_cache_hits_total[5m]) + rate(secretsync_aws_cache_misses_total[5m]))
```

#### `secretsync_aws_secrets_operations_total`
**Type**: Counter  
**Labels**: `operation`, `status`  
**Description**: Total number of secrets operations

**Operations**: `create`, `update`, `skip`, `delete`  
**Status**: `success`, `error`

### Pipeline Metrics

#### `secretsync_pipeline_execution_duration_seconds`
**Type**: Histogram  
**Labels**: `phase`, `operation`  
**Description**: Duration of pipeline execution phases

**Phases**: `merge`, `sync`  
**Operations**: `merge`, `sync`, `pipeline`

**Example**:
```prometheus
secretsync_pipeline_execution_duration_seconds_sum{phase="merge",operation="pipeline"} 45.2
secretsync_pipeline_execution_duration_seconds_count{phase="merge",operation="pipeline"} 1
```

#### `secretsync_pipeline_targets_processed_total`
**Type**: Counter  
**Labels**: `phase`, `status`  
**Description**: Total number of targets processed

**Example**:
```prometheus
secretsync_pipeline_targets_processed_total{phase="merge",status="success"} 10
secretsync_pipeline_targets_processed_total{phase="sync",status="error"} 2
```

#### `secretsync_pipeline_parallel_workers`
**Type**: Gauge  
**Labels**: `phase`  
**Description**: Number of active parallel workers

Real-time view of parallelism during execution.

#### `secretsync_pipeline_errors_total`
**Type**: Counter  
**Labels**: `phase`, `error_type`  
**Description**: Total number of pipeline errors

### S3 Metrics

#### `secretsync_s3_operation_duration_seconds`
**Type**: Histogram  
**Labels**: `operation`, `status`  
**Description**: Duration of S3 operations

**Operations**: S3 read/write for merge store operations

#### `secretsync_s3_object_size_bytes`
**Type**: Histogram  
**Labels**: `operation`  
**Description**: Size of S3 objects in bytes

## Prometheus Configuration

### Scrape Configuration

Add SecretSync to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'secretsync'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:9090']
        labels:
          app: 'secretsync'
          env: 'production'
```

### Kubernetes Service Monitor

For Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: secretsync
  namespace: default
spec:
  selector:
    matchLabels:
      app: secretsync
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Useful Queries

### Performance Monitoring

```prometheus
# Average Vault list operation duration
rate(secretsync_vault_api_call_duration_seconds_sum{operation="list_secrets"}[5m]) /
  rate(secretsync_vault_api_call_duration_seconds_count{operation="list_secrets"}[5m])

# AWS API call error rate
rate(secretsync_aws_api_call_duration_seconds_count{status="error"}[5m])

# Pipeline execution time (full pipeline)
secretsync_pipeline_execution_duration_seconds_sum{operation="pipeline"}

# Current parallel workers
secretsync_pipeline_parallel_workers
```

### Cache Performance

```prometheus
# AWS cache hit rate
rate(secretsync_aws_cache_hits_total[5m]) /
  (rate(secretsync_aws_cache_hits_total[5m]) + rate(secretsync_aws_cache_misses_total[5m]))
```

### Error Monitoring

```prometheus
# Vault error rate
rate(secretsync_vault_errors_total[5m])

# Pipeline target failures
rate(secretsync_pipeline_targets_processed_total{status="error"}[5m])

# Recent AWS write errors
increase(secretsync_aws_secrets_operations_total{status="error"}[1h])
```

### Capacity Planning

```prometheus
# Secrets per mount
secretsync_vault_secrets_listed_total

# Pagination overhead
avg(secretsync_aws_pagination_pages)

# BFS traversal depth
histogram_quantile(0.95, rate(secretsync_vault_traversal_depth_bucket[5m]))
```

## Alerting Rules

Example Prometheus alerting rules:

```yaml
groups:
- name: secretsync
  rules:
  - alert: SecretSyncHighErrorRate
    expr: rate(secretsync_vault_errors_total[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate in SecretSync Vault operations"
      description: "{{ $labels.operation }} error rate is {{ $value }}/sec"

  - alert: SecretSyncPipelineFailures
    expr: increase(secretsync_pipeline_targets_processed_total{status="error"}[30m]) > 5
    labels:
      severity: critical
    annotations:
      summary: "SecretSync pipeline has failed targets"
      description: "{{ $value }} targets failed in the last 30 minutes"

  - alert: SecretSyncSlowVaultOperations
    expr: |
      histogram_quantile(0.95,
        rate(secretsync_vault_api_call_duration_seconds_bucket[5m])
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
```prometheus
secretsync_pipeline_execution_duration_seconds_sum{phase="merge"}
```

**Secrets Throughput**:
```prometheus
rate(secretsync_vault_secrets_listed_total[5m])
```

**Active Workers**:
```prometheus
secretsync_pipeline_parallel_workers
```

**Error Rate by Component**:
```prometheus
sum by (operation) (rate(secretsync_vault_errors_total[5m]))
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
