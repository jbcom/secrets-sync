package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestRegisterCustomMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	cm, err := RegisterCustomMetrics(reg, []CustomMetricConfig{
		{Name: "syncs_total", Type: "counter", Help: "total syncs", Labels: []string{"target"}},
		{Name: "queue_depth", Type: "gauge", Help: "queue depth"},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !cm.Has("syncs_total") || !cm.Has("queue_depth") {
		t.Fatal("metrics not registered")
	}

	cm.IncCounter("syncs_total", "prod")
	cm.IncCounter("syncs_total", "prod")
	cm.SetGauge("queue_depth", 7)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	got := map[string]float64{}
	for _, mf := range mfs {
		for _, m := range mf.Metric {
			got[mf.GetName()] = metricValue(m)
		}
	}
	if got["secrets_sync_custom_syncs_total"] != 2 {
		t.Fatalf("counter value wrong: %v", got)
	}
	if got["secrets_sync_custom_queue_depth"] != 7 {
		t.Fatalf("gauge value wrong: %v", got)
	}
}

func metricValue(m *dto.Metric) float64 {
	if m.Counter != nil {
		return m.Counter.GetValue()
	}
	if m.Gauge != nil {
		return m.Gauge.GetValue()
	}
	return 0
}

func TestRegisterCustomMetricsErrors(t *testing.T) {
	cases := []CustomMetricConfig{
		{Name: "", Type: "counter"},
		{Name: "x", Type: "histogram"},
	}
	for _, c := range cases {
		if _, err := RegisterCustomMetrics(prometheus.NewRegistry(), []CustomMetricConfig{c}); err == nil {
			t.Fatalf("expected error for config %+v", c)
		}
	}
}

func TestRegisterCustomMetricsDuplicate(t *testing.T) {
	_, err := RegisterCustomMetrics(prometheus.NewRegistry(), []CustomMetricConfig{
		{Name: "dup", Type: "counter"},
		{Name: "dup", Type: "gauge"},
	})
	if err == nil {
		t.Fatal("expected duplicate-name error")
	}
}

func TestCustomMetricsUnknownNameNoPanic(t *testing.T) {
	cm, _ := RegisterCustomMetrics(prometheus.NewRegistry(), nil)
	// Must be a safe no-op, not a panic.
	cm.IncCounter("nope")
	cm.SetGauge("nope", 1)
}
