package condition

import (
	"testing"
	"time"
)

func at(h, m int) time.Time {
	return time.Date(2026, 6, 27, h, m, 0, 0, time.UTC)
}

func TestEmptyConfigAlwaysAllows(t *testing.T) {
	if r := (Config{}).Evaluate(EvalContext{Now: at(3, 0)}); !r.Allowed {
		t.Fatalf("empty config should allow: %+v", r)
	}
}

func TestEnvCondition(t *testing.T) {
	c := Config{Env: map[string]string{"DEPLOY_ENV": "prod"}}
	if r := c.Evaluate(EvalContext{Now: at(3, 0)}); r.Allowed {
		t.Fatal("should deny when env var unset")
	}
	t.Setenv("DEPLOY_ENV", "prod")
	if r := c.Evaluate(EvalContext{Now: at(3, 0)}); !r.Allowed {
		t.Fatalf("should allow when env matches: %+v", r)
	}
	t.Setenv("DEPLOY_ENV", "staging")
	if r := c.Evaluate(EvalContext{Now: at(3, 0)}); r.Allowed {
		t.Fatal("should deny when env mismatches")
	}
}

func TestTagCondition(t *testing.T) {
	c := Config{Tags: map[string]string{"tier": "critical"}}
	if r := c.Evaluate(EvalContext{Now: at(3, 0), Tags: map[string]string{"tier": "critical"}}); !r.Allowed {
		t.Fatalf("matching tag should allow: %+v", r)
	}
	if r := c.Evaluate(EvalContext{Now: at(3, 0), Tags: map[string]string{"tier": "low"}}); r.Allowed {
		t.Fatal("mismatched tag should deny")
	}
	if r := c.Evaluate(EvalContext{Now: at(3, 0)}); r.Allowed {
		t.Fatal("missing tag should deny")
	}
}

func TestTimeWindowNormal(t *testing.T) {
	c := Config{TimeWindows: []TimeWindow{{Start: "01:00", End: "05:00"}}}
	if r := c.Evaluate(EvalContext{Now: at(3, 0)}); !r.Allowed {
		t.Fatalf("03:00 should be inside 01:00-05:00: %+v", r)
	}
	if r := c.Evaluate(EvalContext{Now: at(6, 0)}); r.Allowed {
		t.Fatal("06:00 should be outside 01:00-05:00")
	}
}

func TestTimeWindowWrapsMidnight(t *testing.T) {
	c := Config{TimeWindows: []TimeWindow{{Start: "22:00", End: "02:00"}}}
	if r := c.Evaluate(EvalContext{Now: at(23, 30)}); !r.Allowed {
		t.Fatal("23:30 should be inside wrapping 22:00-02:00")
	}
	if r := c.Evaluate(EvalContext{Now: at(1, 0)}); !r.Allowed {
		t.Fatal("01:00 should be inside wrapping 22:00-02:00")
	}
	if r := c.Evaluate(EvalContext{Now: at(12, 0)}); r.Allowed {
		t.Fatal("12:00 should be outside wrapping 22:00-02:00")
	}
}

func TestTimeWindowTimezone(t *testing.T) {
	// 12:00 UTC is 07:00 in America/New_York (EDT, -5 in June... actually -4).
	c := Config{TimeWindows: []TimeWindow{{Start: "07:00", End: "09:00", Timezone: "America/New_York"}}}
	r := c.Evaluate(EvalContext{Now: time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)})
	if !r.Allowed {
		t.Fatalf("12:00 UTC = 08:00 EDT should be inside window: %+v", r)
	}
}

func TestAllConditionsMustHold(t *testing.T) {
	t.Setenv("E", "yes")
	c := Config{
		Env:         map[string]string{"E": "yes"},
		Tags:        map[string]string{"t": "v"},
		TimeWindows: []TimeWindow{{Start: "00:00", End: "23:59"}},
	}
	// All hold.
	if r := c.Evaluate(EvalContext{Now: at(10, 0), Tags: map[string]string{"t": "v"}}); !r.Allowed {
		t.Fatalf("all conditions hold, should allow: %+v", r)
	}
	// Tag fails → overall deny even though env+time pass.
	if r := c.Evaluate(EvalContext{Now: at(10, 0), Tags: map[string]string{"t": "x"}}); r.Allowed {
		t.Fatal("one failing condition should deny")
	}
}

func TestValidate(t *testing.T) {
	good := Config{TimeWindows: []TimeWindow{{Start: "01:00", End: "05:00", Timezone: "UTC"}}}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for _, bad := range []Config{
		{TimeWindows: []TimeWindow{{Start: "25:00", End: "05:00"}}},
		{TimeWindows: []TimeWindow{{Start: "01:00", End: "noon"}}},
		{TimeWindows: []TimeWindow{{Start: "01:00", End: "05:00", Timezone: "Mars/Olympus"}}},
	} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("expected validation error for %+v", bad)
		}
	}
}
