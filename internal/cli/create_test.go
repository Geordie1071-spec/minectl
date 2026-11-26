package cli

import (
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	for in, want := range map[string]bool{"yes\n": true, "YES\n": true, "y\n": false, "no\n": false, "\n": false, "yes": true} {
		got, err := confirm(strings.NewReader(in), "")
		if err != nil || got != want {
			t.Errorf("confirm(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}
