package server

import (
	"reflect"
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
