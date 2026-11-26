// Package watch runs the panel's background duties: scheduled backups and
// noticing servers that crash.
package watch

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/jobs"
	"github.com/Geordie1071-spec/minectl/internal/notify"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

const (
	// A server that crashes this often is stopped instead of restarted.
	crashLoopCount  = 4
	crashLoopWindow = 10 * time.Minute
	// An oom event this close before a die explains it.
	oomWindow = 30 * time.Second
)

type Watcher struct {
	St     *store.Store
	M      *service.Manager
	Jobs   *jobs.Runner
	Notify func(ctx context.Context, level notify.Level, title, body string)
	Tick   time.Duration
	Now    func() time.Time
	// backup is replaced in tests.
	backup func(ctx context.Context, m *service.Manager, name string) error

	mu      sync.Mutex
	crashes map[string][]time.Time
	ooms    map[string]time.Time
}

func New(st *store.Store, m *service.Manager, runner *jobs.Runner) *Watcher {
	w := &Watcher{St: st, M: m, Jobs: runner, Tick: time.Minute, Now: time.Now}
	discord := notify.NewDiscord()
	w.Notify = func(ctx context.Context, level notify.Level, title, body string) {
		cfg, err := st.GetConfig()
		if err != nil || cfg.DiscordWebhook == "" {
			return
		}
		if err := discord.Send(ctx, cfg.DiscordWebhook, level, title, body); err != nil {
			log.Printf("discord: %v", err)
		}
	}
	return w
}

func (w *Watcher) audit(action, srv, detail string) {
	if err := w.St.AddAudit(domain.AuditEntry{Username: "system", Action: action, Server: srv, Detail: detail}); err != nil {
		log.Printf("audit %s: %v", action, err)
	}
}

func (w *Watcher) manager() *service.Manager {
	m := *w.M
	m.Holder = "panel (scheduler)"
	m.Wait = true
	return &m
}

