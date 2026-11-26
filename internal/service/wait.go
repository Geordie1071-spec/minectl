package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/server"
)

const readyTimeout = 10 * time.Minute

func (m *Manager) WaitReady(ctx context.Context, s *domain.Server, since time.Time, ev Events) error {
	timeout := readyTimeout
	if s.HasModpack() {
		timeout = 2 * readyTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ev.Stage("Starting Minecraft")
	start := time.Now()
	recent, err := server.WaitReady(ctx, m.D, s, since, ev.Log)
	switch {
	case err == nil:
		ev.Info(fmt.Sprintf("Ready in %s", time.Since(start).Round(time.Second)))
		return nil
	case errors.Is(err, context.Canceled):
		ev.Info(fmt.Sprintf("Stopped waiting; %s is still starting in the background. Follow with: minectl logs %s -f", s.Name, s.Name))
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s did not finish starting within %s; check: minectl logs %s", s.Name, timeout, s.Name)
	}
	return &StartFailedError{
		Server:  s.Name,
		Hint:    server.DescribeStartFailure(recent),
		Summary: server.FailureSummary(recent),
		LogPath: filepath.Join(s.DataDir, "logs", "latest.log"),
	}
}

func (m *Manager) waitAfterStart(ctx context.Context, name string, since time.Time, ev Events) error {
	s, err := m.get(name)
	if err != nil {
		return err
	}
	if !m.Wait || s.Status != domain.StatusRunning || s.LastStartedAt.Before(since) {
		return nil
	}
	err = m.WaitReady(ctx, s, s.LastStartedAt, ev)
	var sf *StartFailedError
	if errors.As(err, &sf) {
		_ = server.Halt(context.Background(), m.D, m.St, name)
		sf.Cleanup = fmt.Sprintf("Stopped %s so it doesn't keep restarting.", name)
	}
	return err
}
