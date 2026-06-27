package pipeline

import (
	"context"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/audit"
	"github.com/jbcom/secrets-sync/pkg/driver"
	"github.com/jbcom/secrets-sync/pkg/policy"
)

// memAuditSink captures audit entries for assertions.
type memAuditSink struct{ entries []audit.Entry }

func (m *memAuditSink) Write(_ context.Context, e audit.Entry) error {
	m.entries = append(m.entries, e)
	return nil
}
func (m *memAuditSink) Close() error { return nil }

func TestPipelineAuditEmitsChainedEntries(t *testing.T) {
	sink := &memAuditSink{}
	p := &Pipeline{auditor: audit.NewLogger(sink)}

	p.audit(context.Background(), audit.Record{Operation: audit.OpWrite, Driver: "aws", Target: "prod", Secret: "db", Success: true})
	p.audit(context.Background(), audit.Record{Operation: audit.OpWrite, Driver: "aws", Target: "prod", Secret: "api", Success: true})

	if len(sink.entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(sink.entries))
	}
	if idx, err := audit.Verify(sink.entries); idx != -1 || err != nil {
		t.Fatalf("emitted entries should form a valid chain: idx=%d err=%v", idx, err)
	}

	// A nil auditor must be a safe no-op.
	(&Pipeline{}).audit(context.Background(), audit.Record{Operation: audit.OpWrite, Secret: "x"})
}

func TestPolicyDenied(t *testing.T) {
	eng, err := policy.Compile(policy.Config{
		DefaultAction: policy.Allow,
		Rules:         []policy.Rule{{Name: "no-secrets-to-dev", Source: "^secrets$", Target: "^dev", Action: policy.Deny}},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	p := &Pipeline{policy: eng}

	// Denied: secrets→dev.
	if err := p.policyDenied("dev-a", Target{Imports: []string{"secrets"}}); err == nil {
		t.Fatal("expected policy denial for secrets→dev")
	}
	// Allowed: secrets→prod (default allow).
	if err := p.policyDenied("prod-a", Target{Imports: []string{"secrets"}}); err != nil {
		t.Fatalf("secrets→prod should be allowed: %v", err)
	}
	// Nil engine is a no-op.
	if err := (&Pipeline{}).policyDenied("dev-a", Target{Imports: []string{"secrets"}}); err != nil {
		t.Fatalf("nil policy engine must allow: %v", err)
	}
}

// TestGetTargetBackend_RoutesNonDefaultDriver verifies that a target with an
// explicit non-AWS backend is constructed through the registry rather than the
// AWS path, and that an unknown driver surfaces an error.
func TestGetTargetBackend_RoutesNonDefaultDriver(t *testing.T) {
	const drv driver.DriverName = "fake-sink"
	reg := driver.NewRegistry()
	reg.RegisterTarget(drv, func(spec driver.BackendSpec) (driver.TargetBackend, error) {
		return &fakeSource{drv: drv, store: map[string][]byte{}}, nil
	})

	p := &Pipeline{backends: reg}

	tgt := Target{Backend: &TargetBackendConfig{Driver: string(drv), Path: "x", Options: map[string]any{"k": "v"}}}
	b, err := p.getTargetBackend(context.Background(), tgt)
	if err != nil {
		t.Fatalf("getTargetBackend: %v", err)
	}
	if b.Driver() != drv {
		t.Fatalf("expected driver %q, got %q", drv, b.Driver())
	}

	unknown := Target{Backend: &TargetBackendConfig{Driver: "nope"}}
	if _, err := p.getTargetBackend(context.Background(), unknown); err == nil {
		t.Fatal("expected error for unknown target driver")
	}
}

// TestGetTargetBackend_DefaultsToAWS verifies that an unset or aws-named backend
// does not consult the registry (so AWS cross-account auth is preserved).
func TestGetTargetBackend_DefaultsToAWS(t *testing.T) {
	// Registry deliberately has NO aws factory; the default path must not touch
	// it. We can't fully init AWS here without credentials, so we only assert
	// the call does not error out on "driver not registered" — it will instead
	// proceed into getAWSClientForTarget which builds (but does not connect) a
	// client. A nil/empty config yields an AWS client construction attempt.
	p := &Pipeline{backends: driver.NewRegistry(), config: &Config{}}
	// Backend unset → default AWS path. getAWSClientForTarget calls Init which
	// may fail without creds; we only require it not to be the registry's
	// "no target backend registered" error.
	_, err := p.getTargetBackend(context.Background(), Target{})
	if err != nil && err.Error() == `driver: no target backend registered for "aws"` {
		t.Fatalf("default path must not consult the registry: %v", err)
	}
}
