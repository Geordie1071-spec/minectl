package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/jobs"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/dustin/go-humanize"
)

type modpackView struct {
	Source  string `json:"source"`
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

type serverView struct {
	Name          string       `json:"name"`
	Type          string       `json:"type"`
	Version       string       `json:"version"`
	Status        string       `json:"status"`
	Port          int          `json:"port"`
	MemoryMB      int          `json:"memory_mb"`
	CPUCores      float64      `json:"cpu_cores"`
	Modpack       *modpackView `json:"modpack,omitempty"`
	Mods          []domain.Mod `json:"mods"`
	CreatedAt     time.Time    `json:"created_at"`
	LastStartedAt time.Time    `json:"last_started_at"`
	Role          string       `json:"role"`
	Job           *domain.Job  `json:"job,omitempty"`
	DataDir       string       `json:"data_dir,omitempty"`
	Backups       *int         `json:"backups,omitempty"`

	BackupSchedule  string     `json:"backup_schedule,omitempty"`
	NextBackupAt    *time.Time `json:"next_backup_at,omitempty"`
	LastCrashAt     *time.Time `json:"last_crash_at,omitempty"`
	LastCrashReason string     `json:"last_crash_reason,omitempty"`
}

func (a *API) view(s *domain.Server, role string, detail bool) serverView {
	v := serverView{
		Name: s.Name, Type: s.MCType, Version: s.MCVersion, Status: s.Status, Port: s.Port,
		MemoryMB: s.MemoryMB, CPUCores: s.CPUCores, Mods: s.Mods, CreatedAt: s.CreatedAt,
		LastStartedAt: s.LastStartedAt, Role: role,
		BackupSchedule: s.BackupSchedule, NextBackupAt: s.AutoBackupNextAt,
		LastCrashAt: s.LastCrashAt, LastCrashReason: s.LastCrashReason,
	}
	if v.Mods == nil {
		v.Mods = []domain.Mod{}
	}
	if s.ModpackSource != nil && s.ModpackID != nil {
		mp := &modpackView{Source: *s.ModpackSource, ID: *s.ModpackID}
		if s.ModpackVersion != nil {
			mp.Version = *s.ModpackVersion
		}
		v.Modpack = mp
	}
	if j := a.jobs.Active(s.Name); j != nil {
		j.Events = nil
		v.Job = j
	}
	if role == domain.RoleAdmin {
		v.DataDir = s.DataDir
	}
	if detail {
		n := len(s.Backups)
		v.Backups = &n
	}
	return v
}

func (a *API) listServers(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var list []domain.Server
	var err error
	if a.m.D != nil {
		list, err = server.SyncAll(r.Context(), a.m.D, a.st)
	} else {
		list, err = a.st.ListServers()
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	roles := map[string]string{}
	if !c.user.IsAdmin {
		grants, err := a.st.ListGrants(c.user.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, g := range grants {
			roles[g.Server] = g.Role
		}
	}
	out := []serverView{}
	for i := range list {
		role := domain.RoleAdmin
		if !c.user.IsAdmin {
			if role = roles[list[i].Name]; role == "" {
				continue
			}
		}
		out = append(out, a.view(&list[i], role, false))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	s := c.server
	if a.m.D != nil {
		_ = server.SyncStatus(r.Context(), a.m.D, a.st, s)
	}
	writeJSON(w, http.StatusOK, a.view(s, c.role, true))
}

// manager returns a Manager whose locks name the panel user.
func (a *API) manager(c *reqCtx) *service.Manager {
	m := *a.m
	m.Holder = "panel (" + c.user.Username + ")"
	m.Wait = true
	return &m
}

func (a *API) needDocker(w http.ResponseWriter) bool {
	if a.m.D == nil {
		writeErr(w, http.StatusServiceUnavailable, "Docker isn't reachable from the panel; check that it's running")
		return false
	}
	return true
}

// startJob runs fn in the background and answers 202 with the job.
func (a *API) startJob(w http.ResponseWriter, r *http.Request, c *reqCtx, kind, srv, detail string, fn func(ctx context.Context, m *service.Manager, ev service.Events) (string, error)) {
	if !a.needDocker(w) {
		return
	}
	m := a.manager(c)
	j, err := a.jobs.Start(kind, srv, c.user.ID, func(ctx context.Context, ev service.Events) (string, error) {
		return fn(ctx, m, ev)
	})
	var busy *jobs.BusyError
	if errors.As(err, &busy) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": busy.Error(), "job_id": busy.JobID})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r, c, "server."+kind, srv, detail)
	writeJSON(w, http.StatusAccepted, j)
}

type createRequest struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Version        string `json:"version"`
	Memory         string `json:"memory"`
	CPU            string `json:"cpu"`
	Players        int    `json:"players"`
	Mods           int    `json:"mods"`
	Port           int    `json:"port"`
	Modpack        string `json:"modpack"`
	ModpackVersion string `json:"modpack_version"`
	CurseForge     string `json:"curseforge"`
}

func (a *API) createServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req createRequest
	if !decode(w, r, &req) {
		return
	}
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if err := service.ValidateName(req.Name); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Type != "" {
		if err := service.ValidateType(req.Type); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.Port != 0 && (req.Port < 1024 || req.Port > 65535) {
		writeErr(w, http.StatusBadRequest, "port must be between 1024 and 65535, or 0 to pick one")
		return
	}
	if req.Players < 0 || req.Players > 1000 || req.Mods < 0 || req.Mods > 5000 {
		writeErr(w, http.StatusBadRequest, "players must be 0-1000 and mods 0-5000")
		return
	}
	if req.Modpack != "" && req.CurseForge != "" {
		writeErr(w, http.StatusBadRequest, "use either a Modrinth modpack or a CurseForge pack, not both")
		return
	}
	if err := service.ValidateMemory(req.Memory); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if existing, err := a.st.GetServer(req.Name); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	} else if existing != nil {
		writeErr(w, http.StatusConflict, "a server named "+req.Name+" already exists")
		return
	}
	creq := service.CreateRequest{
		Name: req.Name, Type: req.Type, TypeExplicit: req.Type != "", Version: req.Version,
		Memory: req.Memory, CPU: req.CPU, Players: req.Players, Mods: req.Mods, ModsExplicit: req.Mods > 0,
		Port: req.Port, Modpack: req.Modpack, ModpackVersion: req.ModpackVersion, CurseForge: req.CurseForge,
	}
	if creq.Players == 0 {
		creq.Players = 5
	}
	detail := describeCreate(req)
	a.startJob(w, r, c, "create", req.Name, detail, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		s, err := m.Create(ctx, creq, ev)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s is up on port %d", s.Name, s.Port), nil
	})
}

func describeCreate(req createRequest) string {
	switch {
	case req.Modpack != "":
		return "modrinth modpack " + req.Modpack
	case req.CurseForge != "":
		return "curseforge pack " + req.CurseForge
	}
	t := req.Type
	if t == "" {
		t = domain.TypePaper
	}
	return strings.TrimSpace(t + " " + req.Version)
}

func (a *API) deleteServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	purge := r.URL.Query().Get("purge") == "true"
	detail := ""
	if purge {
		detail = "purge"
	}
	name := c.server.Name
	a.startJob(w, r, c, "delete", name, detail, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.Delete(ctx, name, purge, ev); err != nil {
			return "", err
		}
		return "deleted " + name, nil
	})
}

