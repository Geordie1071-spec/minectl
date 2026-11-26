package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
	"github.com/google/uuid"
)

func CreateBackup(ctx context.Context, d *docker.Client, st *store.Store, name string) (*domain.Backup, error) {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	if err := SyncStatus(ctx, d, st, s); err != nil {
		return nil, err
	}
	if s.Status == domain.StatusRunning {
		_, _ = d.ExecRcon(ctx, s.ContainerID, "save-off")
		defer func() { _, _ = d.ExecRcon(ctx, s.ContainerID, "save-on") }()
		_, _ = d.ExecRcon(ctx, s.ContainerID, "save-all flush")
		time.Sleep(2 * time.Second)
	}

	if err := os.MkdirAll(s.BackupDir, 0755); err != nil {
		return nil, err
	}
	now := time.Now()
	id := "bak_" + uuid.New().String()[:8]
	path := filepath.Join(s.BackupDir, fmt.Sprintf("bak_%s_%s.tar.gz", now.Format("20060102_150405"), id[4:]))
	if err := writeArchiveFile(s.DataDir, path); err != nil {
		return nil, fmt.Errorf("write backup: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	b := domain.Backup{ID: id, Path: path, SizeBytes: info.Size(), CreatedAt: now.UTC()}
	s.Backups = append(s.Backups, b)

	cfg, err := st.GetConfig()
	if err != nil {
		return nil, err
	}
	s.Backups = pruneBackups(s.Backups, cfg.BackupRetention)
	return &b, st.SaveServer(s)
}

func writeArchiveFile(srcDir, path string) error {
	tmp := path + ".partial"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := writeTarGz(srcDir, f); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func pruneBackups(backups []domain.Backup, keep int) []domain.Backup {
	if keep <= 0 || len(backups) <= keep {
		return backups
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].CreatedAt.Before(backups[j].CreatedAt) })
	drop := len(backups) - keep
	for _, b := range backups[:drop] {
		_ = os.Remove(b.Path)
	}
	return append([]domain.Backup(nil), backups[drop:]...)
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
	for i := range s.Backups {
		if s.Backups[i].ID == backupID {
			target = &s.Backups[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}
	if _, err := os.Stat(target.Path); err != nil {
		return fmt.Errorf("backup file missing: %w", err)
	}

	if err := SyncStatus(ctx, d, st, s); err != nil {
		return err
	}
	wasRunning := s.Status == domain.StatusRunning
	if err := Stop(ctx, d, st, name, false, 30, nil); err != nil {
		return err
	}

	if err := restoreDataDir(target.Path, s.DataDir); err != nil {
		return err
	}

	if wasRunning {
		_, err = Start(ctx, d, st, name, nil)
	}
	return err
}

func restoreDataDir(archivePath, dataDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	old := dataDir + ".pre-restore"
	_ = os.RemoveAll(old)
	if err := os.Rename(dataDir, old); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("move current data aside: %w", err)
	}
	rollback := func(cause error) error {
		_ = os.RemoveAll(dataDir)
		if _, statErr := os.Stat(old); statErr == nil {
			_ = os.Rename(old, dataDir)
		}
		return fmt.Errorf("restore failed, previous data kept: %w", cause)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return rollback(err)
	}
	if err := extractTarGz(f, dataDir); err != nil {
		return rollback(err)
	}
	return os.RemoveAll(old)
}
