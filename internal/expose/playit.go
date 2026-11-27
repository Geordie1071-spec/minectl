package expose

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// PlayitAPI is the playit.gg control API. The wire format follows the
// official agent's api_client crate: every call is a POST with a JSON body,
// and replies are {"status": "success"|"fail"|"error", "data": ...}.
const PlayitAPI = "https://api.playit.gg"

// PlayitImage is the official agent, run next to the servers with host
// networking so it can reach them on 127.0.0.1.
const PlayitImage = "ghcr.io/playit-cloud/playit-agent:latest"

// Claim statuses from /claim/setup.
const (
	ClaimWaitingForVisit = "WaitingForUserVisit"
	ClaimWaitingForUser  = "WaitingForUser"
	ClaimAccepted        = "UserAccepted"
	ClaimRejected        = "UserRejected"
)

type Playit struct {
	Base   string
	Secret string
	HTTP   *http.Client
}

func NewPlayit(secret string) *Playit {
	return &Playit{Base: PlayitAPI, Secret: secret, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// PlayitFail is a "fail" reply: an expected error such as "CodeExpired".
type PlayitFail struct {
	Path   string
	Reason string
}

func (e *PlayitFail) Error() string {
	return fmt.Sprintf("playit.gg %s: %s", e.Path, e.Reason)
}

func (p *Playit) call(ctx context.Context, path string, req, out any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	if p.Secret != "" {
		r.Header.Set("Authorization", "Agent-Key "+strings.TrimSpace(p.Secret))
	}
	resp, err := p.HTTP.Do(r)
	if err != nil {
		return fmt.Errorf("couldn't reach playit.gg: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("playit.gg %s: too many requests, try again in a minute", path)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("playit.gg %s: unexpected reply (HTTP %d)", path, resp.StatusCode)
	}
	switch env.Status {
	case "success":
		if out == nil {
			return nil
		}
		return json.Unmarshal(env.Data, out)
	case "fail":
		return &PlayitFail{Path: path, Reason: reason(env.Data)}
	default:
		var e struct {
			Type    string          `json:"type"`
			Message json.RawMessage `json:"message"`
		}
		_ = json.Unmarshal(env.Data, &e)
		if e.Type == "auth" {
			return fmt.Errorf("playit.gg rejected minectl's key (%s); link playit again", reason(e.Message))
		}
		return fmt.Errorf("playit.gg %s: %s %s", path, e.Type, reason(e.Message))
	}
}

// reason turns a fail payload into text; it's usually a bare enum string.
func reason(data json.RawMessage) string {
	var s string
	if json.Unmarshal(data, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(data))
}

// NewClaimCode returns a code for linking an agent: 5 random bytes as hex, the
// same as the official agent.
func NewClaimCode() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ClaimURL is where the user approves the agent.
func ClaimURL(code string) string {
	return "https://playit.gg/claim/" + code
}

// ValidClaimCode checks a code came from NewClaimCode.
func ValidClaimCode(code string) bool {
	b, err := hex.DecodeString(code)
	return err == nil && len(b) == 5
}

// ClaimSetup registers the code and reports how far the user has got.
func (p *Playit) ClaimSetup(ctx context.Context, code, version string) (string, error) {
	var status string
	err := p.call(ctx, "/claim/setup", map[string]string{
		"code": code, "agent_type": "self-managed", "version": "minectl " + version,
	}, &status)
	return status, err
}

// ClaimExchange trades an accepted code for the agent's secret key.
func (p *Playit) ClaimExchange(ctx context.Context, code string) (string, error) {
	var out struct {
		SecretKey string `json:"secret_key"`
	}
	if err := p.call(ctx, "/claim/exchange", map[string]string{"code": code}, &out); err != nil {
		return "", err
	}
	return out.SecretKey, nil
}

// AgentID returns the id of the agent the secret belongs to.
func (p *Playit) AgentID(ctx context.Context) (string, error) {
	var out struct {
		AgentID       string `json:"agent_id"`
		AccountStatus string `json:"account_status"`
	}
	if err := p.call(ctx, "/agents/rundata", struct{}{}, &out); err != nil {
		return "", err
	}
	switch out.AccountStatus {
	case "banned", "agent-disabled", "account-delete-scheduled":
		return "", fmt.Errorf("playit.gg says this account can't be used (%s)", out.AccountStatus)
	case "email-not-verified":
		return "", errors.New("verify your email address on playit.gg first")
	}
	return out.AgentID, nil
}

// Tunnel is a playit tunnel to a local port.
type Tunnel struct {
	ID        string
	LocalPort int
	// Address is empty until playit has given the tunnel a public address.
	Address string
	// Disabled is playit's reason when it won't run the tunnel.
	Disabled string
}

type accountTunnel struct {
	ID    string `json:"id"`
	Alloc struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	} `json:"alloc"`
	Origin *struct {
		Type string `json:"type"`
		Data struct {
			AgentID   string `json:"agent_id"`
			LocalPort *int   `json:"local_port"`
		} `json:"data"`
	} `json:"origin"`
}

func (t accountTunnel) tunnel() Tunnel {
	out := Tunnel{ID: t.ID}
	if t.Origin != nil && t.Origin.Data.LocalPort != nil {
		out.LocalPort = *t.Origin.Data.LocalPort
	}
	switch t.Alloc.Status {
	case "allocated":
		var a struct {
			AssignedDomain string `json:"assigned_domain"`
			PortStart      int    `json:"port_start"`
		}
		if json.Unmarshal(t.Alloc.Data, &a) == nil && a.AssignedDomain != "" {
			out.Address = net.JoinHostPort(a.AssignedDomain, fmt.Sprint(a.PortStart))
		}
	case "disabled":
		var d struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(t.Alloc.Data, &d)
		out.Disabled = d.Reason
		if out.Disabled == "" {
			out.Disabled = "disabled"
		}
	}
	return out
}

// Tunnels lists the agent's tunnels.
func (p *Playit) Tunnels(ctx context.Context, agentID string) ([]Tunnel, error) {
	var out struct {
		Tunnels []accountTunnel `json:"tunnels"`
	}
	if err := p.call(ctx, "/tunnels/list", map[string]any{"tunnel_id": nil, "agent_id": agentID}, &out); err != nil {
		return nil, err
	}
	tunnels := make([]Tunnel, 0, len(out.Tunnels))
	for _, t := range out.Tunnels {
		tunnels = append(tunnels, t.tunnel())
	}
	return tunnels, nil
}

// CreateTunnel opens a Minecraft Java tunnel to 127.0.0.1:localPort.
func (p *Playit) CreateTunnel(ctx context.Context, agentID, name string, localPort int) (string, error) {
	req := map[string]any{
		"name":        name,
		"tunnel_type": "minecraft-java",
		"port_type":   "tcp",
		"port_count":  1,
		"origin": map[string]any{
			"type": "agent",
			"data": map[string]any{"agent_id": agentID, "local_ip": "127.0.0.1", "local_port": localPort},
		},
		"enabled":        true,
		"alloc":          nil,
		"firewall_id":    nil,
		"proxy_protocol": nil,
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := p.call(ctx, "/tunnels/create", req, &out); err != nil {
		var fail *PlayitFail
		if errors.As(err, &fail) && fail.Reason == "RequiresVerifiedAccount" {
			return "", errors.New("playit.gg needs you to verify your account before it will make a tunnel")
		}
		return "", err
	}
	return out.ID, nil
}

// DeleteTunnel removes a tunnel; one that's already gone is fine.
func (p *Playit) DeleteTunnel(ctx context.Context, id string) error {
	err := p.call(ctx, "/tunnels/delete", map[string]string{"tunnel_id": id}, nil)
	var fail *PlayitFail
	if errors.As(err, &fail) && fail.Reason == "TunnelNotFound" {
		return nil
	}
	return err
}
