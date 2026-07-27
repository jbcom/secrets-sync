package context

import (
	"strings"
	"testing"
)

// Values that reach a log field from outside the process -- provider API
// responses, config, user input -- must not be able to forge or corrupt log
// entries. A newline lets an attacker append a fabricated line; a carriage
// return can overwrite one on a terminal; ANSI escapes can hide text entirely.
func TestSafeLogValueStripsControlCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text passes through", "111111111111", "111111111111"},
		{"newline forging a second entry", "111\nlevel=fatal msg=\"breach\"", "111level=fatal msg=\"breach\""},
		{"carriage return overwriting a line", "111\rmalicious", "111malicious"},
		{"tab", "111\t222", "111222"},
		{"ANSI escape hiding output", "111\x1b[2Khidden", "111[2Khidden"},
		{"null byte", "111\x00222", "111222"},
		{"delete character", "111\x7f222", "111222"},
		{"empty stays empty", "", ""},
		{"unicode is preserved", "café-eu-west-1", "café-eu-west-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeLogValue(tc.input); got != tc.want {
				t.Errorf("SafeLogValue(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// The property that actually matters, stated independently of any specific
// input: no control character survives.
func TestSafeLogValueLeavesNoControlCharacters(t *testing.T) {
	var sb strings.Builder
	for r := rune(0); r < 0x80; r++ {
		sb.WriteRune(r)
	}

	for _, r := range SafeLogValue(sb.String()) {
		if r < 0x20 || r == 0x7f {
			t.Errorf("control character %#U survived sanitization", r)
		}
	}
}
