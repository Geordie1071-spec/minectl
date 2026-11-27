package watch

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/jobs"
	"github.com/Geordie1071-spec/minectl/internal/notify"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		code         int
		oom, locked  bool
		crash        bool
		reasonPrefix string
	}{
		{0, false, false, false, ""},
		{143, false, false, false, ""},
		{1, false, false, true, "it exited with code 1"},
		{137, false, false, true, "it ran out of memory"},
		{1, true, false, true, "it ran out of memory"},
		{137, false, true, false, ""}, // killed by delete/stop timeout
		{1, false, true, false, ""},
	}
	for _, c := range cases {
		crash, reason := classify(c.code, c.oom, c.locked)
		if crash != c.crash || reason != c.reasonPrefix {
			t.Errorf("classify(%d, oom=%v, locked=%v) = %v %q", c.code, c.oom, c.locked, crash, reason)
		}
	}
}

func TestRecordCrashWindow(t *testing.T) {
	w := &Watcher{}
	t0 := time.Unix(1000, 0)
	for i, want := range []int{1, 2, 3} {
		if n := w.recordCrash("a", t0.Add(time.Duration(i)*time.Minute)); n != want {
			t.Fatalf("crash %d: count %d", i, n)
		}
	}
	if n := w.recordCrash("b", t0); n != 1 {
		t.Fatal("servers are counted separately")
	}
	if n := w.recordCrash("a", t0.Add(11*time.Minute)); n != 2 {
		t.Fatalf("old crashes should age out, got %d", n)
	}
}

func TestScheduledBackups(t *testing.T) {
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := store.New()
	defer st.Close()
	runner, err := jobs.New(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	m := &service.Manager{St: st}
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

	var mu sync.Mutex
	var ran []string
	var notes []string
	w := New(st, m, runner)
	w.Now = func() time.Time { return now }
	w.backup = func(_ context.Context, _ *service.Manager, name string) error {
		mu.Lock()
		ran = append(ran, name)
		mu.Unlock()
		return nil
	}
	w.Notify = func(_ context.Context, _ notify.Level, title, _ string) { notes = append(notes, title) }

	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)
	for _, s := range []domain.Server{
		{Name: "due", BackupSchedule: domain.ScheduleHourly, AutoBackupNextAt: &past},
		{Name: "later", BackupSchedule: domain.ScheduleDaily, AutoBackupNextAt: &future},
		{Name: "new", BackupSchedule: domain.Schedule6Hours},
		{Name: "off"},
	} {
		s := s
		if err := st.SaveServer(&s); err != nil {
			t.Fatal(err)
		}
	}

	w.tickBackups(context.Background())
	runner.Wait()
	if len(ran) != 1 || ran[0] != "due" {
		t.Fatalf("ran = %v, want [due]", ran)
	}
	due, _ := st.GetServer("due")
	if want := now.Add(time.Hour); due.AutoBackupNextAt == nil || !due.AutoBackupNextAt.Equal(want) {
		t.Fatalf("next run for due = %v, want %v", due.AutoBackupNextAt, want)
	}
	fresh, _ := st.GetServer("new")
	if want := now.Add(6 * time.Hour); fresh.AutoBackupNextAt == nil || !fresh.AutoBackupNextAt.Equal(want) {
		t.Fatalf("new schedule's first run = %v, want %v", fresh.AutoBackupNextAt, want)
	}
	if entries, _ := st.ListAudit(10, 0); len(entries) != 1 || entries[0].Action != "server.backup.scheduled" {
		t.Fatalf("audit = %+v", entries)
	}

	// Nothing is due a second time in the same minute.
	w.tickBackups(context.Background())
	runner.Wait()
	if len(ran) != 1 {
		t.Fatalf("backup ran again: %v", ran)
	}
}

func TestRenewPortForwards(t *testing.T) {
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := store.New()
	defer st.Close()
	runner, err := jobs.New(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	w := New(st, &service.Manager{St: st}, runner)
	w.Now = func() time.Time { return now }
	var renewed []string
	w.renew = func(_ context.Context, _ *service.Manager, name string) error {
		renewed = append(renewed, name)
		return errors.New("router unplugged")
	}
	old := now.Add(-time.Hour)
	for _, s := range []domain.Server{
		{Name: "stale", Public: &domain.Public{Method: "upnp", RefreshedAt: old}},
		{Name: "fresh", Public: &domain.Public{Method: "nat-pmp", RefreshedAt: now.Add(-time.Minute)}},
		{Name: "tunnel", Public: &domain.Public{Method: "playit", RefreshedAt: old}},
		{Name: "private"},
	} {
		s := s
		_ = st.SaveServer(&s)
	}
	w.tickPublic(context.Background())
	if len(renewed) != 1 || renewed[0] != "stale" {
		t.Fatalf("renewed = %v, want [stale]", renewed)
	}
	// A failed renewal waits for the next round instead of retrying every minute.
	now = now.Add(time.Minute)
	w.tickPublic(context.Background())
	if len(renewed) != 1 {
		t.Fatalf("retried too soon: %v", renewed)
	}
	now = now.Add(renewEvery)
	w.tickPublic(context.Background())
	if len(renewed) < 2 || renewed[1] != "stale" {
		t.Fatalf("should retry after %s: %v", renewEvery, renewed)
	}
}

func TestPanelBackups(t *testing.T) {
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := store.New()
	defer st.Close()
	runner, err := jobs.New(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	w := New(st, &service.Manager{St: st}, runner)
	w.Now = func() time.Time { return now }
	count := func() int {
		entries, _ := os.ReadDir(PanelBackupDir())
		return len(entries)
	}

	w.tickPanelBackup()
	if count() != 1 {
		t.Fatalf("first tick should back up, have %d", count())
	}
	now = now.Add(2 * time.Hour)
	w.tickPanelBackup()
	if count() != 1 {
		t.Fatalf("a second backup the same day: %d", count())
	}
	for i := 0; i < 9; i++ {
		now = now.Add(25 * time.Hour)
		w.tickPanelBackup()
	}
	if count() != panelBackupsKept {
		t.Fatalf("kept %d, want %d", count(), panelBackupsKept)
	}
	fi, _ := os.Stat(PanelBackupDir())
	if fi.Mode().Perm() != 0700 {
		t.Fatalf("backup dir mode %v", fi.Mode().Perm())
	}
}
