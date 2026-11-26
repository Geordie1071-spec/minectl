package service

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"survival", "e2e-123", "a", "bettermc-2", strings.Repeat("a", 32)} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Survival", "-lead", "has space", "../etc", "a_b", "x/y", strings.Repeat("a", 33)} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) should fail", bad)
		}
	}
}

func TestStartFailedErrorMessage(t *testing.T) {
	err := &StartFailedError{
		Server:  "backrooms",
		Hint:    `the mod "spb-revamped" uses client-only code`,
		Summary: []string{"[main/ERROR]: Minecraft has crashed!"},
		LogPath: "/srv/backrooms/logs/latest.log",
		Cleanup: "Stopped backrooms so it doesn't keep restarting.",
	}
	msg := err.Error()
	for _, want := range []string{"backrooms failed to start: the mod", "Key log lines:\n  [main/ERROR]", "Full log: /srv/backrooms/logs/latest.log", "Stopped backrooms"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}
