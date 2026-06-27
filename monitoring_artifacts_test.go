package secrets_sync_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestPrometheusRulesAreValidYAML asserts the alerting rules parse as a
// PrometheusRule and reference secrets_sync_* metrics.
func TestPrometheusRulesAreValidYAML(t *testing.T) {
	data, err := os.ReadFile("deploy/monitoring/prometheus-rules.yaml")
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse rules yaml: %v", err)
	}
	if doc["kind"] != "PrometheusRule" {
		t.Fatalf("expected kind PrometheusRule, got %v", doc["kind"])
	}
	if !strings.Contains(string(data), "secrets_sync_pipeline_errors_total") {
		t.Fatal("rules should alert on secrets_sync_pipeline_errors_total")
	}
}

// TestGrafanaDashboardIsValidJSON asserts the dashboard parses and references
// the metric families.
func TestGrafanaDashboardIsValidJSON(t *testing.T) {
	data, err := os.ReadFile("deploy/monitoring/grafana-dashboard.json")
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse dashboard json: %v", err)
	}
	if doc["uid"] != "secrets-sync" {
		t.Fatalf("expected uid secrets-sync, got %v", doc["uid"])
	}
	panels, ok := doc["panels"].([]any)
	if !ok || len(panels) == 0 {
		t.Fatal("dashboard should define panels")
	}
	if !strings.Contains(string(data), "secrets_sync_") {
		t.Fatal("dashboard should reference secrets_sync_ metrics")
	}
}
