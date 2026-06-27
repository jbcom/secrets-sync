package pipeline

import (
	"context"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/driver"
)

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
