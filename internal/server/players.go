package server

import (
	"regexp"
	"strconv"
	"strings"
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
