package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/minectl/minectl/internal/docker"
	"github.com/minectl/minectl/internal/domain"
	"github.com/minectl/minectl/internal/store"
)

func Start(ctx context.Context, d *docker.Client, st *store.Store, name string) (*domain.Server, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	if s.Status == domain.StatusRunning {
		return s, nil
	}
	if s.ContainerID == "" {
		return nil, fmt.Errorf("server has no container; recreate with minectl create")
	}
	if err := d.StartContainer(ctx, s.ContainerID); err != nil {
		return nil, err
	}
	s.Status = domain.StatusRunning
	s.LastStartedAt = time.Now().UTC()
	return s, st.SaveServer(s)
}

func Stop(ctx context.Context, d *docker.Client, st *store.Store, name string, force bool, timeoutSec int) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if s.Status != domain.StatusRunning {
		return nil
	}
	if !force {
		_, _ = d.ExecRcon(ctx, s.ContainerID, "save-all")
		_, _ = d.ExecRcon(ctx, s.ContainerID, "stop")
		if timeoutSec <= 0 {
			timeoutSec = 30
		}
		for i := 0; i < timeoutSec; i++ {
			time.Sleep(time.Second)
			running, _ := d.ContainerRunning(ctx, s.ContainerID)
			if !running {
				break
			}
		}
	}
	t := timeoutSec
	if t <= 0 {
		t = 10
	}
	if err := d.StopContainer(ctx, s.ContainerID, &t); err != nil {
		return err
	}
	s.Status = domain.StatusStopped
	return st.SaveServer(s)
}

// Restart stops then starts the server
func Restart(ctx context.Context, d *docker.Client, st *store.Store, name string) error {
	if err := Stop(ctx, d, st, name, false, 30); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	_, err := Start(ctx, d, st, name)
	return err
}

// Delete stops and removes the container; does not delete world data unless purge
func Delete(ctx context.Context, d *docker.Client, st *store.Store, name string, purge bool) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if s.ContainerID != "" {
		_ = d.StopContainer(ctx, s.ContainerID, nil)
		_ = d.RemoveContainer(ctx, s.ContainerID)
	}
	if purge {
		_ = removeAll(s.DataDir)
		_ = removeAll(s.BackupDir)
	}
	return st.DeleteServer(name)
}

func removeAll(path string) error {
	return os.RemoveAll(path)
}

// RecreateContainer removes and recreates the server's Docker container using its current config.
// This is used when changing settings (e.g. modpack) that must be applied at container creation time.
func RecreateContainer(ctx context.Context, d *docker.Client, st *store.Store, name string) (*domain.Server, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	// Stop quickly; the caller is changing config so we prefer correctness over a long graceful stop.
	_ = Stop(ctx, d, st, name, true, 15)

	if s.ContainerID != "" {
		_ = d.RemoveContainer(ctx, s.ContainerID)
		s.ContainerID = ""
		if err := st.SaveServer(s); err != nil {
			return nil, err
		}
	}

	env := BuildEnvVars(s)
	containerID, err := d.CreateMinecraftContainer(ctx, s, env)
	if err != nil {
		return nil, fmt.Errorf("recreate container: %w", err)
	}
	s.ContainerID = containerID
	s.Status = domain.StatusStopped
	if err := st.SaveServer(s); err != nil {
		return nil, err
	}

	if _, err := Start(ctx, d, st, name); err != nil {
		return nil, err
	}
	return st.GetServer(name)
}
