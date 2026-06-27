package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRequestRawBody(t *testing.T) {
	body := `{"config_path":"/tmp/c.yaml","options":{"operation":"sync"}}`
	r := httptest.NewRequest("POST", "/sync", strings.NewReader(body))
	req, err := decodeRequest(r)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.ConfigPath != "/tmp/c.yaml" || req.Options.Operation != "sync" {
		t.Fatalf("raw body not decoded: %+v", req)
	}
}

func TestDecodeRequestCustomHandlerEnvelope(t *testing.T) {
	// Azure delivers the request under Data["req"] as a JSON string.
	body := `{"Data":{"req":"{\"config_path\":\"/x.yaml\"}"}}`
	r := httptest.NewRequest("POST", "/sync", strings.NewReader(body))
	req, err := decodeRequest(r)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.ConfigPath != "/x.yaml" {
		t.Fatalf("envelope not decoded: %+v", req)
	}
}

func TestDecodeRequestEnvelopeObject(t *testing.T) {
	// Data["body"] as a JSON object (not string).
	body := `{"Data":{"body":{"config_path":"/y.yaml"}}}`
	r := httptest.NewRequest("POST", "/sync", strings.NewReader(body))
	req, err := decodeRequest(r)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.ConfigPath != "/y.yaml" {
		t.Fatalf("object envelope not decoded: %+v", req)
	}
}

func TestDecodeRequestInvalidBody(t *testing.T) {
	r := httptest.NewRequest("POST", "/sync", strings.NewReader("not json"))
	if _, err := decodeRequest(r); err == nil {
		t.Fatal("expected error for invalid body")
	}
}

func TestHandleInvocationReturnsFailureForMissingConfig(t *testing.T) {
	r := httptest.NewRequest("POST", "/sync", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	handleInvocation(w, r)
	// No config source → 500 with a JSON error body.
	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Fatalf("expected failure body, got %s", w.Body.String())
	}
}
