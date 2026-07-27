package pipeline

import (
	"context"
	"strings"
	"testing"
)

// A config may declare targets dynamically instead of statically -- Validate
// accepts a config with no static targets as long as dynamic_targets is set,
// and four documentation pages describe the feature. Expansion therefore has to
// actually run during construction: if it does not, such a config builds an
// empty dependency graph and the pipeline reports success while syncing
// nothing. A silent no-op is the worst outcome for a tool that moves
// credentials, so these tests pin the wiring rather than the discovery
// mechanics (which need live AWS).

func dynamicOnlyConfig() *Config {
	return &Config{
		Sources: map[string]Source{
			"app-secrets": {Vault: &VaultSource{Mount: "secret", Paths: []string{"app"}}},
		},
		DynamicTargets: map[string]DynamicTarget{
			"discovered": {
				Discovery: DiscoveryConfig{
					AccountsList: &AccountsListDiscovery{Source: "/secrets-sync/accounts"},
				},
				Imports: []string{"app-secrets"},
			},
		},
	}
}

// Validate must not accept a shape the pipeline cannot execute.
func TestDynamicOnlyConfigIsAcceptedByValidate(t *testing.T) {
	if err := dynamicOnlyConfig().Validate(); err != nil {
		t.Fatalf("dynamic-only config should validate, got: %v", err)
	}
}

// The wiring guarantee: constructing a pipeline from a dynamic-only config must
// either produce targets or fail loudly. It must never yield a pipeline that
// runs clean over zero targets.
func TestDynamicTargetsAreExpandedDuringConstruction(t *testing.T) {
	cfg := dynamicOnlyConfig()

	p, err := NewWithContext(context.Background(), cfg)
	if err != nil {
		// Discovery needs AWS credentials, so a failure here is expected in a
		// unit-test environment. What matters is that it surfaces rather than
		// silently degrading to an empty target set.
		if strings.Contains(err.Error(), "dynamic") || strings.Contains(err.Error(), "discover") {
			return
		}
		t.Fatalf("unexpected construction error: %v", err)
	}

	if len(p.config.Targets) == 0 && len(p.graph.TopologicalOrder()) == 0 {
		t.Fatal("dynamic-only config produced a pipeline with zero targets; " +
			"expansion is not wired in, so this pipeline would report success while syncing nothing")
	}
}

// Expansion must not disturb the ordinary static path.
func TestStaticTargetsUnaffectedByExpansion(t *testing.T) {
	cfg := &Config{
		Sources: map[string]Source{"app-secrets": {Vault: &VaultSource{Mount: "secret", Paths: []string{"app"}}}},
		Targets: map[string]Target{
			"static-one": {AccountID: "111111111111", Imports: []string{"app-secrets"}},
		},
	}

	p, err := NewWithContext(context.Background(), cfg)
	if err != nil {
		t.Fatalf("static config should build: %v", err)
	}

	if _, ok := p.config.Targets["static-one"]; !ok {
		t.Error("expansion dropped a statically declared target")
	}
}
