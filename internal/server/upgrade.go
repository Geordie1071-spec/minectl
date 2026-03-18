package server

import (
	"context"
	"fmt"

	"github.com/minectl/minectl/internal/docker"
	"github.com/minectl/minectl/internal/domain"
	"github.com/minectl/minectl/internal/store"
)

func Upgrade(ctx context.Context, d *docker.Client, st *store.Store, name, newVersion string, noBackup bool) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if !noBackup {
		if _, err := CreateBackup(ctx, d, st, name); err != nil {
			return fmt.Errorf("pre-upgrade backup failed: %w", err)
		}
	}
	if err := Stop(ctx, d, st, name, true, 15, nil); err != nil {
		return err
	}
	if s.ContainerID != "" {
		_ = d.RemoveContainer(ctx, s.ContainerID)
		s.ContainerID = ""
	}
	s.MCVersion = newVersion
	env := BuildEnvVars(s)
	containerID, err := d.CreateMinecraftContainer(ctx, s, env)
	if err != nil {
		return fmt.Errorf("recreate container: %w", err)
	}
	s.ContainerID = containerID
	s.Status = domain.StatusStopped
	if err := st.SaveServer(s); err != nil {
		return err
	}
	_, err = Start(ctx, d, st, name, nil)
	return err
}
