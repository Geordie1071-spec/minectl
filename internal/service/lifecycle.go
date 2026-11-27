package service

import (
	"context"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/server"
)

func (m *Manager) Start(ctx context.Context, name string, ev Events) (*domain.Server, error) {
	unlock, err := m.lock(ctx, name, "start")
	if err != nil {
		return nil, err
	}
	defer unlock()
	before := time.Now()
	s, err := server.Start(ctx, m.D, m.St, name, ev.Progress)
	if err != nil {
		return nil, err
	}
	if err := m.waitAfterStart(ctx, name, before, ev); err != nil {
		return nil, err
	}
	return s, nil
}

func (m *Manager) Stop(ctx context.Context, name string, force bool, timeoutSec int, ev Events) error {
	unlock, err := m.lock(ctx, name, "stop")
	if err != nil {
		return err
	}
	defer unlock()
	return server.Stop(ctx, m.D, m.St, name, force, timeoutSec, ev.Progress)
}

func (m *Manager) Restart(ctx context.Context, name string, ev Events) error {
	unlock, err := m.lock(ctx, name, "restart")
	if err != nil {
		return err
	}
	defer unlock()
	before := time.Now()
	if err := server.Restart(ctx, m.D, m.St, name, ev.Progress); err != nil {
		return err
	}
	return m.waitAfterStart(ctx, name, before, ev)
}

func (m *Manager) Delete(ctx context.Context, name string, purge bool, ev Events) error {
	unlock, err := m.lock(ctx, name, "delete")
	if err != nil {
		return err
	}
	defer unlock()
	if s, err := m.St.GetServer(name); err == nil && s != nil && s.Public != nil {
		m.unexpose(ctx, s, ev)
		if err := m.St.SaveServer(s); err != nil {
			return err
		}
	}
	return server.Delete(ctx, m.D, m.St, name, purge, ev.Progress)
}

func (m *Manager) CreateBackup(ctx context.Context, name string) (*domain.Backup, error) {
	unlock, err := m.lock(ctx, name, "backup")
	if err != nil {
		return nil, err
	}
	defer unlock()
	return server.CreateBackup(ctx, m.D, m.St, name)
}

func (m *Manager) RestoreBackup(ctx context.Context, name, backupID string, ev Events) error {
	unlock, err := m.lock(ctx, name, "restore")
	if err != nil {
		return err
	}
	defer unlock()
	before := time.Now()
	if err := server.RestoreBackup(ctx, m.D, m.St, name, backupID); err != nil {
		return err
	}
	ev.Info("Restored " + backupID)
	return m.waitAfterStart(ctx, name, before, ev)
}

func (m *Manager) Upgrade(ctx context.Context, name, version string, noBackup bool, ev Events) error {
	unlock, err := m.lock(ctx, name, "upgrade")
	if err != nil {
		return err
	}
	defer unlock()
	before := time.Now()
	if err := server.Upgrade(ctx, m.D, m.St, name, version, noBackup); err != nil {
		return err
	}
	return m.waitAfterStart(ctx, name, before, ev)
}
