package server

import (
	"context"
	"fmt"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
	"github.com/Geordie1071-spec/minectl/internal/versions"
)

func Upgrade(ctx context.Context, d *docker.Client, st *store.Store, name, newVersion string, noBackup bool) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	resolved, err := versions.NewClient().Resolve(s.MCType, newVersion)
	if err != nil {
		return err
	}
	if !noBackup {
		if _, err := CreateBackup(ctx, d, st, name); err != nil {
			return fmt.Errorf("pre-upgrade backup failed: %w", err)
		}
	}

	oldVersion, oldTag := s.MCVersion, s.ImageTag
	s.MCVersion = resolved.Version
	s.ImageTag = resolved.ImageTag
	if err := ensureImage(ctx, d, s, nil); err != nil {
		return err
	}

	if err := Stop(ctx, d, st, name, false, 30, nil); err != nil {
		return err
	}
	s, err = st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if s.ContainerID != "" {
		_ = d.RemoveContainer(ctx, s.ContainerID)
		s.ContainerID = ""
	}

	s.MCVersion = resolved.Version
	s.ImageTag = resolved.ImageTag
	containerID, err := d.CreateMinecraftContainer(ctx, s, BuildEnvVars(s))
	if err != nil {
		s.MCVersion, s.ImageTag = oldVersion, oldTag
		s.Status = domain.StatusStopped
		_ = st.SaveServer(s)
		return fmt.Errorf("recreate container (kept %s; run start to recreate it): %w", oldVersion, err)
	}
	s.ContainerID = containerID
	s.Status = domain.StatusStopped
	if err := st.SaveServer(s); err != nil {
		return err
	}
	_, err = Start(ctx, d, st, name, nil)
	return err
}
