package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Players struct {
	Online int      `json:"online"`
	Max    int      `json:"max"`
	Names  []string `json:"names"`
}

var (
	listOfMax = regexp.MustCompile(`There are (\d+) of a max(?: of)? (\d+) players online:?((?s).*)`)
	listSlash = regexp.MustCompile(`There are (\d+)/(\d+) players online:?((?s).*)`)
	colorCode = regexp.MustCompile(`§.`)
)

// ParsePlayers reads the output of the "list" console command.
func ParsePlayers(out string) (Players, bool) {
	out = colorCode.ReplaceAllString(strings.TrimSpace(out), "")
	m := listOfMax.FindStringSubmatch(out)
	if m == nil {
		m = listSlash.FindStringSubmatch(out)
	}
	if m == nil {
		return Players{Names: []string{}}, false
	}
	p := Players{Names: []string{}}
	p.Online, _ = strconv.Atoi(m[1])
	p.Max, _ = strconv.Atoi(m[2])
	for _, n := range strings.FieldsFunc(m[3], func(r rune) bool { return r == ',' || r == '\n' }) {
		if n = strings.TrimSpace(n); n != "" {
			p.Names = append(p.Names, n)
		}
	}
	return p, true
}

// KnownPlayer is someone who has joined the server, or is on its op,
// whitelist or ban lists, read from the files the server keeps.
type KnownPlayer struct {
	Name        string     `json:"name"`
	UUID        string     `json:"uuid,omitempty"`
	Op          bool       `json:"op"`
	Whitelisted bool       `json:"whitelisted"`
	Banned      bool       `json:"banned"`
	BanReason   string     `json:"ban_reason,omitempty"`
	LastSeen    *time.Time `json:"last_seen,omitempty"`
}

// The server's own timestamp format in these files.
const mcTime = "2006-01-02 15:04:05 -0700"

// KnownPlayers reads usercache.json, ops.json, whitelist.json and
// banned-players.json from a server's data folder. Missing files are fine:
// a new server has none. Sorted by name.
func KnownPlayers(dataDir string) ([]KnownPlayer, error) {
	type entry struct {
		UUID      string `json:"uuid"`
		Name      string `json:"name"`
		ExpiresOn string `json:"expiresOn"`
		Reason    string `json:"reason"`
	}
	read := func(file string) ([]entry, error) {
		data, err := os.ReadFile(filepath.Join(dataDir, file))
		if errors.Is(err, fs.ErrNotExist) || len(strings.TrimSpace(string(data))) == 0 {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		var out []entry
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		return out, nil
	}
	byName := map[string]*KnownPlayer{}
	get := func(e entry) *KnownPlayer {
		key := strings.ToLower(e.Name)
		p := byName[key]
		if p == nil {
			p = &KnownPlayer{Name: e.Name, UUID: e.UUID}
			byName[key] = p
		}
		return p
	}
	cache, err := read("usercache.json")
	if err != nil {
		return nil, err
	}
	for _, e := range cache {
		p := get(e)
		// The cache keeps an entry for a month after the player was last seen.
		if t, err := time.Parse(mcTime, e.ExpiresOn); err == nil {
			seen := t.AddDate(0, -1, 0).UTC()
			p.LastSeen = &seen
		}
	}
	for file, set := range map[string]func(*KnownPlayer, entry){
		"ops.json":            func(p *KnownPlayer, _ entry) { p.Op = true },
		"whitelist.json":      func(p *KnownPlayer, _ entry) { p.Whitelisted = true },
		"banned-players.json": func(p *KnownPlayer, e entry) { p.Banned, p.BanReason = true, e.Reason },
	} {
		list, err := read(file)
		if err != nil {
			return nil, err
		}
		for _, e := range list {
			if e.Name != "" {
				set(get(e), e)
			}
		}
	}
	out := make([]KnownPlayer, 0, len(byName))
	for _, p := range byName {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}
