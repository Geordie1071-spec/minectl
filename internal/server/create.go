package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
	"github.com/Geordie1071-spec/minectl/internal/versions"
	"github.com/google/uuid"
)

type CreateOptions struct {
	Name           string
	MCType         string
	Version        string
	MemoryMB       int
	CPUCores       float64
	Port           int
	DataDir        string
	NoStart        bool
	ModpackID      string
	ModpackSource  string
	ModpackVersion string
	OnProgress     ProgressFunc
}

func Create(ctx context.Context, d *docker.Client, st *store.Store, opts CreateOptions) (*domain.Server, error) {
	cfg, err := st.GetConfig()
	if err != nil {
		return nil, err
	}
	if opts.MemoryMB <= 0 {
		opts.MemoryMB = cfg.DefaultMemoryMB
	}
	if opts.CPUCores < 0 {
		opts.CPUCores = 0
	}
	if opts.Port <= 0 {
		opts.Port = config.MinecraftPort
	}
	if opts.DataDir == "" {
		opts.DataDir = filepath.Join(cfg.DefaultDataDir, opts.Name)
	}
	opts.DataDir, err = filepath.Abs(opts.DataDir)
	if err != nil {
		return nil, err
	}
	backupDir := filepath.Join(cfg.DefaultBackupDir, opts.Name)

	existing, _ := st.GetServer(opts.Name)
	if existing != nil {
		if existing.Status != domain.StatusCreating || existing.ContainerID != "" {
			return nil, fmt.Errorf("server already exists: %s", opts.Name)
		}
		if id, err := d.ContainerByName(ctx, opts.Name); err == nil {
			_ = d.RemoveContainer(ctx, id)
		}
	}

	report := func(percent int, msg string) { reportProgress(opts.OnProgress, percent, msg) }
	report(0, "checking "+opts.MCType+" versions")
	resolved, err := versions.NewClient().Resolve(opts.MCType, opts.Version)
	if err != nil {
		return nil, err
	}
	report(2, "preparing directories")
	if err := os.MkdirAll(opts.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}
	if opts.ModpackSource == "curseforge" {
		if err := os.MkdirAll(filepath.Join(opts.DataDir, CurseForgeDownloadsDir), 0755); err != nil {
			return nil, fmt.Errorf("create downloads dir: %w", err)
		}
	}

	s := &domain.Server{
		ID:         "srv_" + uuid.New().String()[:8],
		Name:       opts.Name,
		MCType:     opts.MCType,
		MCVersion:  resolved.Version,
		MemoryMB:   opts.MemoryMB,
		CPUCores:   opts.CPUCores,
		Port:       opts.Port,
		Status:     domain.StatusCreating,
		ModLoader:  opts.MCType,
		DataDir:    opts.DataDir,
		BackupDir:  backupDir,
		ModsLocked: false,
	}
	if opts.ModpackID != "" {
		source := opts.ModpackSource
		if source == "" {
			source = "modrinth"
		}
		s.ModpackSource = &source
		s.ModpackID = &opts.ModpackID
		if opts.ModpackVersion != "" {
			s.ModpackVersion = &opts.ModpackVersion
		}
	}
	s.ImageTag = resolved.ImageTag
	s.CreatedAt = time.Now().UTC()

	if err := st.SaveServer(s); err != nil {
		return nil, err
	}

	report(5, "checking image")
	pull := func(percent int) { report(5+percent*85/100, "downloading "+config.MinecraftImage+":"+s.ImageTag) }
	if err := ensureImage(ctx, d, s, pull); err != nil {
		_ = st.DeleteServer(s.Name)
		return nil, err
	}
	report(92, "creating container")

	env := BuildEnvVars(s)
	containerID, err := d.CreateMinecraftContainer(ctx, s, env)
	if err != nil {
		_ = st.DeleteServer(s.Name)
		return nil, fmt.Errorf("create container: %w", err)
	}
	s.ContainerID = containerID
	if err := st.SaveServer(s); err != nil {
		return nil, err
	}

	if !opts.NoStart {
		report(96, "starting container")
		startedAt := time.Now().UTC()
		if err := d.StartContainer(ctx, containerID); err != nil {
			s.Status = domain.StatusStopped
			_ = st.SaveServer(s)
			return nil, fmt.Errorf("start container: %w", err)
		}
		s.Status = domain.StatusRunning
		s.LastStartedAt = startedAt
		if err := st.SaveServer(s); err != nil {
			return nil, err
		}
	} else {
		s.Status = domain.StatusStopped
		if err := st.SaveServer(s); err != nil {
			return nil, err
		}
	}
	report(100, "container created")
	return s, nil
}
