package server

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParsePlayers(t *testing.T) {
	cases := []struct {
		in   string
		want Players
		ok   bool
	}{
		{"There are 0 of a max of 20 players online: ", Players{0, 20, []string{}}, true},
		{"There are 2 of a max of 20 players online: Steve, Alex", Players{2, 20, []string{"Steve", "Alex"}}, true},
		{"There are 1 of a max 10 players online: Notch", Players{1, 10, []string{"Notch"}}, true},
		{"There are 1/8 players online:\nJeb_", Players{1, 8, []string{"Jeb_"}}, true},
		{"§6There are §c1§6 of a max of §c20§6 players online: §fSteve", Players{1, 20, []string{"Steve"}}, true},
		{"Unknown command", Players{Names: []string{}}, false},
	}
	for _, c := range cases {
		got, ok := ParsePlayers(c.in)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParsePlayers(%q) = %+v, %v; want %+v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestKnownPlayers(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if list, err := KnownPlayers(dir); err != nil || len(list) != 0 {
		t.Fatalf("a new server has nobody: %v %v", list, err)
	}
	write("usercache.json", `[{"uuid":"u1","name":"GMONEY7110","expiresOn":"2026-12-27 11:48:24 +0000"},{"uuid":"u2","name":"alex","expiresOn":"2026-12-01 09:00:00 +0000"}]`)
	write("ops.json", `[{"uuid":"u2","name":"alex","level":4}]`)
	write("whitelist.json", `[{"uuid":"u3","name":"NeverJoined"}]`)
	write("banned-players.json", `[
  {"uuid":"u1","name":"GMONEY7110","created":"2026-11-27 11:48:30 +0000","source":"Rcon","expires":"forever","reason":"griefing"}
]`)
	list, err := KnownPlayers(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Name != "alex" || list[1].Name != "GMONEY7110" || list[2].Name != "NeverJoined" {
		t.Fatalf("players = %+v", list)
	}
	alex, gm, nj := list[0], list[1], list[2]
	if !alex.Op || alex.Banned || alex.LastSeen == nil || alex.LastSeen.Format("2006-01-02") != "2026-11-01" {
		t.Errorf("alex = %+v", alex)
	}
	if !gm.Banned || gm.BanReason != "griefing" || gm.LastSeen.Format("2006-01-02 15:04") != "2026-11-27 11:48" {
		t.Errorf("GMONEY7110 = %+v", gm)
	}
	if !nj.Whitelisted || nj.LastSeen != nil {
		t.Errorf("NeverJoined = %+v", nj)
	}
	write("ops.json", `not json`)
	if _, err := KnownPlayers(dir); err == nil || !strings.Contains(err.Error(), "ops.json") {
		t.Fatalf("broken file: %v", err)
	}
}
