package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/server"
)

// ScheduleInterval returns how often a backup schedule runs, or 0 for none.
func ScheduleInterval(schedule string) time.Duration {
	switch schedule {
	case domain.ScheduleHourly:
		return time.Hour
	case domain.Schedule6Hours:
		return 6 * time.Hour
	case domain.ScheduleDaily:
		return 24 * time.Hour
	}
	return 0
}

// SetBackupSchedule sets or clears ("" or "off") a server's automatic
// backups. The first one runs one interval from now.
func (m *Manager) SetBackupSchedule(name, schedule string) (*domain.Server, error) {
	if schedule == "off" || schedule == "none" {
		schedule = ""
	}
	every := ScheduleInterval(schedule)
	if schedule != "" && every == 0 {
		return nil, fmt.Errorf("invalid schedule %q; use hourly, 6h, daily, or off", schedule)
	}
	s, err := m.get(name)
	if err != nil {
		return nil, err
	}
	s.BackupSchedule = schedule
	s.AutoBackupNextAt = nil
	if every > 0 {
		next := time.Now().UTC().Add(every)
		s.AutoBackupNextAt = &next
	}
	return s, m.St.SaveServer(s)
}

// SetProperties replaces a server's game settings and recreates its
// container so they take effect.
func (m *Manager) SetProperties(ctx context.Context, name string, p domain.Properties, ev Events) error {
	if err := server.ValidateProperties(&p); err != nil {
		return err
	}
	unlock, err := m.lock(ctx, name, "settings")
	if err != nil {
		return err
	}
	defer unlock()
	s, err := m.get(name)
	if err != nil {
		return err
	}
	unset := server.UnsetKeys(s.Properties, &p)
	if p == (domain.Properties{}) {
		s.Properties = nil
	} else {
		s.Properties = &p
	}
	if err := m.St.SaveServer(s); err != nil {
		return err
	}
	if err := server.ResetProperties(s.DataDir, unset); err != nil {
		ev.Warn(fmt.Sprintf("couldn't reset %s in server.properties: %v", strings.Join(unset, ", "), err))
	}
	ev.Info("Game settings saved")
	return m.apply(ctx, name, ev)
}
