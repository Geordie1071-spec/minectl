package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/modrinth"
	"github.com/Geordie1071-spec/minectl/internal/server"
)

type CreateRequest struct {
	Name           string
	Type           string
	TypeExplicit   bool
	Version        string
	Memory         string
	CPU            string
	Players        int
	Mods           int
	ModsExplicit   bool
	Port           int
	DataDir        string
	NoStart        bool
	Modpack        string
	ModpackVersion string
	CurseForge     string
}

func (m *Manager) Create(ctx context.Context, req CreateRequest, ev Events) (*domain.Server, error) {
	if err := ValidateName(req.Name); err != nil {
		return nil, err
	}
	if req.Modpack != "" && req.CurseForge != "" {
		return nil, errors.New("use either a Modrinth modpack or a CurseForge pack, not both")
	}
	if req.Type == "" {
		req.Type = domain.TypePaper
	}
	if err := ValidateType(req.Type); err != nil {
		return nil, err
	}
	if req.Memory == "" {
		req.Memory = "auto"
	}
	if req.CPU == "" {
		req.CPU = "auto"
	}
	unlock, err := m.lock(ctx, req.Name, "create")
	if err != nil {
		return nil, err
	}
	defer unlock()

	mcType, version, modpackVersion, mods := req.Type, req.Version, req.ModpackVersion, req.Mods
	if req.Modpack != "" {
		loader := ""
		if req.TypeExplicit {
			if modrinth.NormalizeLoader(req.Type) == "" {
				return nil, fmt.Errorf("modpack requires a mod loader (use --type fabric, forge, neoforge, or quilt), not %q", req.Type)
			}
			loader = req.Type
		}
		mc := modrinth.NewClient()
		rel, err := mc.ResolveModpack(req.Modpack, loader, req.Version, req.ModpackVersion)
		if err != nil {
			return nil, err
		}
		mcType, version, modpackVersion = rel.Loader, rel.MCVersion, rel.VersionID
		if n, err := mc.CountModpackMods(rel.FileURL); err == nil {
			mods = n
		} else {
			ev.Warn(fmt.Sprintf("could not count mods in modpack: %v", err))
		}
		ev.Info(fmt.Sprintf("Modpack: %s %s (%s, Minecraft %s, %d server mods)", rel.Slug, rel.VersionNumber, rel.Loader, rel.MCVersion, mods))
	}

	port := req.Port
	if port <= 0 {
		port = m.nextFreePort(config.MinecraftPort)
	}

	modpackID, modpackSource, dataDir := req.Modpack, "", req.DataDir
	if req.CurseForge != "" {
		dataDir, err = server.DataDirFor(m.St, req.Name, req.DataDir)
		if err != nil {
			return nil, err
		}
		pack, err := m.installCurseForge(ctx, req, dataDir, ev)
		if err != nil {
			return nil, err
		}
		modpackID, modpackSource = req.CurseForge, "curseforge"
		mcType, version = pack.MCType, pack.MCVersion
		if !req.ModsExplicit {
			mods = pack.Mods
		}
		ev.Info(fmt.Sprintf("Modpack: %s %s (%s, Minecraft %s, %d mods)", pack.Name, pack.PackVersion, pack.MCType, pack.MCVersion, pack.Mods))
	}

	plan, err := m.PlanResources(ctx, mcType, req.Players, mods, req.Memory, req.CPU)
	if err != nil {
		return nil, err
	}
	plan.report(ev, m.CLIHints)

	s, err := server.Create(ctx, m.D, m.St, server.CreateOptions{
		Name:           req.Name,
		MCType:         mcType,
		Version:        version,
		MemoryMB:       plan.MemoryMB,
		CPUCores:       plan.CPUCores,
		Port:           port,
		DataDir:        dataDir,
		NoStart:        req.NoStart,
		ModpackID:      modpackID,
		ModpackSource:  modpackSource,
		ModpackVersion: modpackVersion,
		OnProgress:     ev.Progress,
	})
	if err != nil {
		return nil, err
	}
	if s.Status != domain.StatusRunning || !m.Wait {
		return s, nil
	}
	err = m.WaitReady(ctx, s, s.LastStartedAt, ev)
	var sf *StartFailedError
	if errors.As(err, &sf) {
		_ = server.AbortCreate(context.Background(), m.D, m.St, s.Name)
		sf.Cleanup = fmt.Sprintf("Removed the failed server %q so it doesn't keep restarting (its folder %s was kept). Fix the problem and run create again.", s.Name, s.DataDir)
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (m *Manager) installCurseForge(ctx context.Context, req CreateRequest, dataDir string, ev Events) (*server.CurseForgePack, error) {
	staged := false
	start := time.Now()
	onPercent := func(p int) { ev.Progress(p, "downloading Java image") }
	onLine := func(line string) {
		if !staged {
			ev.Stage("Installing modpack")
			staged = true
			start = time.Now()
		}
		ev.Log(line)
	}
	pack, err := server.InstallCurseForgePack(ctx, m.D, m.St, req.Name, dataDir, req.CurseForge, req.ModpackVersion, onPercent, onLine)
	if err == nil {
		ev.Info(fmt.Sprintf("Modpack installed in %s", time.Since(start).Round(time.Second)))
		return pack, nil
	}
	var setupErr *server.SetupError
	if !errors.As(err, &setupErr) {
		return nil, err
	}
	msg := "the modpack failed to install"
	if hint := server.DescribeStartFailure(setupErr.Logs); hint != "" {
		msg += ": " + hint
	}
	if summary := server.FailureSummary(setupErr.Logs); len(summary) > 0 {
		msg += "\n\nKey log lines:\n  " + strings.Join(summary, "\n  ")
	}
	if server.NeedsManualDownloads(setupErr.Logs) {
		msg += fmt.Sprintf("\n\nSave those files into %s, then run the same create command again.", filepath.Join(dataDir, server.CurseForgeDownloadsDir))
	}
	return nil, errors.New(msg)
}

func (m *Manager) nextFreePort(start int) int {
	servers, _ := m.St.ListServers()
	used := map[int]bool{}
	for _, s := range servers {
		used[s.Port] = true
	}
	for p := start; ; p++ {
		if !used[p] {
			return p
		}
	}
}
