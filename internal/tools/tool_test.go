package tools

import "testing"

func TestArgString(t *testing.T) {
	args := map[string]any{
		"s":   "hello",
		"n":   42.0,
		"b":   true,
		"nil": nil,
	}
	cases := map[string]string{
		"s":       "hello",
		"n":       "42",
		"b":       "true",
		"nil":     "",
		"missing": "",
	}
	for key, want := range cases {
		if got := argString(args, key); got != want {
			t.Errorf("argString(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestArgBool(t *testing.T) {
	args := map[string]any{"t": true, "f": false, "s": "yes"}
	if !argBool(args, "t", false) {
		t.Error("argBool(t) = false, want true")
	}
	if argBool(args, "f", true) {
		t.Error("argBool(f) = true, want false")
	}
	// Missing key falls back to default.
	if !argBool(args, "missing", true) {
		t.Error("argBool(missing, true) = false, want true (default)")
	}
	// Non-bool value falls back to default.
	if !argBool(args, "s", true) {
		t.Error("argBool(s, true) = false, want true (default)")
	}
}
