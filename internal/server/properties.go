package server

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

// Property describes one setting for help text and the CLI.
type Property struct {
	Key  string
	Env  string
	Help string
}

// PropertyList is every setting minectl manages, in display order. Each maps
// to the itzg/minecraft-server variable that writes it into server.properties.
var PropertyList = []Property{
	{"motd", "MOTD", "message shown in the server list"},
	{"max-players", "MAX_PLAYERS", "player limit (1-1000)"},
	{"difficulty", "DIFFICULTY", "peaceful, easy, normal or hard"},
	{"gamemode", "MODE", "survival, creative, adventure or spectator"},
	{"view-distance", "VIEW_DISTANCE", "chunks sent to players (3-32)"},
	{"pvp", "PVP", "players can hurt each other (true/false)"},
	{"hardcore", "HARDCORE", "one life, hardest difficulty (true/false)"},
	{"online-mode", "ONLINE_MODE", "require real Minecraft accounts (true/false)"},
	{"whitelist", "ENABLE_WHITELIST", "only whitelisted players can join (true/false)"},
	{"allow-flight", "ALLOW_FLIGHT", "don't kick players for flying (true/false)"},
}

var (
	difficulties = []string{"peaceful", "easy", "normal", "hard"}
	gamemodes    = []string{"survival", "creative", "adventure", "spectator"}
)

func oneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// ValidateProperties checks every set field.
func ValidateProperties(p *domain.Properties) error {
	if p == nil {
		return nil
	}
	if len(p.MOTD) > 150 || strings.ContainsAny(p.MOTD, "\r\n\x00") {
		return fmt.Errorf("motd must be one line of up to 150 characters")
	}
	if p.MaxPlayers != 0 && (p.MaxPlayers < 1 || p.MaxPlayers > 1000) {
		return fmt.Errorf("max-players must be between 1 and 1000")
	}
	if p.Difficulty != "" && !oneOf(p.Difficulty, difficulties) {
		return fmt.Errorf("difficulty must be one of: %s", strings.Join(difficulties, ", "))
	}
	if p.Gamemode != "" && !oneOf(p.Gamemode, gamemodes) {
		return fmt.Errorf("gamemode must be one of: %s", strings.Join(gamemodes, ", "))
	}
	if p.ViewDistance != 0 && (p.ViewDistance < 3 || p.ViewDistance > 32) {
		return fmt.Errorf("view-distance must be between 3 and 32")
	}
	return nil
}

func propertyEnv(p *domain.Properties) []string {
	if p == nil {
		return nil
	}
	var env []string
	add := func(k, v string) { env = append(env, k+"="+v) }
	if p.MOTD != "" {
		add("MOTD", p.MOTD)
	}
	if p.MaxPlayers > 0 {
		add("MAX_PLAYERS", strconv.Itoa(p.MaxPlayers))
	}
	if p.Difficulty != "" {
		add("DIFFICULTY", p.Difficulty)
	}
	if p.Gamemode != "" {
		add("MODE", p.Gamemode)
	}
	if p.ViewDistance > 0 {
		add("VIEW_DISTANCE", strconv.Itoa(p.ViewDistance))
	}
	for _, b := range []struct {
		env string
		v   *bool
	}{{"PVP", p.PVP}, {"HARDCORE", p.Hardcore}, {"ONLINE_MODE", p.OnlineMode}, {"ENABLE_WHITELIST", p.Whitelist}, {"ALLOW_FLIGHT", p.AllowFlight}} {
		if b.v != nil {
			add(b.env, strconv.FormatBool(*b.v))
		}
	}
	return env
}

// SetProperty sets one setting from its CLI key; an empty value unsets it.
func SetProperty(p *domain.Properties, key, value string) error {
	key = strings.ToLower(strings.ReplaceAll(key, "_", "-"))
	value = strings.TrimSpace(value)
	num := func() (int, error) {
		if value == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("%s needs a number", key)
		}
		return n, nil
	}
	flag := func() (*bool, error) {
		if value == "" {
			return nil, nil
		}
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("%s needs true or false", key)
		}
		return &b, nil
	}
	var err error
	switch key {
	case "motd":
		p.MOTD = value
	case "max-players":
		p.MaxPlayers, err = num()
	case "difficulty":
		p.Difficulty = strings.ToLower(value)
	case "gamemode", "mode":
		p.Gamemode = strings.ToLower(value)
	case "view-distance":
		p.ViewDistance, err = num()
	case "pvp":
		p.PVP, err = flag()
	case "hardcore":
		p.Hardcore, err = flag()
	case "online-mode":
		p.OnlineMode, err = flag()
	case "whitelist":
		p.Whitelist, err = flag()
	case "allow-flight":
		p.AllowFlight, err = flag()
	default:
		keys := make([]string, 0, len(PropertyList))
		for _, pr := range PropertyList {
			keys = append(keys, pr.Key)
		}
		sort.Strings(keys)
		return fmt.Errorf("unknown setting %q; use one of: %s", key, strings.Join(keys, ", "))
	}
	if err != nil {
		return err
	}
	return ValidateProperties(p)
}

// PropertyValues returns the set values by CLI key, for display.
func PropertyValues(p *domain.Properties) map[string]string {
	out := map[string]string{}
	for _, e := range propertyEnv(p) {
		k, v, _ := strings.Cut(e, "=")
		for _, pr := range PropertyList {
			if pr.Env == k {
				out[pr.Key] = v
			}
		}
	}
	return out
}
