package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/expose"
)

// How MakePublic reaches the internet.
const (
	ViaAuto   = "auto"
	ViaRouter = "router"
	ViaPlayit = "playit"
)

// Claim results from PlayitClaim.
const (
	ClaimWaiting  = "waiting"
	ClaimLinked   = "linked"
	ClaimRejected = "rejected"
)

// Replaced in tests.
var (
	mapPort    = expose.MapPort
	unmapPort  = expose.UnmapPort
	newPlayit  = expose.NewPlayit
	tunnelWait = 90 * time.Second
	tunnelPoll = 3 * time.Second
)

func (m *Manager) playitHint() string {
	if m.CLIHints {
		return "Link a free playit.gg account to use a tunnel instead: minectl playit link"
	}
	return "Link a free playit.gg account under Panel settings to use a tunnel instead."
}

// MakePublic makes a server joinable from the internet. With ViaAuto it asks
// the router to forward the port and falls back to a playit.gg tunnel.
func (m *Manager) MakePublic(ctx context.Context, name, via string, ev Events) (*domain.Public, error) {
	if via == "" {
		via = ViaAuto
	}
	if via != ViaAuto && via != ViaRouter && via != ViaPlayit {
		return nil, fmt.Errorf("invalid method %q; use auto, router, or playit", via)
	}
	unlock, err := m.lock(ctx, name, "make public")
	if err != nil {
		return nil, err
	}
	defer unlock()
	s, err := m.get(name)
	if err != nil {
		return nil, err
	}
	prev := ""
	if s.Public != nil {
		prev = s.Public.Method
		// Switching between router and tunnel: undo the old one first.
		if (via == ViaPlayit) != (prev == expose.MethodPlayit) && via != ViaAuto {
			m.unexpose(ctx, s, ev)
			prev = ""
			if err := m.St.SaveServer(s); err != nil {
				return nil, err
			}
		}
	}

	var pub *domain.Public
	var routerErr error
	if via != ViaPlayit && !(via == ViaAuto && prev == expose.MethodPlayit) {
		ev.Stage(fmt.Sprintf("Asking your router to forward port %d", s.Port))
		mp, err := mapPort(ctx, s.Port, "minectl "+name, prev)
		switch {
		case err == nil:
			pub = &domain.Public{Method: mp.Method, Address: mp.Address()}
		case via == ViaRouter:
			return nil, err
		default:
			routerErr = err
			ev.Warn("Port forwarding didn't work: " + err.Error())
		}
	}
	if pub == nil {
		cfg, err := m.St.GetConfig()
		if err != nil {
			return nil, err
		}
		if cfg.PlayitSecret == "" {
			if routerErr != nil {
				return nil, fmt.Errorf("%w\n\n%s", routerErr, m.playitHint())
			}
			return nil, errors.New("playit.gg isn't linked. " + m.playitHint())
		}
		ev.Stage("Opening a playit.gg tunnel")
		if pub, err = m.openTunnel(ctx, s, cfg.PlayitSecret, ev); err != nil {
			return nil, err
		}
	}
	pub.RefreshedAt = time.Now().UTC()
	s.Public = pub
	if err := m.St.SaveServer(s); err != nil {
		return nil, err
	}
	ev.Info("Friends can join at " + pub.Address)
	if pub.Method != expose.MethodPlayit {
		local := "this machine's local address"
		if ip := expose.LANIP(); ip != "" {
			local = fmt.Sprintf("%s:%d", ip, s.Port)
		}
		ev.Warn(fmt.Sprintf("that address may not work from your own network: most home routers can't connect to their own public address. At home, join with %s.", local))
	}
	return pub, nil
}

func (m *Manager) openTunnel(ctx context.Context, s *domain.Server, secret string, ev Events) (*domain.Public, error) {
	if m.D == nil {
		return nil, errors.New("Docker isn't reachable, so the playit.gg agent can't start")
	}
	ev.Info("Starting the playit.gg agent")
	if err := m.D.EnsurePlayitAgent(ctx, expose.PlayitImage, secret); err != nil {
		return nil, fmt.Errorf("start the playit.gg agent: %w", err)
	}
	p := newPlayit(secret)
	agentID, err := p.AgentID(ctx)
	if err != nil {
		return nil, err
	}
	tunnels, err := p.Tunnels(ctx, agentID)
	if err != nil {
		return nil, err
	}
	id := ""
	for _, t := range tunnels {
		if (s.Public != nil && t.ID == s.Public.TunnelID) || t.LocalPort == s.Port {
			id = t.ID
			break
		}
	}
	if id == "" {
		if id, err = p.CreateTunnel(ctx, agentID, "minectl-"+s.Name, s.Port); err != nil {
			return nil, err
		}
	}
	ev.Info("Waiting for playit.gg to assign an address")
	deadline := time.Now().Add(tunnelWait)
	for {
		tunnels, err := p.Tunnels(ctx, agentID)
		if err != nil {
			return nil, err
		}
		for _, t := range tunnels {
			if t.ID != id {
				continue
			}
			if t.Disabled != "" {
				return nil, fmt.Errorf("playit.gg won't run the tunnel: %s", t.Disabled)
			}
			if t.Address != "" {
				return &domain.Public{Method: expose.MethodPlayit, Address: t.Address, TunnelID: id}, nil
			}
		}
		if time.Now().After(deadline) {
			return nil, errors.New("playit.gg is still setting up the tunnel; try again in a minute")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(tunnelPoll):
		}
	}
}

