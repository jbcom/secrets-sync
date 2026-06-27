package httpstore

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeDoer records requests and returns canned responses keyed by "METHOD URL".
type fakeDoer struct {
	responses map[string]*http.Response
	requests  []*http.Request
	bodies    []string
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, req)
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		f.bodies = append(f.bodies, string(b))
	} else {
		f.bodies = append(f.bodies, "")
	}
	key := req.Method + " " + req.URL.String()
	if resp, ok := f.responses[key]; ok {
		return resp, nil
	}
	return resp(http.StatusNotFound, ""), nil
}

func resp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func newTestClient(doer HTTPDoer) *Client {
	c := &Client{BaseURL: "https://store.test/api", Path: "team", doer: doer}
	_ = c.Init(context.Background())
	return c
}

func TestNewRequiresBaseURL(t *testing.T) {
	if _, err := New(driver.BackendSpec{Driver: driver.DriverNameHTTP}); err == nil {
		t.Fatal("expected error when base_url missing")
	}
	c, err := New(driver.BackendSpec{Driver: driver.DriverNameHTTP, Options: map[string]any{"base_url": "https://x"}})
	if err != nil || c.BaseURL != "https://x" {
		t.Fatalf("New: c=%+v err=%v", c, err)
	}
}

func TestURLComposition(t *testing.T) {
	c := &Client{BaseURL: "https://store.test/api/", Path: "/team/"}
	if got := c.url("app/db"); got != "https://store.test/api/team/app/db" {
		t.Fatalf("url composition wrong: %q", got)
	}
}

func TestListSecrets(t *testing.T) {
	doer := &fakeDoer{responses: map[string]*http.Response{
		"GET https://store.test/api/team": resp(http.StatusOK, `{"secrets":["a","b"]}`),
	}}
	c := newTestClient(doer)
	names, err := c.ListSecrets(context.Background(), "")
	if err != nil || len(names) != 2 {
		t.Fatalf("list: names=%v err=%v", names, err)
	}
}

func TestListSecretsNotFoundIsEmpty(t *testing.T) {
	c := newTestClient(&fakeDoer{responses: map[string]*http.Response{}})
	names, err := c.ListSecrets(context.Background(), "")
	if err != nil || names != nil {
		t.Fatalf("404 list should be empty/nil: names=%v err=%v", names, err)
	}
}

func TestGetSecret(t *testing.T) {
	doer := &fakeDoer{responses: map[string]*http.Response{
		"GET https://store.test/api/team/app/db": resp(http.StatusOK, `{"u":"p"}`),
	}}
	c := newTestClient(doer)
	got, err := c.GetSecret(context.Background(), "app/db")
	if err != nil || string(got) != `{"u":"p"}` {
		t.Fatalf("get: got=%s err=%v", got, err)
	}
}

func TestWriteSecret(t *testing.T) {
	doer := &fakeDoer{responses: map[string]*http.Response{
		"PUT https://store.test/api/team/app/db": resp(http.StatusOK, ""),
	}}
	c := newTestClient(doer)
	if _, err := c.WriteSecret(context.Background(), metav1.ObjectMeta{}, "app/db", []byte(`{"u":"p"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(doer.bodies) != 1 || doer.bodies[0] != `{"u":"p"}` {
		t.Fatalf("body not sent: %v", doer.bodies)
	}
}

func TestDeleteIdempotent(t *testing.T) {
	c := newTestClient(&fakeDoer{responses: map[string]*http.Response{}}) // all 404
	if err := c.DeleteSecret(context.Background(), "gone"); err != nil {
		t.Fatalf("delete of missing must be nil: %v", err)
	}
}

func TestServerErrorDoesNotLeakBody(t *testing.T) {
	doer := &fakeDoer{responses: map[string]*http.Response{
		"GET https://store.test/api/team/app/db": resp(http.StatusInternalServerError, "super-secret-value-in-error"),
	}}
	c := newTestClient(doer)
	_, err := c.GetSecret(context.Background(), "app/db")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if strings.Contains(err.Error(), "super-secret-value-in-error") {
		t.Fatalf("error leaked response body: %v", err)
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("error should report status code: %v", err)
	}
}

func TestAuthHeaders(t *testing.T) {
	doer := &fakeDoer{responses: map[string]*http.Response{
		"GET https://store.test/api/team": resp(http.StatusOK, `{"secrets":[]}`),
	}}
	c := &Client{BaseURL: "https://store.test/api", Path: "team", BearerToken: "tok", Headers: map[string]string{"X-Tenant": "acme"}, doer: doer}
	_ = c.Init(context.Background())
	if _, err := c.ListSecrets(context.Background(), ""); err != nil {
		t.Fatalf("list: %v", err)
	}
	req := doer.requests[0]
	if req.Header.Get("Authorization") != "Bearer tok" {
		t.Fatalf("bearer not set: %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("X-Tenant") != "acme" {
		t.Fatalf("custom header not set: %q", req.Header.Get("X-Tenant"))
	}
}

func TestRedactStripsQuery(t *testing.T) {
	if got := redact("https://x/y?token=abc"); got != "https://x/y?<redacted>" {
		t.Fatalf("redact: %q", got)
	}
}
