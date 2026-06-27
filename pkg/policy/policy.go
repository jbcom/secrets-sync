// Package policy implements declarative allow/deny sync policies (policy as
// code). Policies gate which source→target syncs are permitted, are validated
// statically during `secrets-sync validate`, and are enforced before the sync
// phase writes anything.
package policy

import (
	"fmt"
	"regexp"
)

// Action is a policy decision.
type Action string

const (
	// Allow permits a matching source→target sync.
	Allow Action = "allow"
	// Deny blocks a matching source→target sync.
	Deny Action = "deny"
)

// Rule is a single allow/deny rule. Source and Target are regular expressions
// matched against the source name and target name respectively; an empty
// pattern matches anything. Rules are evaluated in order; the first match wins.
type Rule struct {
	Name   string `mapstructure:"name" yaml:"name,omitempty"`
	Source string `mapstructure:"source" yaml:"source,omitempty"`
	Target string `mapstructure:"target" yaml:"target,omitempty"`
	Action Action `mapstructure:"action" yaml:"action"`

	sourceRe *regexp.Regexp
	targetRe *regexp.Regexp
}

// Config is the policy block in pipeline config.
type Config struct {
	// DefaultAction is applied when no rule matches. Defaults to allow, so
	// policy is opt-in: an empty policy permits everything.
	DefaultAction Action `mapstructure:"default_action" yaml:"default_action,omitempty"`
	Rules         []Rule `mapstructure:"rules" yaml:"rules,omitempty"`
}

// Engine evaluates compiled policy rules.
type Engine struct {
	defaultAction Action
	rules         []Rule
}

// Compile validates and compiles a policy config into an Engine. It returns an
// error for invalid regexes or unknown actions, so misconfiguration is caught
// at `validate` time rather than mid-sync.
func Compile(cfg Config) (*Engine, error) {
	def := cfg.DefaultAction
	if def == "" {
		def = Allow
	}
	if def != Allow && def != Deny {
		return nil, fmt.Errorf("policy: invalid default_action %q (want allow or deny)", def)
	}

	rules := make([]Rule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		if r.Action != Allow && r.Action != Deny {
			return nil, fmt.Errorf("policy: rule %q: invalid action %q (want allow or deny)", ruleLabel(r, i), r.Action)
		}
		var err error
		if r.Source != "" {
			if r.sourceRe, err = regexp.Compile(r.Source); err != nil {
				return nil, fmt.Errorf("policy: rule %q: invalid source pattern: %w", ruleLabel(r, i), err)
			}
		}
		if r.Target != "" {
			if r.targetRe, err = regexp.Compile(r.Target); err != nil {
				return nil, fmt.Errorf("policy: rule %q: invalid target pattern: %w", ruleLabel(r, i), err)
			}
		}
		rules[i] = r
	}
	return &Engine{defaultAction: def, rules: rules}, nil
}

func ruleLabel(r Rule, i int) string {
	if r.Name != "" {
		return r.Name
	}
	return fmt.Sprintf("#%d", i)
}

// Decision is the outcome of evaluating a source→target pair.
type Decision struct {
	Allowed bool
	// Rule is the name/index of the matching rule, or "default" when none did.
	Rule string
}

// Evaluate returns the policy decision for syncing source into target. Rules are
// checked in order; the first whose source and target patterns both match
// decides. When none match, the default action applies.
func (e *Engine) Evaluate(source, target string) Decision {
	for i, r := range e.rules {
		if r.sourceRe != nil && !r.sourceRe.MatchString(source) {
			continue
		}
		if r.targetRe != nil && !r.targetRe.MatchString(target) {
			continue
		}
		return Decision{Allowed: r.Action == Allow, Rule: ruleLabel(r, i)}
	}
	return Decision{Allowed: e.defaultAction == Allow, Rule: "default"}
}
