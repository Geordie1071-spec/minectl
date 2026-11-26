package service

import "testing"

func TestParseMemory(t *testing.T) {
	cases := map[string]int{
		"2G":   2048,
		"4g":   4096,
		"512M": 512,
		"4":    4096,
		"4096": 4096,
	}
	for in, want := range cases {
		got, err := ParseMemory(in)
		if err != nil {
			t.Errorf("ParseMemory(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseMemory(%q) = %d, want %d", in, got, want)
		}
	}
	if _, err := ParseMemory("lots"); err == nil {
		t.Error("expected error for invalid memory")
	}
}
