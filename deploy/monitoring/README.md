# Monitoring

Ready-to-apply observability artifacts for secrets-sync.

## `prometheus-rules.yaml`

A `PrometheusRule` (kube-prometheus-stack) with alerts for pipeline errors,
sync latency above the 100ms p95 SLO, stalled runs, Vault errors, AWS API
latency, and cache-miss spikes. For standalone Prometheus, copy the `groups`
block into a `rule_files` entry.

```bash
kubectl apply -f deploy/monitoring/prometheus-rules.yaml
```

## `grafana-dashboard.json`

A Grafana dashboard (uid `secrets-sync`) covering target throughput, per-phase
errors and latency, AWS/Vault API latency, and AWS cache hit ratio. Import it
and select your Prometheus data source for the `DS_PROMETHEUS` input.

All panels reference the `secrets_sync_*` metric families exported on the
`/metrics` endpoint. Custom metrics declared via `observability.custom_metrics`
appear under the `secrets_sync_custom_*` namespace and can be added as
additional panels.
