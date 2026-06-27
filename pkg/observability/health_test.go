package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthCheckerAllPass(t *testing.T) {
	h := NewHealthChecker(time.Second)
	h.Register("vault", func(context.Context) error { return nil })
	h.Register("aws", func(context.Context) error { return nil })

	rec := httptest.NewRecorder()
	h.Handler()(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" || len(resp.Checks) != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHealthCheckerOneFails(t *testing.T) {
	h := NewHealthChecker(time.Second)
	h.Register("vault", func(context.Context) error { return nil })
	h.Register("aws", func(context.Context) error { return fmt.Errorf("unreachable") })

	rec := httptest.NewRecorder()
	h.Handler()(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var resp healthResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "fail" || resp.Checks["aws"].Status != "fail" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Checks["aws"].Error != "unreachable" {
		t.Fatalf("missing probe error: %+v", resp.Checks["aws"])
	}
}

func TestHealthCheckerNoProbes(t *testing.T) {
	h := NewHealthChecker(0) // default timeout
	rec := httptest.NewRecorder()
	h.Handler()(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("no probes should be healthy, got %d", rec.Code)
	}
}

func TestHealthCheckerProbeTimeout(t *testing.T) {
	h := NewHealthChecker(50 * time.Millisecond)
	h.Register("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	})
	rec := httptest.NewRecorder()
	h.Handler()(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("slow probe should fail via timeout, got %d", rec.Code)
	}
}

func TestProbeNamesSorted(t *testing.T) {
	h := NewHealthChecker(time.Second)
	h.Register("zeta", func(context.Context) error { return nil })
	h.Register("alpha", func(context.Context) error { return nil })
	got := h.ProbeNames()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("expected sorted names, got %v", got)
	}
}