// MakePrivate removes the port forward or tunnel.
func (m *Manager) MakePrivate(ctx context.Context, name string, ev Events) error {
	unlock, err := m.lock(ctx, name, "make private")
	if err != nil {
		return err
	}
	defer unlock()
	s, err := m.get(name)
	if err != nil {
		return err
	}
	if s.Public == nil {
		return nil
	}
	m.unexpose(ctx, s, ev)
	return m.St.SaveServer(s)
}

// unexpose undoes s.Public as far as it can and clears it. Failures are only
// warnings: the router or playit.gg may be unreachable, and the server should
// still stop claiming to be public.
func (m *Manager) unexpose(ctx context.Context, s *domain.Server, ev Events) {
	pub := s.Public
	if pub == nil {
		return
	}
	s.Public = nil
	if pub.Method != expose.MethodPlayit {
		if err := unmapPort(ctx, pub.Method, s.Port); err != nil {
			ev.Warn(fmt.Sprintf("couldn't remove the port forward for %d from your router: %v", s.Port, err))
		}
		return
	}
	cfg, err := m.St.GetConfig()
	if err == nil && cfg.PlayitSecret != "" && pub.TunnelID != "" {
		if err := newPlayit(cfg.PlayitSecret).DeleteTunnel(ctx, pub.TunnelID); err != nil {
			ev.Warn("couldn't delete the playit.gg tunnel: " + err.Error())
		}
	}
	if m.D != nil && !m.otherTunnels(s.Name) {
		if err := m.D.RemovePlayitAgent(ctx); err != nil {
			ev.Warn("couldn't stop the playit.gg agent: " + err.Error())
		}
	}
}

// otherTunnels reports whether a server other than name uses a playit tunnel.
func (m *Manager) otherTunnels(name string) bool {
	servers, err := m.St.ListServers()
	if err != nil {
		return true
	}
	for _, o := range servers {
		if o.Name != name && o.Public != nil && o.Public.Method == expose.MethodPlayit {
			return true
		}
	}
	return false
}

// RenewPublic re-adds a router port forward, which also restores it after the
// router restarts and picks up a new internet address. Tunnels need nothing.
// A server busy with another job is skipped.
func (m *Manager) RenewPublic(ctx context.Context, name string) error {
	unlock, err := m.lock(ctx, name, "renew port forward")
	if err != nil {
		return err
	}
	defer unlock()
	s, err := m.get(name)
	if err != nil {
		return err
	}
	if s.Public == nil || s.Public.Method == expose.MethodPlayit {
		return nil
	}
	mp, err := mapPort(ctx, s.Port, "minectl "+name, s.Public.Method)
	if err != nil {
		return err
	}
	s.Public.Method = mp.Method
	s.Public.Address = mp.Address()
	s.Public.RefreshedAt = time.Now().UTC()
	return m.St.SaveServer(s)
}

// PlayitClaim checks on a claim code from expose.NewClaimCode. Once the user
// approves it on playit.gg, the agent key is saved and it returns ClaimLinked.
func (m *Manager) PlayitClaim(ctx context.Context, code string) (string, error) {
	if !expose.ValidClaimCode(code) {
		return "", errors.New("invalid claim code")
	}
	p := newPlayit("")
	status, err := p.ClaimSetup(ctx, code, config.Version)
	if err != nil {
		return "", err
	}
	switch status {
	case expose.ClaimRejected:
		return ClaimRejected, nil
	case expose.ClaimAccepted:
	default:
		return ClaimWaiting, nil
	}
	secret, err := p.ClaimExchange(ctx, code)
	var fail *expose.PlayitFail
	if errors.As(err, &fail) && fail.Reason == "NotAccepted" {
		return ClaimWaiting, nil
	}
	if err != nil {
		return "", err
	}
	cfg, err := m.St.GetConfig()
	if err != nil {
		return "", err
	}
	cfg.PlayitSecret = secret
	if err := m.St.SaveConfig(cfg); err != nil {
		return "", err
	}
	return ClaimLinked, nil
}

// UnlinkPlayit closes every tunnel, stops the agent and forgets the key.
func (m *Manager) UnlinkPlayit(ctx context.Context, ev Events) error {
	servers, err := m.St.ListServers()
	if err != nil {
		return err
	}
	for _, s := range servers {
		if s.Public != nil && s.Public.Method == expose.MethodPlayit {
			if err := m.MakePrivate(ctx, s.Name, ev); err != nil {
				return err
			}
		}
	}
	if m.D != nil {
		if err := m.D.RemovePlayitAgent(ctx); err != nil {
			ev.Warn("couldn't stop the playit.gg agent: " + err.Error())
		}
	}
	cfg, err := m.St.GetConfig()
	if err != nil {
		return err
	}
	cfg.PlayitSecret = ""
	return m.St.SaveConfig(cfg)
}
