package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are registered
	collectors := []prometheus.Collector{
		VaultAPICallDuration,
		VaultSecretsListed,
		VaultTraversalDepth,
		VaultQueueSize,
		VaultErrors,
		AWSAPICallDuration,
		AWSPaginationCount,
		AWSCacheHits,
		AWSCacheMisses,
		AWSSecretsOperations,
		PipelineExecutionDuration,
		PipelineTargetsProcessed,
		PipelineParallelWorkers,
		PipelineErrors,
		S3OperationDuration,
		S3ObjectSize,
	}

	for _, collector := range collectors {
		if err := Registry.Register(collector); err != nil {
			// Already registered is expected
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				t.Errorf("Unexpected error registering collector: %v", err)
			}
		}
	}
}

func TestVaultMetrics(t *testing.T) {
	// Test Vault API call duration
	start := time.Now().Add(-100 * time.Millisecond)
	RecordDuration(VaultAPICallDuration, start, "list_secrets", "success")

	// Verify metric was recorded
	if count := testutil.CollectAndCount(VaultAPICallDuration); count == 0 {
		t.Error("VaultAPICallDuration metric not recorded")
	}

	// Test secrets listed counter
	VaultSecretsListed.WithLabelValues("kv/test").Add(10)
	if count := testutil.CollectAndCount(VaultSecretsListed); count == 0 {
		t.Error("VaultSecretsListed metric not recorded")
	}

	// Test traversal depth
	VaultTraversalDepth.WithLabelValues("kv/test").Observe(5)
	if count := testutil.CollectAndCount(VaultTraversalDepth); count == 0 {
		t.Error("VaultTraversalDepth metric not recorded")
	}

	// Test queue size gauge
	VaultQueueSize.WithLabelValues("kv/test").Set(15)
	if count := testutil.CollectAndCount(VaultQueueSize); count == 0 {
		t.Error("VaultQueueSize metric not recorded")
	}

	// Test error counter
	RecordError(VaultErrors, "list_secrets", "api_error")
	if count := testutil.CollectAndCount(VaultErrors); count == 0 {
		t.Error("VaultErrors metric not recorded")
	}
}

func TestAWSMetrics(t *testing.T) {
	// Test AWS API call duration
	start := time.Now().Add(-200 * time.Millisecond)
	RecordDuration(AWSAPICallDuration, start, "list_secrets", "us-east-1", "success")

	if count := testutil.CollectAndCount(AWSAPICallDuration); count == 0 {
		t.Error("AWSAPICallDuration metric not recorded")
	}

	// Test pagination count
	AWSPaginationCount.WithLabelValues("list_secrets").Observe(3)
	if count := testutil.CollectAndCount(AWSPaginationCount); count == 0 {
		t.Error("AWSPaginationCount metric not recorded")
	}

	// Test cache hits/misses
	AWSCacheHits.WithLabelValues("list_secrets").Inc()
	if count := testutil.CollectAndCount(AWSCacheHits); count == 0 {
		t.Error("AWSCacheHits metric not recorded")
	}

	AWSCacheMisses.WithLabelValues("list_secrets").Inc()
	if count := testutil.CollectAndCount(AWSCacheMisses); count == 0 {
		t.Error("AWSCacheMisses metric not recorded")
	}

	// Test secrets operations
	AWSSecretsOperations.WithLabelValues("create", "success").Inc()
	if count := testutil.CollectAndCount(AWSSecretsOperations); count == 0 {
		t.Error("AWSSecretsOperations metric not recorded")
	}
}

