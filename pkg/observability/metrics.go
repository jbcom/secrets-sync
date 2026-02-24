// Package observability provides metrics and tracing for production debugging.
package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Namespace for all metrics
	namespace = "secretsync"

	// Subsystems
	subsystemVault    = "vault"
	subsystemAWS      = "aws"
	subsystemPipeline = "pipeline"
	subsystemS3       = "s3"
)

var (
	// Vault metrics
	VaultAPICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemVault,
			Name:      "api_call_duration_seconds",
			Help:      "Duration of Vault API calls in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "status"},
	)

	VaultSecretsListed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemVault,
			Name:      "secrets_listed_total",
			Help:      "Total number of secrets listed from Vault",
		},
		[]string{"path"},
	)

	VaultTraversalDepth = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemVault,
			Name:      "traversal_depth",
			Help:      "Depth reached during BFS traversal",
			Buckets:   []float64{1, 5, 10, 20, 50, 100},
		},
		[]string{"path"},
	)

	VaultQueueSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemVault,
			Name:      "queue_size",
			Help:      "Current size of the BFS traversal queue",
		},
		[]string{"path"},
	)

	VaultErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemVault,
			Name:      "errors_total",
			Help:      "Total number of Vault errors",
		},
		[]string{"operation", "error_type"},
	)

	// AWS Secrets Manager metrics
	AWSAPICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemAWS,
			Name:      "api_call_duration_seconds",
			Help:      "Duration of AWS API calls in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "region", "status"},
	)

	AWSPaginationCount = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemAWS,
			Name:      "pagination_pages",
			Help:      "Number of pagination pages processed",
			Buckets:   []float64{1, 2, 5, 10, 20, 50, 100},
		},
		[]string{"operation"},
	)

	AWSCacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemAWS,
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		},
		[]string{"operation"},
	)

	AWSCacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemAWS,
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		},
		[]string{"operation"},
	)

	AWSSecretsOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemAWS,
			Name:      "secrets_operations_total",
			Help:      "Total number of secrets operations",
		},
		[]string{"operation", "status"},
	)

	// Pipeline metrics
	PipelineExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemPipeline,
			Name:      "execution_duration_seconds",
			Help:      "Duration of pipeline execution in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"phase", "operation"},
	)

	PipelineTargetsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemPipeline,
			Name:      "targets_processed_total",
			Help:      "Total number of targets processed",
		},
		[]string{"phase", "status"},
	)

	PipelineParallelWorkers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemPipeline,
			Name:      "parallel_workers",
			Help:      "Number of active parallel workers",
		},
		[]string{"phase"},
	)

	PipelineErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemPipeline,
			Name:      "errors_total",
			Help:      "Total number of pipeline errors",
		},
		[]string{"phase", "error_type"},
	)

	// S3 merge store metrics
	S3OperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemS3,
			Name:      "operation_duration_seconds",
			Help:      "Duration of S3 operations in seconds",
			Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "status"},
	)

	S3ObjectSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemS3,
			Name:      "object_size_bytes",
			Help:      "Size of S3 objects in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100B to ~100MB
		},
		[]string{"operation"},
	)
)

// Registry holds all metrics
var Registry = prometheus.NewRegistry()

// init registers all metrics
func init() {
	// Vault metrics
	Registry.MustRegister(VaultAPICallDuration)
	Registry.MustRegister(VaultSecretsListed)
	Registry.MustRegister(VaultTraversalDepth)
	Registry.MustRegister(VaultQueueSize)
	Registry.MustRegister(VaultErrors)

	// AWS metrics
	Registry.MustRegister(AWSAPICallDuration)
	Registry.MustRegister(AWSPaginationCount)
	Registry.MustRegister(AWSCacheHits)
	Registry.MustRegister(AWSCacheMisses)
	Registry.MustRegister(AWSSecretsOperations)

	// Pipeline metrics
	Registry.MustRegister(PipelineExecutionDuration)
	Registry.MustRegister(PipelineTargetsProcessed)
	Registry.MustRegister(PipelineParallelWorkers)
	Registry.MustRegister(PipelineErrors)

	// S3 metrics
	Registry.MustRegister(S3OperationDuration)
	Registry.MustRegister(S3ObjectSize)
}

// Handler returns an HTTP handler for Prometheus metrics
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordDuration is a helper to record histogram durations
func RecordDuration(histogram *prometheus.HistogramVec, start time.Time, labels ...string) {
	duration := time.Since(start).Seconds()
	histogram.WithLabelValues(labels...).Observe(duration)
}

// RecordError is a helper to record errors
func RecordError(counter *prometheus.CounterVec, labels ...string) {
	counter.WithLabelValues(labels...).Inc()
}
