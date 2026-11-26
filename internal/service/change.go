package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/modrinth"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/versions"
)

func (m *Manager) Resize(ctx context.Context, name, memory, cpu string, players int, ev Events) error {
	if memory == "" && cpu == "" {
		return errors.New("nothing to change; pass --memory and/or --cpu")
	}
	unlock, err := m.lock(ctx, name, "resize")
	if err != nil {
		return err
	}
	defer unlock()
	s, err := m.get(name)
	if err != nil {
		return err
	}
	if memory == "" {
		memory = fmt.Sprintf("%dM", s.MemoryMB)
	}
	if cpu == "" {
		cpu = fmt.Sprintf("%g", s.CPUCores)
	}
	plan, err := m.PlanResources(ctx, s.MCType, players, m.modpackModCount(s), memory, cpu)
	if err != nil {
		return err
	}
	s.MemoryMB, s.CPUCores = plan.MemoryMB, plan.CPUCores
	if err := m.St.SaveServer(s); err != nil {
		return err
	}
	plan.report(ev)
	return m.apply(ctx, name, ev)
}

func (m *Manager) SetModpack(ctx context.Context, name, slug, packVersion string, force bool, ev Events) error {
	unlock, err := m.lock(ctx, name, "modpack")
	if err != nil {
		return err
	}
	defer unlock()
	s, err := m.get(name)
	if err != nil {
		return err
	}

	mc := modrinth.NewClient()
	rel, err := mc.ResolveModpack(slug, "", "", packVersion)
	if err != nil {
		return err
	}
	resolved, err := versions.NewClient().Resolve(rel.Loader, rel.MCVersion)
	if err != nil {
		return err
	}
	_, statErr := os.Stat(filepath.Join(s.DataDir, "world"))
	hasWorld := statErr == nil
	if hasWorld && versions.Compare(resolved.Version, s.MCVersion) < 0 && !force {
		return fmt.Errorf("%s's world is on Minecraft %s but %s needs %s; Minecraft can't downgrade worlds safely.\nCreate a new server for the modpack instead, or pass --force to replace it anyway (a backup is made first)", name, s.MCVersion, slug, resolved.Version)
	}
	if hasWorld {
		b, err := server.CreateBackup(ctx, m.D, m.St, name)
		if err != nil {
			return fmt.Errorf("backup before switching to modpack: %w", err)
		}
		ev.Info("Backed up current world: " + b.ID)
		if s, err = m.get(name); err != nil {
			return err
		}
	}

	source := "modrinth"
	s.ModpackSource = &source
	s.ModpackID = &slug
	s.ModpackVersion = &rel.VersionID
	s.MCType, s.ModLoader = rel.Loader, rel.Loader
	s.MCVersion, s.ImageTag = resolved.Version, resolved.ImageTag
	if err := m.St.SaveServer(s); err != nil {
		return err
	}
	ev.Info(fmt.Sprintf("Modpack: %s %s (%s, Minecraft %s)", slug, rel.VersionNumber, rel.Loader, resolved.Version))
	if len(s.Mods) > 0 {
		ev.Warn("individually added mods are ignored while a modpack is set")
	}
	if n, err := mc.CountModpackMods(rel.FileURL); err == nil {
		host := m.host(ctx)
		for _, w := range server.CheckMemoryChoice(s.MemoryMB, server.Recommend(rel.Loader, 5, n, host), host, 0) {
			ev.Warn(fmt.Sprintf("%s. Change it with: minectl resize %s --memory auto", w, name))
		}
	}
	return m.apply(ctx, name, ev)
}

func (m *Manager) AddMod(ctx context.Context, name, slug, version string, ev Events) (*domain.Mod, error) {
	unlock, err := m.lock(ctx, name, "mods")
	if err != nil {
		return nil, err
	}
	defer unlock()
	mod, err := server.AddMod(ctx, m.St, name, slug, version)
	if err != nil {
		return nil, err
	}
	ev.Info(fmt.Sprintf("Mod added: %s (required dependencies are installed automatically)", mod.ModName))
	return mod, m.apply(ctx, name, ev)
}

func (m *Manager) RemoveMod(ctx context.Context, name, slug string, ev Events) (*domain.Mod, error) {
	unlock, err := m.lock(ctx, name, "mods")
	if err != nil {
		return nil, err
	}
	defer unlock()
	mod, err := server.RemoveMod(m.St, name, slug)
	if err != nil {
		return nil, err
	}
	ev.Info("Mod removed: " + mod.ModName)
	return mod, m.apply(ctx, name, ev)
}

func (m *Manager) apply(ctx context.Context, name string, ev Events) error {
	if m.D == nil {
		ev.Warn("saved, but Docker isn't reachable; the change applies the next time the container is recreated")
		return nil
	}
	s, err := m.get(name)
	if err != nil {
		return err
	}
	if s.ContainerID == "" {
		ev.Info("Saved. It applies the next time you run: minectl start " + name)
		return nil
	}
	ev.Info("Recreating container to apply changes...")
	started, err := server.RecreateContainer(ctx, m.D, m.St, name)
	if err != nil {
		return err
	}
	if !started {
		ev.Info("Saved. It applies the next time you run: minectl start " + name)
		return nil
	}
	if s, err = m.get(name); err != nil {
		return err
	}
	return m.waitAfterStart(ctx, name, s.LastStartedAt, ev)
}
