package expose

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnreachable(t *testing.T) {
	for _, ip := range []string{"192.168.1.1", "10.0.0.5", "172.16.3.4", "100.64.0.1", "100.127.255.254", "127.0.0.1", "0.0.0.0", "169.254.1.1"} {
		if !Unreachable(net.ParseIP(ip)) {
			t.Errorf("%s should be unreachable", ip)
		}
	}
	for _, ip := range []string{"203.0.113.7", "100.63.255.255", "100.128.0.1", "81.2.69.160"} {
		if Unreachable(net.ParseIP(ip)) {
			t.Errorf("%s should be reachable", ip)
		}
	}
	if !Unreachable(nil) {
		t.Error("a missing address is unreachable")
	}
}

func TestClaimCode(t *testing.T) {
	code := NewClaimCode()
	if len(code) != 10 || !ValidClaimCode(code) {
		t.Fatalf("claim code %q", code)
	}
	if ClaimURL(code) != "https://playit.gg/claim/"+code {
		t.Fatalf("claim url %q", ClaimURL(code))
	}
	for _, bad := range []string{"", "xyz", "0123456789ab", "../../etc"} {
		if ValidClaimCode(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

// fakePlayit answers like api.playit.gg: POST only, {"status","data"} replies.
func fakePlayit(t *testing.T, handle func(path string, body map[string]any) (string, any)) *Playit {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("%s %s: playit calls are all POST", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Agent-Key sekret" {
			t.Errorf("Authorization = %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		status, data := handle(r.URL.Path, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "data": data})
	}))
	t.Cleanup(srv.Close)
	p := NewPlayit(" sekret\n")
	p.Base = srv.URL
	return p
}

func TestPlayitTunnels(t *testing.T) {
	var created map[string]any
	p := fakePlayit(t, func(path string, body map[string]any) (string, any) {
		switch path {
		case "/agents/rundata":
			return "success", map[string]any{"agent_id": "agent-1", "account_status": "ready"}
		case "/tunnels/create":
			created = body
			return "success", map[string]any{"id": "t-new"}
		case "/tunnels/list":
			if body["agent_id"] != "agent-1" {
				t.Errorf("list body %v", body)
			}
			return "success", map[string]any{"tunnels": []any{
				map[string]any{
					"id":     "t-1",
					"alloc":  map[string]any{"status": "allocated", "data": map[string]any{"assigned_domain": "abc.joinmc.link", "port_start": 41234}},
					"origin": map[string]any{"type": "agent", "data": map[string]any{"agent_id": "agent-1", "local_port": 25565}},
				},
				map[string]any{"id": "t-2", "alloc": map[string]any{"status": "pending"}, "origin": nil},
				map[string]any{"id": "t-3", "alloc": map[string]any{"status": "disabled", "data": map[string]any{"reason": "over-port-limit"}}},
			}}
		case "/tunnels/delete":
			return "fail", "TunnelNotFound"
		}
		t.Errorf("unexpected call %s", path)
		return "error", nil
	})
	ctx := context.Background()
	id, err := p.AgentID(ctx)
	if err != nil || id != "agent-1" {
		t.Fatalf("agent id %q %v", id, err)
	}
	tunnels, err := p.Tunnels(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := []Tunnel{
		{ID: "t-1", LocalPort: 25565, Address: "abc.joinmc.link:41234"},
		{ID: "t-2"},
		{ID: "t-3", Disabled: "over-port-limit"},
	}
	if len(tunnels) != len(want) {
		t.Fatalf("tunnels = %+v", tunnels)
	}
	for i := range want {
		if tunnels[i] != want[i] {
			t.Errorf("tunnel %d = %+v, want %+v", i, tunnels[i], want[i])
		}
	}
	if tid, err := p.CreateTunnel(ctx, "agent-1", "minectl-survival", 25570); err != nil || tid != "t-new" {
		t.Fatalf("create %q %v", tid, err)
	}
	origin := created["origin"].(map[string]any)
	data := origin["data"].(map[string]any)
	if created["tunnel_type"] != "minecraft-java" || created["port_type"] != "tcp" || origin["type"] != "agent" ||
		data["local_ip"] != "127.0.0.1" || data["local_port"] != float64(25570) || data["agent_id"] != "agent-1" {
		t.Errorf("create body %v", created)
	}
	if err := p.DeleteTunnel(ctx, "gone"); err != nil {
		t.Errorf("deleting a missing tunnel should be fine: %v", err)
	}
}

func TestPlayitErrors(t *testing.T) {
	p := fakePlayit(t, func(path string, body map[string]any) (string, any) {
		switch path {
		case "/agents/rundata":
			return "error", map[string]any{"type": "auth", "message": "InvalidAgentKey"}
		case "/tunnels/create":
			return "fail", "RequiresVerifiedAccount"
		}
		return "fail", "CodeExpired"
	})
	ctx := context.Background()
	if _, err := p.AgentID(ctx); err == nil || !strings.Contains(err.Error(), "link playit again") {
		t.Errorf("auth error: %v", err)
	}
	if _, err := p.CreateTunnel(ctx, "a", "n", 1); err == nil || !strings.Contains(err.Error(), "verify your account") {
		t.Errorf("create error: %v", err)
	}
	_, err := p.ClaimExchange(ctx, "0011223344")
	var fail *PlayitFail
	if !errors.As(err, &fail) || fail.Reason != "CodeExpired" {
		t.Errorf("fail reply: %v", err)
	}
}
