package cli

import (
	"strings"
	"testing"
)

func TestParseMemory(t *testing.T) {
	cases := map[string]int{
		"2G":   2048,
		"4g":   4096,
		"512M": 512,
		"4":    4096,
		"4096": 4096,
	}
	for in, want := range cases {
		got, err := parseMemory(in)
		if err != nil {
			t.Errorf("parseMemory(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseMemory(%q) = %d, want %d", in, got, want)
		}
	}
	if _, err := parseMemory("lots"); err == nil {
		t.Error("expected error for invalid memory")
	}
}

func TestConfirm(t *testing.T) {
	for in, want := range map[string]bool{"yes\n": true, "YES\n": true, "y\n": false, "no\n": false, "\n": false, "yes": true} {
		got, err := confirm(strings.NewReader(in), "")
		if err != nil || got != want {
			t.Errorf("confirm(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}
