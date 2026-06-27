package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Probe is a named dependency health check. It returns nil when the dependency
// is reachable/healthy and an error describing the failure otherwise.
type Probe func(ctx context.Context) error

// HealthChecker aggregates dependency probes behind /health and /ready
// endpoints. Probes are evaluated concurrently with a per-request timeout.
type HealthChecker struct {
	mu      sync.RWMutex
	probes  map[string]Probe
	timeout time.Duration
}

// NewHealthChecker returns a HealthChecker with the given per-probe timeout
// (defaulting to 5s when zero).
func NewHealthChecker(timeout time.Duration) *HealthChecker {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HealthChecker{probes: map[string]Probe{}, timeout: timeout}
}

// Register adds or replaces a named probe.
func (h *HealthChecker) Register(name string, p Probe) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.probes[name] = p
}

// probeResult is the per-dependency outcome reported in the JSON body.
type probeResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// healthResponse is the aggregate /health and /ready payload.
type healthResponse struct {
	Status string                 `json:"status"`
	Checks map[string]probeResult `json:"checks,omitempty"`
}

// evaluate runs every probe concurrently and returns the aggregate plus whether
// all passed.
func (h *HealthChecker) evaluate(ctx context.Context) (healthResponse, bool) {
	h.mu.RLock()
	probes := make(map[string]Probe, len(h.probes))
	for n, p := range h.probes {
		probes[n] = p
	}
	timeout := h.timeout
	h.mu.RUnlock()

	resp := healthResponse{Status: "ok", Checks: map[string]probeResult{}}
	if len(probes) == 0 {
		return resp, true
	}

	type named struct {
		name string
		err  error
	}
	results := make(chan named, len(probes))
	for name, probe := range probes {
		go func(name string, probe Probe) {
			pctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			results <- named{name: name, err: probe(pctx)}
		}(name, probe)
	}

	allOK := true
	for i := 0; i < len(probes); i++ {
		r := <-results
		if r.err != nil {
			allOK = false
			resp.Checks[r.name] = probeResult{Status: "fail", Error: r.err.Error()}
		} else {
			resp.Checks[r.name] = probeResult{Status: "ok"}
		}
	}
	if !allOK {
		resp.Status = "fail"
	}
	return resp, allOK
}

// Handler serves the aggregate health/readiness JSON. It returns 200 when all
// probes pass and 503 otherwise.
func (h *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, ok := h.evaluate(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if ok {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// ProbeNames returns the registered probe names, sorted (for tests/diagnostics).
func (h *HealthChecker) ProbeNames() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.probes))
	for n := range h.probes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