func (a *API) startServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	name := c.server.Name
	a.startJob(w, r, c, "start", name, "", func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if _, err := m.Start(ctx, name, ev); err != nil {
			return "", err
		}
		return name + " is running", nil
	})
}

func (a *API) stopServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Force bool `json:"force"`
	}
	if !decode(w, r, &req) {
		return
	}
	name := c.server.Name
	a.startJob(w, r, c, "stop", name, "", func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.Stop(ctx, name, req.Force, 30, ev); err != nil {
			return "", err
		}
		return name + " stopped", nil
	})
}

func (a *API) restartServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	name := c.server.Name
	a.startJob(w, r, c, "restart", name, "", func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.Restart(ctx, name, ev); err != nil {
			return "", err
		}
		return name + " restarted", nil
	})
}

func (a *API) resizeServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Memory  string `json:"memory"`
		CPU     string `json:"cpu"`
		Players int    `json:"players"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Memory == "" && req.CPU == "" {
		writeErr(w, http.StatusBadRequest, "nothing to change; send memory and/or cpu")
		return
	}
	if err := service.ValidateMemory(req.Memory); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Players <= 0 {
		req.Players = 5
	}
	name := c.server.Name
	a.startJob(w, r, c, "resize", name, strings.TrimSpace("memory="+req.Memory+" cpu="+req.CPU), func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.Resize(ctx, name, req.Memory, req.CPU, req.Players, ev); err != nil {
			return "", err
		}
		return name + " resized", nil
	})
}

func (a *API) upgradeServer(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Version  string `json:"version"`
		NoBackup bool   `json:"no_backup"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Version == "" {
		writeErr(w, http.StatusBadRequest, "send the version to upgrade to")
		return
	}
	name := c.server.Name
	a.startJob(w, r, c, "upgrade", name, req.Version, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.Upgrade(ctx, name, req.Version, req.NoBackup, ev); err != nil {
			return "", err
		}
		return name + " upgraded to " + req.Version, nil
	})
}