// RunScheduler starts due backups every Tick until ctx ends.
func (w *Watcher) RunScheduler(ctx context.Context) {
	t := time.NewTicker(w.Tick)
	defer t.Stop()
	for {
		w.tickBackups(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (w *Watcher) tickBackups(ctx context.Context) {
	servers, err := w.St.ListServers()
	if err != nil {
		log.Printf("scheduler: %v", err)
		return
	}
	now := w.Now().UTC()
	for i := range servers {
		s := &servers[i]
		every := service.ScheduleInterval(s.BackupSchedule)
		if every == 0 {
			continue
		}
		if s.AutoBackupNextAt == nil {
			next := now.Add(every)
			s.AutoBackupNextAt = &next
			_ = w.St.SaveServer(s)
			continue
		}
		if now.Before(*s.AutoBackupNextAt) || w.Jobs.Active(s.Name) != nil {
			continue
		}
		// Move the next run forward first, so a failing backup isn't retried
		// every minute.
		next := now.Add(every)
		s.AutoBackupNextAt = &next
		if err := w.St.SaveServer(s); err != nil {
			log.Printf("scheduler: %v", err)
			continue
		}
		w.startBackup(s.Name)
	}
}

func (w *Watcher) startBackup(name string) {
	run := w.backup
	if run == nil {
		run = func(ctx context.Context, m *service.Manager, name string) error {
			_, err := m.CreateBackup(ctx, name)
			return err
		}
	}
	m := w.manager()
	_, err := w.Jobs.Start("backup", name, 0, func(ctx context.Context, ev service.Events) (string, error) {
		ev.Stage("Scheduled backup")
		if err := run(ctx, m, name); err != nil {
			w.Notify(context.Background(), notify.Warning, "Scheduled backup of "+name+" failed", err.Error())
			return "", err
		}
		return "scheduled backup done", nil
	})
	if err != nil {
		log.Printf("scheduler: backup %s: %v", name, err)
		return
	}
	w.audit("server.backup.scheduled", name, "")
}

// RunCrashWatch follows Docker events until ctx ends, reconnecting if the
// stream drops (for example when Docker restarts).
func (w *Watcher) RunCrashWatch(ctx context.Context) {
	if w.M.D == nil {
		return
	}
	for ctx.Err() == nil {
		err := w.M.D.WatchContainers(ctx, func(e docker.ContainerEvent) { w.handle(ctx, e) })
		if ctx.Err() != nil {
			return
		}
		log.Printf("crash watch: %v; reconnecting", err)
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

// classify decides whether a container exit was a crash. Exits during a
// minectl operation (stop, restart, delete, upgrade...) hold the server's
// lock and are deliberate; a clean exit code means the server shut itself
// down normally.
func classify(exitCode int, oom, locked bool) (bool, string) {
	if locked {
		return false, ""
	}
	switch {
	case oom || exitCode == 137:
		return true, "it ran out of memory"
	case exitCode == 0 || exitCode == 143:
		return false, ""
	}
	return true, fmt.Sprintf("it exited with code %d", exitCode)
}

// recordCrash notes a crash and returns how many happened in the window.
func (w *Watcher) recordCrash(name string, at time.Time) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.crashes == nil {
		w.crashes = map[string][]time.Time{}
	}
	recent := []time.Time{}
	for _, t := range w.crashes[name] {
		if at.Sub(t) < crashLoopWindow {
			recent = append(recent, t)
		}
	}
	recent = append(recent, at)
	w.crashes[name] = recent
	return len(recent)
}

func (w *Watcher) handle(ctx context.Context, e docker.ContainerEvent) {
	now := w.Now()
	if e.Action == "oom" {
		w.mu.Lock()
		if w.ooms == nil {
			w.ooms = map[string]time.Time{}
		}
		w.ooms[e.Server] = now
		w.mu.Unlock()
		return
	}
	if e.Action != "die" {
		return
	}
	s, err := w.St.GetServer(e.Server)
	if err != nil || s == nil {
		return
	}
	w.mu.Lock()
	oom := now.Sub(w.ooms[e.Server]) < oomWindow
	delete(w.ooms, e.Server)
	w.mu.Unlock()
	_, locked := w.St.LockHolder(e.Server)
	crashed, reason := classify(e.ExitCode, oom, locked)
	if !crashed {
		return
	}

	var details []string
	if w.M.D != nil && s.ContainerID != "" {
		lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		lines, _ := w.M.D.GetLogs(lctx, s.ContainerID, 120)
		cancel()
		if hint := server.DescribeStartFailure(lines); hint != "" && !oom {
			reason += ": " + hint
		}
		details = server.FailureSummary(lines)
	}

	at := now.UTC()
	s.LastCrashAt, s.LastCrashReason = &at, reason
	_ = w.St.SaveServer(s)
	w.audit("server.crash", s.Name, reason)

	body := fmt.Sprintf("**%s** crashed because %s.", s.Name, reason)
	if len(details) > 0 {
		body += "\n```\n" + strings.Join(details, "\n") + "\n```"
	}
	if n := w.recordCrash(s.Name, now); n >= crashLoopCount {
		w.haltLoop(ctx, s.Name, n)
		w.Notify(ctx, notify.Error, s.Name+" keeps crashing, so it was stopped", body+fmt.Sprintf("\nIt crashed %d times in %s. Fix the problem, then start it again.", n, crashLoopWindow))
		return
	}
	w.Notify(ctx, notify.Error, s.Name+" crashed", body+"\nDocker is restarting it.")
}

func (w *Watcher) haltLoop(ctx context.Context, name string, n int) {
	unlock, err := w.St.Lock(ctx, name, "panel (crash watch): stop")
	if err != nil {
		return
	}
	defer unlock()
	if err := server.Halt(ctx, w.M.D, w.St, name); err != nil {
		log.Printf("crash watch: stop %s: %v", name, err)
		return
	}
	w.mu.Lock()
	delete(w.crashes, name)
	w.mu.Unlock()
	w.audit("server.crash_loop", name, fmt.Sprintf("stopped after %d crashes in %s", n, crashLoopWindow))
}
