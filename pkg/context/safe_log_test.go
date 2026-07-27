package context

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
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

		// Filtering only the ASCII range leaves these, and each breaks a log
		// line as effectively as \n does.
		{"U+2028 line separator", "111 forged", "111forged"},
		{"U+2029 paragraph separator", "111 forged", "111forged"},
		{"U+0085 next line", "111forged", "111forged"},
		{"U+009B C1 control sequence introducer", "111forged", "111forged"},
		{"U+200E bidi mark reordering display", "111‎forged", "111forged"},
		{"U+202E right-to-left override", "111‮forged", "111forged"},
		{"U+200B zero-width space", "111​forged", "111forged"},
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
// input: nothing that can break a log line survives. Swept across the whole
// Basic Multilingual Plane rather than just ASCII, since the characters most
// likely to be missed -- U+2028, U+0085, the C1 block -- all live above it.
func TestSafeLogValueLeavesNothingThatBreaksALogLine(t *testing.T) {
	var sb strings.Builder
	for r := rune(0); r <= 0xFFFF; r++ {
		if utf8.ValidRune(r) {
			sb.WriteRune(r)
		}
	}

	for _, r := range SafeLogValue(sb.String()) {
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) ||
			unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			t.Errorf("line-breaking character %#U survived sanitization", r)
		}
	}
}

// Sanitizing must not mangle ordinary account names, or operators lose the
// ability to recognize their own targets in the logs.
func TestSafeLogValuePreservesOrdinaryText(t *testing.T) {
	for _, s := range []string{
		"111111111111",
		"us-east-1",
		"acme-production_eu",
		"café",
		"日本語アカウント",
		"Ünïcödé Näme",
	} {
		if got := SafeLogValue(s); got != s {
			t.Errorf("SafeLogValue(%q) = %q, want it unchanged", s, got)
		}
	}
}