func (a *API) setModpack(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Slug    string `json:"slug"`
		Version string `json:"version"`
		Force   bool   `json:"force"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Slug == "" {
		writeErr(w, http.StatusBadRequest, "send the Modrinth modpack slug")
		return
	}
	name := c.server.Name
	a.startJob(w, r, c, "modpack", name, req.Slug, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.SetModpack(ctx, name, req.Slug, req.Version, req.Force, ev); err != nil {
			return "", err
		}
		return name + " now runs " + req.Slug, nil
	})
}

func (a *API) addMod(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Slug    string `json:"slug"`
		Version string `json:"version"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Slug == "" {
		writeErr(w, http.StatusBadRequest, "send the Modrinth mod slug")
		return
	}
	name := c.server.Name
	a.startJob(w, r, c, "mods.add", name, req.Slug, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		mod, err := m.AddMod(ctx, name, req.Slug, req.Version, ev)
		if err != nil {
			return "", err
		}
		return "added " + mod.ModName, nil
	})
}

func (a *API) removeMod(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	slug := r.PathValue("slug")
	name := c.server.Name
	a.startJob(w, r, c, "mods.remove", name, slug, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		mod, err := m.RemoveMod(ctx, name, slug, ev)
		if err != nil {
			return "", err
		}
		return "removed " + mod.ModName, nil
	})
}

func (a *API) listBackups(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	list, err := server.ListBackups(a.st, c.server.Name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	type backupView struct {
		ID        string    `json:"id"`
		SizeBytes int64     `json:"size_bytes"`
		CreatedAt time.Time `json:"created_at"`
	}
	out := []backupView{}
	for _, b := range list {
		out = append(out, backupView{ID: b.ID, SizeBytes: b.SizeBytes, CreatedAt: b.CreatedAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createBackup(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	name := c.server.Name
	a.startJob(w, r, c, "backup", name, "", func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		ev.Stage("Backing up")
		b, err := m.CreateBackup(ctx, name)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("backup %s (%s)", b.ID, humanize.Bytes(uint64(b.SizeBytes))), nil
	})
}

func (a *API) restoreBackup(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	id := r.PathValue("id")
	found := false
	for _, b := range c.server.Backups {
		found = found || b.ID == id
	}
	if !found {
		writeErr(w, http.StatusNotFound, "backup not found: "+id)
		return
	}
	name := c.server.Name
	a.startJob(w, r, c, "restore", name, id, func(ctx context.Context, m *service.Manager, ev service.Events) (string, error) {
		if err := m.RestoreBackup(ctx, name, id, ev); err != nil {
			return "", err
		}
		return "restored " + id, nil
	})
}

func (a *API) running(w http.ResponseWriter, c *reqCtx) bool {
	if !a.needDocker(w) {
		return false
	}
	if c.server.ContainerID == "" || c.server.Status != domain.StatusRunning {
		writeErr(w, http.StatusConflict, c.server.Name+" isn't running")
		return false
	}
	return true
}

func (a *API) rcon(r *http.Request, c *reqCtx, command string) (string, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	return a.m.D.ExecRcon(ctx, c.server.ContainerID, command)
}

func (a *API) console(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	var req struct {
		Command string `json:"command"`
	}
	if !decode(w, r, &req) {
		return
	}
	cmd := strings.TrimPrefix(strings.TrimSpace(req.Command), "/")
	if cmd == "" || len(cmd) > 1000 || strings.ContainsAny(cmd, "\r\n\x00") {
		writeErr(w, http.StatusBadRequest, "send one command of up to 1000 characters")
		return
	}
	if !a.running(w, c) {
		return
	}
	out, err := a.rcon(r, c, cmd)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "console: "+err.Error())
		return
	}
	a.audit(r, c, "server.console", c.server.Name, truncate(cmd, 200))
	writeJSON(w, http.StatusOK, map[string]string{"output": out})
}

func (a *API) players(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	if !a.running(w, c) {
		return
	}
	out, err := a.rcon(r, c, "list")
	if err != nil {
		writeErr(w, http.StatusBadGateway, "console: "+err.Error())
		return
	}
	p, ok := server.ParsePlayers(out)
	if !ok {
		writeErr(w, http.StatusBadGateway, "couldn't read the player list: "+truncate(out, 200))
		return
	}
	writeJSON(w, http.StatusOK, p)
}

var (
	validPlayer   = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)
	playerActions = map[string]string{
		"whitelist":   "whitelist add %s",
		"unwhitelist": "whitelist remove %s",
		"op":          "op %s",
		"deop":        "deop %s",
		"kick":        "kick %s",
		"ban":         "ban %s",
		"pardon":      "pardon %s",
	}
)

func (a *API) playerAction(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	player, action := r.PathValue("player"), r.PathValue("action")
	format, ok := playerActions[action]
	if !ok {
		writeErr(w, http.StatusBadRequest, "action must be one of: whitelist, unwhitelist, op, deop, kick, ban, pardon")
		return
	}
	if !validPlayer.MatchString(player) {
		writeErr(w, http.StatusBadRequest, "invalid player name")
		return
	}
	if !a.running(w, c) {
		return
	}
	out, err := a.rcon(r, c, fmt.Sprintf(format, player))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "console: "+err.Error())
		return
	}
	a.audit(r, c, "player."+action, c.server.Name, player)
	writeJSON(w, http.StatusOK, map[string]string{"output": out})
}
