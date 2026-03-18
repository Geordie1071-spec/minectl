package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/minectl/minectl/internal/domain"
	"github.com/minectl/minectl/internal/docker"
	"github.com/minectl/minectl/internal/store"
)

func CreateBackup(ctx context.Context, d *docker.Client, st *store.Store, name string) (*domain.Backup, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	if s.Status == domain.StatusRunning {
		_, _ = d.ExecRcon(ctx, s.ContainerID, "save-off")
		_, _ = d.ExecRcon(ctx, s.ContainerID, "save-all")
		time.Sleep(2 * time.Second)
	}
	ts := time.Now().Format("20060102_150405")
	id := "bak_" + uuid.New().String()[:8]
	fname := fmt.Sprintf("bak_%s.tar.gz", ts)
	path := filepath.Join(s.BackupDir, fname)
	if err := os.MkdirAll(s.BackupDir, 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	info, _ := os.Stat(path)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	if s.Status == domain.StatusRunning {
		_, _ = d.ExecRcon(ctx, s.ContainerID, "save-on")
	}
	b := &domain.Backup{ID: id, Path: path, SizeBytes: size, CreatedAt: time.Now().UTC()}
	s.Backups = append(s.Backups, *b)
	return b, st.SaveServer(s)
}

func ListBackups(st *store.Store, name string) ([]domain.Backup, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	return s.Backups, nil
}

func RestoreBackup(ctx context.Context, d *docker.Client, st *store.Store, name, backupID string) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	var target *domain.Backup
	for _, b := range s.Backups {
		if b.ID == backupID {
			target = &b
			break
		}
	}
	if target == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}
	_ = Stop(ctx, d, st, name, true, 10, nil)
	_, err = Start(ctx, d, st, name, nil)
	return err
}
