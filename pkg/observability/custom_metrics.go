package observability

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// CustomMetricConfig declares a user-defined metric in pipeline config.
type CustomMetricConfig struct {
	// Name is the metric name (without the secrets_sync_custom_ prefix).
	Name string `mapstructure:"name" yaml:"name"`
	// Type is "counter" or "gauge".
	Type string `mapstructure:"type" yaml:"type"`
	// Help is the metric help text.
	Help string `mapstructure:"help" yaml:"help"`
	// Labels are the metric's label names.
	Labels []string `mapstructure:"labels" yaml:"labels,omitempty"`
}

const customSubsystem = "custom"

// CustomMetrics holds user-defined counters and gauges registered from config.
// It is safe for concurrent use.
type CustomMetrics struct {
	mu       sync.RWMutex
	counters map[string]*prometheus.CounterVec
	gauges   map[string]*prometheus.GaugeVec
}

// RegisterCustomMetrics builds custom metrics from config and registers them
// into the given registry. Duplicate or invalid declarations return an error.
func RegisterCustomMetrics(reg prometheus.Registerer, configs []CustomMetricConfig) (*CustomMetrics, error) {
	cm := &CustomMetrics{
		counters: map[string]*prometheus.CounterVec{},
		gauges:   map[string]*prometheus.GaugeVec{},
	}
	for _, c := range configs {
		if c.Name == "" {
			return nil, fmt.Errorf("custom metric: name is required")
		}
		if _, dup := cm.counters[c.Name]; dup {
			return nil, fmt.Errorf("custom metric %q: duplicate name", c.Name)
		}
		if _, dup := cm.gauges[c.Name]; dup {
			return nil, fmt.Errorf("custom metric %q: duplicate name", c.Name)
		}
		switch c.Type {
		case "counter":
			v := prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace, Subsystem: customSubsystem, Name: c.Name, Help: c.Help,
			}, c.Labels)
			if err := reg.Register(v); err != nil {
				return nil, fmt.Errorf("register custom counter %q: %w", c.Name, err)
			}
			cm.counters[c.Name] = v
		case "gauge":
			v := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: namespace, Subsystem: customSubsystem, Name: c.Name, Help: c.Help,
			}, c.Labels)
			if err := reg.Register(v); err != nil {
				return nil, fmt.Errorf("register custom gauge %q: %w", c.Name, err)
			}
			cm.gauges[c.Name] = v
		default:
			return nil, fmt.Errorf("custom metric %q: unsupported type %q (want counter or gauge)", c.Name, c.Type)
		}
	}
	return cm, nil
}

// IncCounter increments a registered custom counter by 1. Unknown names are a
// no-op so instrumentation calls never panic on a typo.
func (cm *CustomMetrics) IncCounter(name string, labelValues ...string) {
	cm.mu.RLock()
	c := cm.counters[name]
	cm.mu.RUnlock()
	if c != nil {
		c.WithLabelValues(labelValues...).Inc()
	}
}

// SetGauge sets a registered custom gauge. Unknown names are a no-op.
func (cm *CustomMetrics) SetGauge(name string, value float64, labelValues ...string) {
	cm.mu.RLock()
	g := cm.gauges[name]
	cm.mu.RUnlock()
	if g != nil {
		g.WithLabelValues(labelValues...).Set(value)
	}
}

// Has reports whether a metric with the given name is registered.
func (cm *CustomMetrics) Has(name string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, c := cm.counters[name]
	_, g := cm.gauges[name]
	return c || g
}
