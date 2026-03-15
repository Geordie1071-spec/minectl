package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/minectl/minectl/internal/config"
	"github.com/minectl/minectl/internal/docker"
	"github.com/minectl/minectl/internal/domain"
	"github.com/minectl/minectl/internal/store"
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
	ModpackVersion string
	OnProgress     func(msg string)
}

func Create(ctx context.Context, d *docker.Client, st *store.Store, opts CreateOptions) (*domain.Server, error) {
	cfg, err := st.GetConfig()
	if err != nil {
		return nil, err
	}
	if opts.MemoryMB <= 0 {
		opts.MemoryMB = cfg.DefaultMemoryMB
	}
	if opts.CPUCores <= 0 {
		opts.CPUCores = 1.0
	}
	if opts.Port <= 0 {
		opts.Port = config.MinecraftPort
	}
	if opts.DataDir == "" {
		opts.DataDir = filepath.Join(cfg.DefaultDataDir, opts.Name)
	}
	backupDir := filepath.Join(cfg.DefaultBackupDir, opts.Name)

	existing, _ := st.GetServer(opts.Name)
	if existing != nil {
		return nil, fmt.Errorf("server already exists: %s", opts.Name)
	}

	if opts.OnProgress != nil {
		opts.OnProgress("Preparing directories...")
	}
	if err := os.MkdirAll(opts.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	s := &domain.Server{
		ID:         "srv_" + uuid.New().String()[:8],
		Name:       opts.Name,
		MCType:     opts.MCType,
		MCVersion:  opts.Version,
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
		source := "modrinth"
		s.ModpackSource = &source
		s.ModpackID = &opts.ModpackID
		if opts.ModpackVersion != "" {
			s.ModpackVersion = &opts.ModpackVersion
		}
	}
	s.ImageTag = config.ImageTagForMCVersion(s.MCVersion, s.MCType)
	if s.ImageTag == "" {
		s.ImageTag = "latest"
	}
	s.CreatedAt = time.Now().UTC()

	if err := st.SaveServer(s); err != nil {
		return nil, err
	}

	imageRef := config.MinecraftImage + ":" + s.ImageTag
	exists, err := d.ImageExists(ctx, imageRef)
	if err != nil {
		_ = st.DeleteServer(s.Name)
		return nil, fmt.Errorf("check image: %w", err)
	}
	if !exists {
		if opts.OnProgress != nil {
			opts.OnProgress("Pulling Docker image " + imageRef + "...")
		}
		rc, err := d.PullImage(ctx, imageRef)
		if err != nil {
			_ = st.DeleteServer(s.Name)
			return nil, fmt.Errorf("pull image %s: %w", imageRef, err)
		}
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}
	if opts.OnProgress != nil {
		opts.OnProgress("Creating container...")
	}

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
		if opts.OnProgress != nil {
			opts.OnProgress("Starting server...")
		}
		if err := d.StartContainer(ctx, containerID); err != nil {
			return nil, fmt.Errorf("start container: %w", err)
		}
		s.Status = domain.StatusRunning
		s.LastStartedAt = time.Now().UTC()
		if err := st.SaveServer(s); err != nil {
			return nil, err
		}
	} else {
		s.Status = domain.StatusStopped
		if err := st.SaveServer(s); err != nil {
			return nil, err
		}
	}
	if opts.OnProgress != nil {
		opts.OnProgress("Done")
	}
	return s, nil
}
