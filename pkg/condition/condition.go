// Package condition implements declarative gating for conditional sync: a
// target can be skipped unless environment, tag, and time-window conditions are
// all satisfied. Conditions are evaluated immediately before the sync phase.
package condition

import (
	"fmt"
	"os"
	"time"
)

// Config is the conditions block on a target. A nil/empty Config always passes.
// All configured conditions must hold (logical AND) for the sync to proceed.
type Config struct {
	// Env requires named environment variables to equal the given values.
	Env map[string]string `mapstructure:"env" yaml:"env,omitempty"`
	// Tags requires the target's tags to contain the given key=value pairs.
	Tags map[string]string `mapstructure:"tags" yaml:"tags,omitempty"`
	// TimeWindows restrict syncing to the listed recurring daily windows. If any
	// window is configured, the current time must fall within at least one.
	TimeWindows []TimeWindow `mapstructure:"time_windows" yaml:"time_windows,omitempty"`
}

// TimeWindow is a recurring daily allow-window in a named timezone. Start and
// End are "HH:MM" 24-hour local times; a window where End < Start wraps past
// midnight (e.g. 22:00–02:00).
type TimeWindow struct {
	Start    string `mapstructure:"start" yaml:"start"`
	End      string `mapstructure:"end" yaml:"end"`
	Timezone string `mapstructure:"timezone" yaml:"timezone,omitempty"`
}

// EvalContext carries the inputs a condition is evaluated against.
type EvalContext struct {
	Now  time.Time
	Tags map[string]string
}

// Result reports whether a sync is allowed and, if not, why.
type Result struct {
	Allowed bool
	Reason  string
}

// Validate checks the config is well-formed (parseable times/timezones) so
// errors surface at `secrets-sync validate` rather than at sync time.
func (c Config) Validate() error {
	for i, w := range c.TimeWindows {
		if _, err := parseHM(w.Start); err != nil {
			return fmt.Errorf("condition: time_windows[%d].start: %w", i, err)
		}
		if _, err := parseHM(w.End); err != nil {
			return fmt.Errorf("condition: time_windows[%d].end: %w", i, err)
		}
		if w.Timezone != "" {
			if _, err := time.LoadLocation(w.Timezone); err != nil {
				return fmt.Errorf("condition: time_windows[%d].timezone: %w", i, err)
			}
		}
	}
	return nil
}

// Evaluate returns whether the sync should proceed given the context.
func (c Config) Evaluate(ec EvalContext) Result {
	for k, want := range c.Env {
		if os.Getenv(k) != want {
			return Result{Allowed: false, Reason: fmt.Sprintf("env %s != %q", k, want)}
		}
	}
	for k, want := range c.Tags {
		if ec.Tags[k] != want {
			return Result{Allowed: false, Reason: fmt.Sprintf("tag %s != %q", k, want)}
		}
	}
	if len(c.TimeWindows) > 0 {
		if !c.inAnyWindow(ec.Now) {
			return Result{Allowed: false, Reason: "outside all configured time windows"}
		}
	}
	return Result{Allowed: true}
}

func (c Config) inAnyWindow(now time.Time) bool {
	for _, w := range c.TimeWindows {
		loc := time.UTC
		if w.Timezone != "" {
			if l, err := time.LoadLocation(w.Timezone); err == nil {
				loc = l
			}
		}
		local := now.In(loc)
		cur := local.Hour()*60 + local.Minute()
		start, errS := parseHM(w.Start)
		end, errE := parseHM(w.End)
		if errS != nil || errE != nil {
			continue
		}
		if start <= end {
			if cur >= start && cur <= end {
				return true
			}
		} else {
			// Wrapping window (e.g. 22:00–02:00).
			if cur >= start || cur <= end {
				return true
			}
		}
	}
	return false
}

// parseHM parses "HH:MM" into minutes-since-midnight.
func parseHM(s string) (int, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, fmt.Errorf("invalid HH:MM time %q", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q out of range", s)
	}
	return h*60 + m, nil
}
