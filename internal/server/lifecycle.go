package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/minectl/minectl/internal/domain"
	"github.com/minectl/minectl/internal/docker"
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

func Restart(ctx context.Context, d *docker.Client, st *store.Store, name string) error {
	if err := Stop(ctx, d, st, name, false, 30); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	_, err := Start(ctx, d, st, name)
	return err
}

func RecreateContainer(ctx context.Context, d *docker.Client, st *store.Store, name string) (*domain.Server, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	if s.ContainerID != "" {
		_ = d.StopContainer(ctx, s.ContainerID, nil)
		if err := d.RemoveContainer(ctx, s.ContainerID); err != nil {
			return nil, fmt.Errorf("remove old container: %w", err)
		}
		s.ContainerID = ""
		if err := st.SaveServer(s); err != nil {
			return nil, err
		}
	}
	env := BuildEnvVars(s)
	containerID, err := d.CreateMinecraftContainer(ctx, s, env)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	s.ContainerID = containerID
	if err := st.SaveServer(s); err != nil {
		return nil, err
	}
	if err := d.StartContainer(ctx, containerID); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}
	s.Status = domain.StatusRunning
	s.LastStartedAt = time.Now().UTC()
	return s, st.SaveServer(s)
}

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
