// Package httpstore implements a generic HTTP/webhook secret store backend.
//
// It targets custom or third-party secret stores that expose a small REST
// convention over a base URL:
//
//	GET    {base}/{path}            -> {"secrets": ["name", ...]}   (list)
//	GET    {base}/{path}/{name}     -> {<secret JSON>}              (read)
//	PUT    {base}/{path}/{name}     <- {<secret JSON>}             (write)
//	DELETE {base}/{path}/{name}                                    (delete)
//
// Authentication supports bearer tokens, arbitrary custom headers, and mTLS
// client certificates. All calls are wrapped in the shared circuit breaker.
package httpstore

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jbcom/secrets-sync/pkg/circuitbreaker"
	"github.com/jbcom/secrets-sync/pkg/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Compile-time assertions: the HTTP store is a full source + sync target.
var (
	_ driver.SourceBackend = (*Client)(nil)
	_ driver.TargetBackend = (*Client)(nil)
)

// HTTPDoer is the subset of http.Client used, abstracted for testing.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is a generic HTTP secret store scoped to a base URL and path prefix.
type Client struct {
	// BaseURL is the store root, e.g. "https://secrets.internal/api/v1".
	BaseURL string
	// Path is the scope prefix beneath BaseURL (the backend's GetPath()).
	Path string
	// BearerToken, when set, is sent as "Authorization: Bearer <token>".
	BearerToken string
	// Headers are extra headers applied to every request.
	Headers map[string]string
	// ClientCert/ClientKey/CACert enable mTLS when all are provided.
	ClientCert string
	ClientKey  string
	CACert     string
	// Timeout bounds each request (default 30s).
	Timeout time.Duration
	// InsecureSkipVerify disables TLS verification (test/dev only).
	InsecureSkipVerify bool

	doer    HTTPDoer
	breaker *circuitbreaker.CircuitBreaker
}

// listResponse is the expected shape of the list endpoint.
type listResponse struct {
	Secrets []string `json:"secrets"`
}

// New constructs an HTTP store backend from a driver.BackendSpec. Path is the
// scope prefix; required option "base_url" sets the root; optional options:
// bearer_token, headers (map[string]string), client_cert, client_key, ca_cert,
// timeout_seconds (int), insecure_skip_verify (bool).
func New(spec driver.BackendSpec) (*Client, error) {
	c := &Client{Path: spec.Path, Timeout: 30 * time.Second}
	if spec.Options != nil {
		if v, ok := spec.Options["base_url"].(string); ok {
			c.BaseURL = v
		}
		if v, ok := spec.Options["bearer_token"].(string); ok {
			c.BearerToken = v
		}
		if v, ok := spec.Options["headers"].(map[string]string); ok {
			c.Headers = v
		}
		if v, ok := spec.Options["client_cert"].(string); ok {
			c.ClientCert = v
		}
		if v, ok := spec.Options["client_key"].(string); ok {
			c.ClientKey = v
		}
		if v, ok := spec.Options["ca_cert"].(string); ok {
			c.CACert = v
		}
		if v, ok := spec.Options["insecure_skip_verify"].(bool); ok {
			c.InsecureSkipVerify = v
		}
		if v, ok := spec.Options["timeout_seconds"].(int); ok && v > 0 {
			c.Timeout = time.Duration(v) * time.Second
		}
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("httpstore: base_url is required")
	}
	return c, nil
}

// Init builds the HTTP client (including mTLS transport) and circuit breaker.
func (c *Client) Init(_ context.Context) error {
	if c.breaker == nil {
		c.breaker = circuitbreaker.New(circuitbreaker.DefaultConfig("httpstore:" + c.BaseURL))
	}
	if c.doer != nil {
		return nil
	}

	transport := &http.Transport{}
	tlsCfg := &tls.Config{InsecureSkipVerify: c.InsecureSkipVerify} //nolint:gosec // opt-in dev flag
	if c.ClientCert != "" && c.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCert, c.ClientKey)
		if err != nil {
			return fmt.Errorf("httpstore: load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	transport.TLSClientConfig = tlsCfg
	c.doer = &http.Client{Timeout: c.Timeout, Transport: transport}
	return nil
}

// Driver reports the HTTP driver name.
func (c *Client) Driver() driver.DriverName { return driver.DriverNameHTTP }

// GetPath returns the scope prefix.
func (c *Client) GetPath() string { return c.Path }

// Close is a no-op.
func (c *Client) Close() error { return nil }

func (c *Client) url(parts ...string) string {
	segs := []string{strings.TrimRight(c.BaseURL, "/")}
	if c.Path != "" {
		segs = append(segs, strings.Trim(c.Path, "/"))
	}
	for _, p := range parts {
		if p = strings.Trim(p, "/"); p != "" {
			segs = append(segs, p)
		}
	}
	return strings.Join(segs, "/")
}

func (c *Client) newRequest(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	if c.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// do executes a request through the circuit breaker and returns the body bytes
// for 2xx responses. A 404 yields (nil, errNotFound) so callers can treat it as
// absence.
func (c *Client) do(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	if c.doer == nil {
		return nil, fmt.Errorf("httpstore: not initialized")
	}
	return circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) ([]byte, error) {
		req, err := c.newRequest(ctx, method, url, body)
		if err != nil {
			return nil, err
		}
		resp, err := c.doer.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return nil, errNotFound
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Status code only — never echo the response body, which may contain
			// secret material or sensitive error context.
			return nil, fmt.Errorf("httpstore: %s %s returned status %d", method, redact(url), resp.StatusCode)
		}
		return payload, nil
	})
}

var errNotFound = fmt.Errorf("httpstore: not found")

// ListSecrets GETs the scope and returns the names from the list response.
func (c *Client) ListSecrets(ctx context.Context, path string) ([]string, error) {
	payload, err := c.do(ctx, http.MethodGet, c.url(path), nil)
	if err != nil {
		if err == errNotFound {
			return nil, nil
		}
		return nil, err
	}
	var lr listResponse
	if err := json.Unmarshal(payload, &lr); err != nil {
		return nil, fmt.Errorf("httpstore: decode list response: %w", err)
	}
	return lr.Secrets, nil
}

// GetSecret GETs a single secret's raw JSON payload.
func (c *Client) GetSecret(ctx context.Context, path string) ([]byte, error) {
	payload, err := c.do(ctx, http.MethodGet, c.url(path), nil)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// WriteSecret PUTs a secret's JSON payload to the store.
func (c *Client) WriteSecret(ctx context.Context, _ metav1.ObjectMeta, path string, secret []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPut, c.url(path), secret)
}

// DeleteSecret DELETEs a secret. A missing secret is not an error.
func (c *Client) DeleteSecret(ctx context.Context, path string) error {
	_, err := c.do(ctx, http.MethodDelete, c.url(path), nil)
	if err == errNotFound {
		return nil
	}
	return err
}

// redact strips any query string (which could carry tokens) from a URL before
// it appears in an error message.
func redact(u string) string {
	if i := strings.IndexByte(u, '?'); i >= 0 {
		return u[:i] + "?<redacted>"
	}
	return u
}
