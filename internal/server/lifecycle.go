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

type ProgressFunc func(percent int, msg string)

func reportProgress(p ProgressFunc, percent int, msg string) {
	if p == nil {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	p(percent, msg)
}

func Start(ctx context.Context, d *docker.Client, st *store.Store, name string, progress ProgressFunc) (*domain.Server, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	if s.Status == domain.StatusRunning {
		reportProgress(progress, 100, "already running")
		return s, nil
	}
	if s.ContainerID == "" {
		return nil, fmt.Errorf("server has no container; recreate with minectl create")
	}
	reportProgress(progress, 0, "starting")
	if err := d.StartContainer(ctx, s.ContainerID); err != nil {
		return nil, err
	}

	waitTimeoutSec := 90
	for i := 0; i < waitTimeoutSec; i++ {
		running, _ := d.ContainerRunning(ctx, s.ContainerID)
		percent := 20 + int(float64(i)/float64(waitTimeoutSec)*80)
		if running {
			percent = 100
		}
		if running || i == waitTimeoutSec-1 {
			reportProgress(progress, percent, "starting container")
		} else {
			reportProgress(progress, percent, "starting container")
		}
		if running {
			break
		}
		time.Sleep(time.Second)
	}
	running, _ := d.ContainerRunning(ctx, s.ContainerID)
	if !running {
		return nil, fmt.Errorf("start timed out: container did not reach running state")
	}

	s.Status = domain.StatusRunning
	s.LastStartedAt = time.Now().UTC()
	reportProgress(progress, 100, "running")
	return s, st.SaveServer(s)
}

func Stop(ctx context.Context, d *docker.Client, st *store.Store, name string, force bool, timeoutSec int, progress ProgressFunc) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if s.Status != domain.StatusRunning {
		reportProgress(progress, 100, "already stopped")
		return nil
	}
	reportProgress(progress, 0, "stopping")
	if !force {
		_, _ = d.ExecRcon(ctx, s.ContainerID, "save-all")
		_, _ = d.ExecRcon(ctx, s.ContainerID, "stop")
		if timeoutSec <= 0 {
			timeoutSec = 30
		}
		for i := 0; i < timeoutSec; i++ {
			time.Sleep(time.Second)
			running, _ := d.ContainerRunning(ctx, s.ContainerID)
			shutdownPercent := 10 + int(float64(i)/float64(timeoutSec)*60)
			if !running {
				shutdownPercent = 70
			}
			reportProgress(progress, shutdownPercent, "shutdown in progress")
			if !running {
				break
			}
		}
	}
	reportProgress(progress, 70, "stopping container")
	t := timeoutSec
	if t <= 0 {
		t = 10
	}
	if err := d.StopContainer(ctx, s.ContainerID, &t); err != nil {
		return err
	}
	s.Status = domain.StatusStopped
	reportProgress(progress, 100, "stopped")
	return st.SaveServer(s)
}

func Restart(ctx context.Context, d *docker.Client, st *store.Store, name string, progress ProgressFunc) error {
	stopProgress := func(percent int, msg string) {
		if progress == nil {
			return
		}
		reportProgress(progress, percent/2, msg)
	}
	startProgress := func(percent int, msg string) {
		if progress == nil {
			return
		}
		reportProgress(progress, 50+percent/2, msg)
	}

	reportProgress(progress, 0, "restarting")
	if err := Stop(ctx, d, st, name, false, 30, stopProgress); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	_, err := Start(ctx, d, st, name, startProgress)
	return err
}

func Delete(ctx context.Context, d *docker.Client, st *store.Store, name string, purge bool, progress ProgressFunc) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	reportProgress(progress, 0, "deleting")
	if s.ContainerID != "" {
		reportProgress(progress, 10, "stopping container")
		_ = d.StopContainer(ctx, s.ContainerID, nil)
		_ = d.RemoveContainer(ctx, s.ContainerID)
		reportProgress(progress, 40, "container removed")
	}
	if purge {
		reportProgress(progress, 45, "purging world data")
		_ = removeAll(s.DataDir)
		reportProgress(progress, 70, "world data purged")
		_ = removeAll(s.BackupDir)
		reportProgress(progress, 85, "backups purged")
	}
	if err := st.DeleteServer(name); err != nil {
		return err
	}
	reportProgress(progress, 100, "deleted")
	return nil
}

func removeAll(path string) error {
	return os.RemoveAll(path)
}

func RecreateContainer(ctx context.Context, d *docker.Client, st *store.Store, name string, progress ProgressFunc) (*domain.Server, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	_ = Stop(ctx, d, st, name, true, 15, nil)

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

	if _, err := Start(ctx, d, st, name, nil); err != nil {
		return nil, err
	}
	return st.GetServer(name)
}
