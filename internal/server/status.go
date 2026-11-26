package server

import (
	"context"
	"fmt"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

func SyncStatus(ctx context.Context, d *docker.Client, st *store.Store, s *domain.Server) error {
	if s.Status == domain.StatusCreating {
		return nil
	}
	var status string
	containerID := s.ContainerID
	if containerID == "" {
		status = domain.StatusStopped
	} else {
		running, exists, err := d.ContainerState(ctx, containerID)
		if err != nil {
			return err
		}
		switch {
		case !exists:
			containerID = ""
			status = domain.StatusStopped
		case running:
			status = domain.StatusRunning
		default:
			status = domain.StatusStopped
		}
	}
	if status == s.Status && containerID == s.ContainerID {
		return nil
	}
	s.Status = status
	s.ContainerID = containerID
	return st.SaveServer(s)
}

func SyncAll(ctx context.Context, d *docker.Client, st *store.Store) ([]domain.Server, error) {
	servers, err := st.ListServers()
	if err != nil {
		return nil, err
	}
	for i := range servers {
		if err := SyncStatus(ctx, d, st, &servers[i]); err != nil {
			return nil, err
		}
	}
	return servers, nil
}

func ensureImage(ctx context.Context, d *docker.Client, s *domain.Server, onPercent func(percent int)) error {
	return ensureImageRef(ctx, d, config.MinecraftImage+":"+s.ImageTag, onPercent)
}

func ensureImageRef(ctx context.Context, d *docker.Client, imageRef string, onPercent func(percent int)) error {
	exists, err := d.ImageExists(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("check image: %w", err)
	}
	if exists {
		return nil
	}
	if err := d.PullImageWithProgress(ctx, imageRef, onPercent); err != nil {
		return fmt.Errorf("pull image %s: %w", imageRef, err)
	}
	return nil
}
