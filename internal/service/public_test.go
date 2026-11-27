package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/expose"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

func publicManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := store.New()
	t.Cleanup(func() { st.Close() })
	if err := st.SaveServer(&domain.Server{Name: "survival", Port: 25570}); err != nil {
		t.Fatal(err)
	}
	// Behave like a home machine, whatever network the tests run on.
	old, oldCloud := hosted, cloud
	hosted = func() (net.IP, bool) { return net.ParseIP("192.168.1.20"), false }
	cloud = func() *expose.Cloud { return nil }
	t.Cleanup(func() { hosted, cloud = old, oldCloud })
	return &Manager{St: st}
}

func TestMakePublicOnAVPS(t *testing.T) {
	m := publicManager(t)
	hosted = func() (net.IP, bool) { return net.ParseIP("203.0.113.50"), true }
	stubRouter(t, func(int, string) (*expose.Mapping, error) {
		t.Fatal("a VPS has no router to ask")
		return nil, nil
	})
	_, err := m.MakePublic(context.Background(), "survival", ViaAuto, NopEvents{})
	if err == nil || !strings.Contains(err.Error(), "203.0.113.50:25570") || !strings.Contains(err.Error(), "ufw allow 25570/tcp") {
		t.Fatalf("err = %v", err)
	}
	if s, _ := m.St.GetServer("survival"); s.Public != nil {
		t.Fatal("nothing should be recorded")
	}

	cloud = func() *expose.Cloud { return &expose.Cloud{Provider: "oracle", Name: "Oracle Cloud"} }
	_, err = m.MakePublic(context.Background(), "survival", ViaAuto, NopEvents{})
	if err == nil || !strings.Contains(err.Error(), "open TCP 25570 in the subnet's security list") {
		t.Fatalf("err = %v", err)
	}
}

func stubRouter(t *testing.T, mapFn func(port int, prefer string) (*expose.Mapping, error)) *[]string {
	t.Helper()
	var unmapped []string
	oldMap, oldUnmap := mapPort, unmapPort
	mapPort = func(_ context.Context, port int, _ string, prefer string) (*expose.Mapping, error) {
		return mapFn(port, prefer)
	}
	unmapPort = func(_ context.Context, method string, port int) error {
		unmapped = append(unmapped, method)
		return nil
	}
	t.Cleanup(func() { mapPort, unmapPort = oldMap, oldUnmap })
	return &unmapped
}

func TestMakePublicThroughRouter(t *testing.T) {
	m := publicManager(t)
	ip := "203.0.113.7"
	unmapped := stubRouter(t, func(port int, prefer string) (*expose.Mapping, error) {
		return &expose.Mapping{Method: expose.MethodUPnP, ExternalIP: net.ParseIP(ip), Port: port}, nil
	})
	pub, err := m.MakePublic(context.Background(), "survival", "", NopEvents{})
	if err != nil {
		t.Fatal(err)
	}
	if pub.Method != expose.MethodUPnP || pub.Address != "203.0.113.7:25570" {
		t.Fatalf("public = %+v", pub)
	}
	s, _ := m.St.GetServer("survival")
	if s.Public == nil || s.Public.Address != pub.Address || s.Public.RefreshedAt.IsZero() {
		t.Fatalf("saved public = %+v", s.Public)
	}

	// The router got a new internet address; renewing picks it up.
	ip = "198.51.100.9"
	if err := m.RenewPublic(context.Background(), "survival"); err != nil {
		t.Fatal(err)
	}
	if s, _ = m.St.GetServer("survival"); s.Public.Address != "198.51.100.9:25570" {
		t.Fatalf("renewed address = %s", s.Public.Address)
	}

	if err := m.MakePrivate(context.Background(), "survival", NopEvents{}); err != nil {
		t.Fatal(err)
	}
	if s, _ = m.St.GetServer("survival"); s.Public != nil {
		t.Fatal("still public after MakePrivate")
	}
	if len(*unmapped) != 1 || (*unmapped)[0] != expose.MethodUPnP {
		t.Fatalf("unmapped = %v", *unmapped)
	}
}

