package cli

import (
	"strings"
	"testing"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

func TestListJoinAddress(t *testing.T) {
	servers := []domain.Server{
		{Name: "survival", Port: 25565, Status: domain.StatusRunning, MCType: "paper", MCVersion: "26.3", MemoryMB: 2048},
		{Name: "creative", Port: 25566, Status: domain.StatusStopped, MCType: "paper", MCVersion: "26.3", MemoryMB: 1536,
			Public: &domain.Public{Method: "playit", Address: "abc.joinmc.link:41234"}},
	}
	// On a cloud machine the panel's public address beats the private IP.
	out := renderListTable(servers, false, "mc.example.com")
	for _, want := range []string{"mc.example.com:25565", "abc.joinmc.link:41234", "2 GB", "1.5 GB", "2 servers · 1 running"} {
		if !strings.Contains(out, want) {
			t.Errorf("list missing %q:\n%s", want, out)
		}
	}
	if out := renderListTable(nil, false, ""); !strings.Contains(out, "No servers yet") {
		t.Errorf("empty list: %q", out)
	}
}

func TestSavePublicAddress(t *testing.T) {
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := getStore()
	defer st.Close()
	for _, c := range []struct{ public, domain, want string }{
		{"", "panel.example.com", "panel.example.com"},
		{"play.example.com", "panel.example.com", "play.example.com"},
		{"", "", ""}, // starting without either clears it
	} {
		if err := savePublicAddress(st, c.public, c.domain); err != nil {
			t.Fatal(err)
		}
		if cfg, _ := st.GetConfig(); cfg.PublicAddress != c.want {
			t.Errorf("public=%q domain=%q: saved %q, want %q", c.public, c.domain, cfg.PublicAddress, c.want)
		}
	}
}
