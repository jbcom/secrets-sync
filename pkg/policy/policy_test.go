package policy

import "testing"

func TestCompileDefaults(t *testing.T) {
	e, err := Compile(Config{})
	if err != nil {
		t.Fatalf("empty config should compile: %v", err)
	}
	// Empty policy permits everything (opt-in semantics).
	if d := e.Evaluate("any", "thing"); !d.Allowed || d.Rule != "default" {
		t.Fatalf("empty policy should allow by default: %+v", d)
	}
}

func TestCompileInvalid(t *testing.T) {
	cases := []Config{
		{DefaultAction: "maybe"},
		{Rules: []Rule{{Action: "permit"}}},
		{Rules: []Rule{{Action: Allow, Source: "(unclosed"}}},
		{Rules: []Rule{{Action: Deny, Target: "[bad"}}},
	}
	for i, c := range cases {
		if _, err := Compile(c); err == nil {
			t.Fatalf("case %d should fail to compile", i)
		}
	}
}

func TestEvaluateFirstMatchWins(t *testing.T) {
	e, err := Compile(Config{
		DefaultAction: Deny,
		Rules: []Rule{
			{Name: "allow-shared-to-prod", Source: "^shared$", Target: "^prod", Action: Allow},
			{Name: "deny-all-prod", Target: "^prod", Action: Deny},
		},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// First rule matches shared→prod-east → allowed.
	if d := e.Evaluate("shared", "prod-east"); !d.Allowed || d.Rule != "allow-shared-to-prod" {
		t.Fatalf("shared→prod should be allowed by first rule: %+v", d)
	}
	// Second rule matches other→prod-west → denied.
	if d := e.Evaluate("other", "prod-west"); d.Allowed || d.Rule != "deny-all-prod" {
		t.Fatalf("other→prod should be denied by second rule: %+v", d)
	}
	// No rule matches dev → default deny.
	if d := e.Evaluate("shared", "dev"); d.Allowed || d.Rule != "default" {
		t.Fatalf("unmatched should hit default deny: %+v", d)
	}
}

func TestEmptyPatternMatchesAny(t *testing.T) {
	e, _ := Compile(Config{Rules: []Rule{{Name: "block-prod", Target: "prod", Action: Deny}}})
	// Empty source pattern matches any source.
	if d := e.Evaluate("anything", "prod-db"); d.Allowed {
		t.Fatalf("empty source pattern should match any source: %+v", d)
	}
	// Non-prod falls through to default allow.
	if d := e.Evaluate("x", "staging"); !d.Allowed {
		t.Fatalf("non-matching target should default-allow: %+v", d)
	}
}
