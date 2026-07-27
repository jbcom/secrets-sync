package policy

import "testing"

// Policy patterns decide where credentials may be written, so they must match
// whole names. Go's MatchString is a substring search, which would silently
// widen every rule: a deny for "prod" would also catch "nonprod", and an allow
// for "vault-prod" would be satisfied by an attacker-named "evil-vault-prod-x".
func TestPatternsMatchWholeNamesOnly(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		target  string
		want    bool
	}{
		{"exact match allowed", "prod", "prod", true},
		{"prefixed name is a different target", "prod", "nonprod", false},
		{"suffixed name is a different target", "prod", "prod-eu", false},
		{"embedded name is a different target", "vault-prod", "evil-vault-prod-x", false},
		{"explicit anchors still work", "^prod$", "prod", true},
		{"wildcard still spans", ".*", "anything", true},
		{"prefix wildcard is honored", "prod-.*", "prod-eu", true},
		{"prefix wildcard does not match the bare stem", "prod-.*", "prod", false},
		{"alternation anchors as a whole", "staging|prod", "prod", true},
		{"alternation does not match a superstring", "staging|prod", "prod-eu", false},
		{"alternation does not match a prefixed superstring", "staging|prod", "xstaging", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := Compile(Config{
				DefaultAction: Deny,
				Rules:         []Rule{{Target: tc.pattern, Action: Allow}},
			})
			if err != nil {
				t.Fatalf("compile %q: %v", tc.pattern, err)
			}

			got := engine.Evaluate("any-source", tc.target).Allowed
			if got != tc.want {
				t.Errorf("pattern %q against target %q: allowed=%v, want %v", tc.pattern, tc.target, got, tc.want)
			}
		})
	}
}

// The same widening applies to source patterns.
func TestSourcePatternsMatchWholeNamesOnly(t *testing.T) {
	engine, err := Compile(Config{
		DefaultAction: Allow,
		Rules:         []Rule{{Source: "prod", Action: Deny}},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if engine.Evaluate("nonprod", "any-target").Allowed == false {
		t.Error(`a deny rule for source "prod" must not also deny "nonprod"`)
	}
	if engine.Evaluate("prod", "any-target").Allowed {
		t.Error(`a deny rule for source "prod" must deny "prod"`)
	}
}

// Anchoring must not turn a genuinely malformed pattern into a silent pass.
func TestInvalidPatternStillFailsToCompile(t *testing.T) {
	if _, err := Compile(Config{
		DefaultAction: Deny,
		Rules:         []Rule{{Target: "prod(", Action: Allow}},
	}); err == nil {
		t.Fatal("expected an unbalanced-parenthesis pattern to fail compilation")
	}
}