func TestMakePublicFallsBackToPlayit(t *testing.T) {
	m := publicManager(t)
	stubRouter(t, func(int, string) (*expose.Mapping, error) {
		return nil, &expose.SharedAddressError{IP: net.ParseIP("100.72.1.2")}
	})
	ctx := context.Background()

	// Without playit linked, the error explains both problems.
	_, err := m.MakePublic(ctx, "survival", ViaAuto, NopEvents{})
	if err == nil || !strings.Contains(err.Error(), "CGNAT") || !strings.Contains(err.Error(), "playit.gg") {
		t.Fatalf("error = %v", err)
	}
	// Asking for the router only doesn't mention playit.
	if _, err = m.MakePublic(ctx, "survival", ViaRouter, NopEvents{}); err == nil || strings.Contains(err.Error(), "playit") {
		t.Fatalf("router-only error = %v", err)
	}

	// Linked, it moves on to the tunnel (which needs Docker for the agent).
	cfg, _ := m.St.GetConfig()
	cfg.PlayitSecret = "sekret"
	_ = m.St.SaveConfig(cfg)
	if _, err = m.MakePublic(ctx, "survival", ViaAuto, NopEvents{}); err == nil || !strings.Contains(err.Error(), "playit.gg agent") {
		t.Fatalf("fallback error = %v", err)
	}
	if s, _ := m.St.GetServer("survival"); s.Public != nil {
		t.Fatal("a failed attempt must not mark the server public")
	}
	if _, err = m.MakePublic(ctx, "survival", "ngrok", NopEvents{}); err == nil {
		t.Fatal("unknown method accepted")
	}
}

func TestPlayitClaim(t *testing.T) {
	m := publicManager(t)
	setups := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		reply := func(status string, data any) {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "data": data})
		}
		switch r.URL.Path {
		case "/claim/setup":
			if body["agent_type"] != "self-managed" || !strings.HasPrefix(body["version"], "minectl ") {
				t.Errorf("setup body %v", body)
			}
			setups++
			if setups == 1 {
				reply("success", expose.ClaimWaitingForUser)
				return
			}
			reply("success", expose.ClaimAccepted)
		case "/claim/exchange":
			reply("success", map[string]string{"secret_key": "the-secret"})
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	old := newPlayit
	newPlayit = func(secret string) *expose.Playit {
		p := old(secret)
		p.Base = srv.URL
		return p
	}
	defer func() { newPlayit = old }()

	ctx := context.Background()
	if _, err := m.PlayitClaim(ctx, "not-hex"); err == nil {
		t.Fatal("bad code accepted")
	}
	code := expose.NewClaimCode()
	if status, err := m.PlayitClaim(ctx, code); err != nil || status != ClaimWaiting {
		t.Fatalf("first poll %q %v", status, err)
	}
	if status, err := m.PlayitClaim(ctx, code); err != nil || status != ClaimLinked {
		t.Fatalf("second poll %q %v", status, err)
	}
	if cfg, _ := m.St.GetConfig(); cfg.PlayitSecret != "the-secret" {
		t.Fatalf("secret not saved: %q", cfg.PlayitSecret)
	}
	if err := m.UnlinkPlayit(ctx, NopEvents{}); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := m.St.GetConfig(); cfg.PlayitSecret != "" {
		t.Fatal("unlink kept the secret")
	}
}

func TestMakePublicWithSavedPublicAddress(t *testing.T) {
	// An Azure or Oracle machine only sees a private IP; the panel's saved
	// --domain tells it it's on the internet anyway.
	m := publicManager(t)
	cfg, _ := m.St.GetConfig()
	cfg.PublicAddress = "mc.example.com"
	_ = m.St.SaveConfig(cfg)
	stubRouter(t, func(int, string) (*expose.Mapping, error) {
		t.Fatal("a cloud machine has no router to ask")
		return nil, nil
	})
	_, err := m.MakePublic(context.Background(), "survival", ViaAuto, NopEvents{})
	if err == nil || !strings.Contains(err.Error(), "mc.example.com:25570") {
		t.Fatalf("err = %v", err)
	}
}
