package main

import "testing"

func TestDisplayVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	for input, expected := range map[string]string{
		"dev":    "dev",
		"v1.2.3": "v1.2.3",
		"1.2.3":  "v1.2.3",
	} {
		version = input
		if got := displayVersion(); got != expected {
			t.Errorf("displayVersion() with %q = %q; want %q", input, got, expected)
		}
	}
}
