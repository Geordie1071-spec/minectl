package config

import "testing"

func TestImageTagForMCVersion(t *testing.T) {
	cases := map[string]string{
		"latest": "latest",
		"1.12.2": "java8",
		"1.16.5": "java8",
		"1.17.1": "java16",
		"1.18.2": "java17",
		"1.20.4": "java17",
		"1.21":   "java21",
		"1.21.1": "java21",
		"26.1":   "java25",
		"26.3":   "java25",
	}
	for version, want := range cases {
		if got := ImageTagForMCVersion(version, "paper"); got != want {
			t.Errorf("ImageTagForMCVersion(%q) = %q, want %q", version, got, want)
		}
	}
}