func TestPipelineMetrics(t *testing.T) {
	// Test pipeline execution duration
	start := time.Now().Add(-5 * time.Second)
	RecordDuration(PipelineExecutionDuration, start, "merge", "pipeline")

	if count := testutil.CollectAndCount(PipelineExecutionDuration); count == 0 {
		t.Error("PipelineExecutionDuration metric not recorded")
	}

	// Test targets processed
	PipelineTargetsProcessed.WithLabelValues("merge", "success").Inc()
	if count := testutil.CollectAndCount(PipelineTargetsProcessed); count == 0 {
		t.Error("PipelineTargetsProcessed metric not recorded")
	}

	// Test parallel workers gauge
	PipelineParallelWorkers.WithLabelValues("execute").Set(4)
	if count := testutil.CollectAndCount(PipelineParallelWorkers); count == 0 {
		t.Error("PipelineParallelWorkers metric not recorded")
	}

	// Test pipeline errors
	PipelineErrors.WithLabelValues("sync", "target_error").Inc()
	if count := testutil.CollectAndCount(PipelineErrors); count == 0 {
		t.Error("PipelineErrors metric not recorded")
	}
}

func TestS3Metrics(t *testing.T) {
	// Test S3 operation duration
	start := time.Now().Add(-50 * time.Millisecond)
	RecordDuration(S3OperationDuration, start, "read", "success")

	if count := testutil.CollectAndCount(S3OperationDuration); count == 0 {
		t.Error("S3OperationDuration metric not recorded")
	}

	// Test object size
	S3ObjectSize.WithLabelValues("write").Observe(1024)
	if count := testutil.CollectAndCount(S3ObjectSize); count == 0 {
		t.Error("S3ObjectSize metric not recorded")
	}
}

func TestRecordDurationHelper(t *testing.T) {
	// Test that RecordDuration helper works correctly
	start := time.Now().Add(-1 * time.Second)
	RecordDuration(VaultAPICallDuration, start, "test", "success")

	// The duration should be approximately 1 second
	// We can't test exact value due to timing, but we can verify it was recorded
	if count := testutil.CollectAndCount(VaultAPICallDuration); count == 0 {
		t.Error("RecordDuration did not record metric")
	}
}

func TestRecordErrorHelper(t *testing.T) {
	// Reset counter for clean test
	initialCount := testutil.CollectAndCount(VaultErrors)

	// Record an error
	RecordError(VaultErrors, "test_op", "test_error")

	// Verify counter increased
	newCount := testutil.CollectAndCount(VaultErrors)
	if newCount <= initialCount {
		t.Error("RecordError did not increment counter")
	}
}

func TestMetricsNamespace(t *testing.T) {
	// Verify metrics use correct namespace
	if namespace != "secretsync" {
		t.Errorf("Expected namespace 'secretsync', got '%s'", namespace)
	}
}

func TestMetricsSubsystems(t *testing.T) {
	// Verify subsystems are correctly defined
	expectedSubsystems := map[string]string{
		"vault":    subsystemVault,
		"aws":      subsystemAWS,
		"pipeline": subsystemPipeline,
		"s3":       subsystemS3,
	}

	for expected, actual := range expectedSubsystems {
		if expected != actual {
			t.Errorf("Expected subsystem '%s', got '%s'", expected, actual)
		}
	}
}

func TestMetricsHandler(t *testing.T) {
	// Test that Handler returns a valid HTTP handler
	handler := Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	// Record some metrics to ensure they're exposed
	VaultAPICallDuration.WithLabelValues("test_op", "success").Observe(0.1)
	AWSAPICallDuration.WithLabelValues("test_op", "us-east-1", "success").Observe(0.2)
	PipelineExecutionDuration.WithLabelValues("merge", "pipeline").Observe(1.0)

	// Create a test request
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Verify response contains Prometheus metrics format
	body := rr.Body.String()
	if !strings.Contains(body, "# HELP") {
		t.Error("Response does not contain Prometheus format")
	}
	if !strings.Contains(body, "secretsync_vault_api_call_duration_seconds") {
		t.Error("Response does not contain Vault metrics")
	}
	if !strings.Contains(body, "secretsync_aws_api_call_duration_seconds") {
		t.Error("Response does not contain AWS metrics")
	}
	if !strings.Contains(body, "secretsync_pipeline_execution_duration_seconds") {
		t.Error("Response does not contain Pipeline metrics")
	}
}
