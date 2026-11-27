package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

func useTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MINECTL_CONFIG_DIR", dir)
	return dir
}

func TestImportsLegacyJSON(t *testing.T) {
	dir := useTempConfig(t)
	servers := `{"version":1,"servers":[{"name":"survival","mc_type":"paper","mc_version":"26.3","memory_mb":2048},{"name":"modded","mc_type":"fabric","mc_version":"1.20.1"}]}`
	if err := os.WriteFile(filepath.Join(dir, "servers.json"), []byte(servers), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"default_data_dir":"/srv/mc","backup_retention":3}`), 0644); err != nil {
		t.Fatal(err)
	}

	st := New()
	defer st.Close()
	list, err := st.ListServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "survival" || list[1].Name != "modded" || list[0].MemoryMB != 2048 {
		t.Fatalf("imported servers = %+v", list)
	}
	cfg, err := st.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultDataDir != "/srv/mc" || cfg.BackupRetention != 3 {
		t.Fatalf("imported config = %+v", cfg)
	}
	for _, f := range []string{"servers.json", "config.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Errorf("%s should be renamed after import", f)
		}
		if _, err := os.Stat(filepath.Join(dir, f+".migrated")); err != nil {
			t.Errorf("%s.migrated missing: %v", f, err)
		}
	}
}

func TestServerCRUDKeepsInsertionOrder(t *testing.T) {
	useTempConfig(t)
	st := New()
	defer st.Close()
	for _, n := range []string{"b", "a", "c"} {
		if err := st.SaveServer(&domain.Server{Name: n, Status: domain.StatusStopped}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SaveServer(&domain.Server{Name: "b", Status: domain.StatusRunning}); err != nil {
		t.Fatal(err)
	}
	list, _ := st.ListServers()
	if len(list) != 3 || list[0].Name != "b" || list[0].Status != domain.StatusRunning || list[2].Name != "c" {
		t.Fatalf("list = %+v", list)
	}
	if err := st.DeleteServer("a"); err != nil {
		t.Fatal(err)
	}
	if s, _ := st.GetServer("a"); s != nil {
		t.Fatal("a should be deleted")
	}
	if s, _ := st.GetServer("missing"); s != nil {
		t.Fatal("missing server should be nil")
	}
}

func TestConcurrentWritersFromSeparateStores(t *testing.T) {
	useTempConfig(t)
	if err := InitConfigDir(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			st := New()
			defer st.Close()
			for i := 0; i < 25; i++ {
				if err := st.SaveServer(&domain.Server{Name: fmt.Sprintf("w%d-%d", w, i)}); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	st := New()
	defer st.Close()
	list, _ := st.ListServers()
	if len(list) != 100 {
		t.Fatalf("got %d servers, want 100", len(list))
	}
}

func TestLock(t *testing.T) {
	useTempConfig(t)
	a, b := New(), New()
	defer a.Close()
	defer b.Close()
	ctx := context.Background()

	release, err := a.Lock(ctx, "survival", "cli: restart")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Lock(ctx, "survival", "panel: backup")
	var locked *LockedError
	if !errors.As(err, &locked) || locked.Holder != "cli: restart" {
		t.Fatalf("second lock err = %v", err)
	}
	if r, err := b.Lock(ctx, "other", "panel"); err != nil {
		t.Fatalf("lock on a different server should succeed: %v", err)
	} else {
		r()
	}

	release()
	release()
	r2, err := b.Lock(ctx, "survival", "panel: backup")
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	r2()
}

func TestExpiredLockIsTakenOver(t *testing.T) {
	useTempConfig(t)
	st := New()
	defer st.Close()
	db, err := st.open()
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO locks (server, holder, token, acquired_at, expires_at) VALUES ('survival', 'crashed', 'x', ?, ?)`, past, past); err != nil {
		t.Fatal(err)
	}
	release, err := st.Lock(context.Background(), "survival", "cli: start")
	if err != nil {
		t.Fatalf("expired lock should be taken over: %v", err)
	}
	release()
}

func TestRefusesNewerSchema(t *testing.T) {
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := New()
	if _, err := st.ListServers(); err != nil {
		t.Fatal(err)
	}
	db, _ := st.open()
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations)+1)); err != nil {
		t.Fatal(err)
	}
	st.Close()

	_, err := New().ListServers()
	var newer *NewerSchemaError
	if !errors.As(err, &newer) || !strings.Contains(err.Error(), "minectl update") {
		t.Fatalf("err = %v", err)
	}
}
