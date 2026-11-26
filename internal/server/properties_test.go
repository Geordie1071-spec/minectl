package server

import (
	"strings"
	"testing"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

func TestSetPropertyAndEnv(t *testing.T) {
	p := &domain.Properties{}
	for _, kv := range [][2]string{
		{"motd", "§aWelcome to survival"},
		{"max_players", "40"},
		{"difficulty", "Hard"},
		{"gamemode", "creative"},
		{"view-distance", "12"},
		{"pvp", "false"},
		{"whitelist", "true"},
	} {
		if err := SetProperty(p, kv[0], kv[1]); err != nil {
			t.Fatalf("%s=%s: %v", kv[0], kv[1], err)
		}
	}
	env := envMap(BuildEnvVars(&domain.Server{MCType: "paper", MCVersion: "26.3", Properties: p}))
	want := map[string]string{
		"MOTD": "§aWelcome to survival", "MAX_PLAYERS": "40", "DIFFICULTY": "hard", "MODE": "creative",
		"VIEW_DISTANCE": "12", "PVP": "false", "ENABLE_WHITELIST": "true",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
	if _, ok := env["HARDCORE"]; ok {
		t.Error("unset settings must not be sent")
	}
	if got := PropertyValues(p); got["max-players"] != "40" || got["pvp"] != "false" || len(got) != 7 {
		t.Errorf("values = %v", got)
	}

	if err := SetProperty(p, "pvp", ""); err != nil || p.PVP != nil {
		t.Fatalf("unset pvp: %v %v", err, p.PVP)
	}
}

func TestSetPropertyRejects(t *testing.T) {
	for _, kv := range [][2]string{
		{"difficulty", "impossible"},
		{"gamemode", "god"},
		{"max-players", "0x10"},
		{"max-players", "5000"},
		{"view-distance", "64"},
		{"pvp", "maybe"},
		{"motd", "two\nlines"},
		{"motd", strings.Repeat("x", 151)},
		{"spawn-protection", "0"},
	} {
		if err := SetProperty(&domain.Properties{}, kv[0], kv[1]); err == nil {
			t.Errorf("%s=%q should be rejected", kv[0], kv[1])
		}
	}
}
