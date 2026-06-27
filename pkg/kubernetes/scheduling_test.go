package kubernetes

import (
	"strconv"
	"strings"
	"testing"
)

func TestStaggerScheduleNoOp(t *testing.T) {
	// stagger <= 1 is a no-op.
	if got := staggerSchedule("0 2 * * *", "x", 0); got != "0 2 * * *" {
		t.Fatalf("stagger 0 should be no-op, got %q", got)
	}
	if got := staggerSchedule("0 2 * * *", "x", 1); got != "0 2 * * *" {
		t.Fatalf("stagger 1 should be no-op, got %q", got)
	}
}

func TestStaggerScheduleOffsetsMinute(t *testing.T) {
	got := staggerSchedule("0 2 * * *", "target-a", 30)
	if got == "0 2 * * *" {
		t.Fatalf("expected a staggered minute, got unchanged %q", got)
	}
	// Same input must be deterministic.
	if again := staggerSchedule("0 2 * * *", "target-a", 30); again != got {
		t.Fatalf("stagger must be deterministic: %q != %q", got, again)
	}
	// Different names generally land on different minutes.
	other := staggerSchedule("0 2 * * *", "target-b", 30)
	if other == got {
		t.Logf("note: target-a and target-b hashed to the same offset (%q) — acceptable but rare", got)
	}
}

func TestStaggerScheduleLeavesComplexUnchanged(t *testing.T) {
	for _, sched := range []string{
		"*/5 * * * *",  // step minute
		"0,30 2 * * *", // list minute
		"0-10 2 * * *", // range minute
		"@daily",       // non-5-field
		"0 2 * *",      // wrong field count
	} {
		if got := staggerSchedule(sched, "x", 30); got != sched {
			t.Fatalf("complex schedule %q must be unchanged, got %q", sched, got)
		}
	}
}

func TestStaggerOffsetWithinBound(t *testing.T) {
	// The staggered minute must stay a valid 0-59 value, even when base+offset
	// would exceed 59 (wraps via mod 60).
	for _, name := range []string{"a", "bb", "ccc", "dddd", "team-platform-prod"} {
		got := staggerSchedule("55 2 * * *", name, 30)
		minute, err := strconv.Atoi(strings.Fields(got)[0])
		if err != nil {
			t.Fatalf("staggered schedule %q minute not parseable: %v", got, err)
		}
		if minute < 0 || minute > 59 {
			t.Fatalf("staggered minute %d out of range for %q", minute, name)
		}
	}
}

func TestTimeZonePtr(t *testing.T) {
	if timeZonePtr("") != nil {
		t.Fatal("empty timezone should be nil")
	}
	if got := timeZonePtr("Europe/Berlin"); got == nil || *got != "Europe/Berlin" {
		t.Fatalf("timezone pointer wrong: %v", got)
	}
}
