package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

const readyTimeout = 10 * time.Minute

type startFailedError struct{ msg string }

func (e *startFailedError) Error() string { return e.msg }

func isStartFailed(err error) bool {
	var sf *startFailedError
	return errors.As(err, &sf)
}

var logPrefix = regexp.MustCompile(`^(\[[^\]]*\]\s*)+:?\s*`)

type spinner struct {
	mu      sync.Mutex
	label   string
	detail  string
	started time.Time
	stop    chan struct{}
	done    chan struct{}
}

func startSpinner(label string) *spinner {
	sp := &spinner{label: label, started: time.Now(), stop: make(chan struct{}), done: make(chan struct{})}
	go sp.run()
	return sp
}

func (sp *spinner) run() {
	defer close(sp.done)
	if !interactive() {
		<-sp.stop
		return
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for i := 0; ; i++ {
		sp.mu.Lock()
		elapsed := time.Since(sp.started).Round(time.Second)
		line := fmt.Sprintf("%s %s (%s)", frames[i%len(frames)], sp.label, elapsed)
		if sp.detail != "" {
			line += "  " + sp.detail
		}
		sp.mu.Unlock()
		fmt.Fprintf(os.Stderr, "\r\033[K%s", truncate(line, 100))
		select {
		case <-sp.stop:
			fmt.Fprint(os.Stderr, "\r\033[K")
			return
		case <-t.C:
		}
	}
}

func (sp *spinner) setDetail(s string) {
	sp.mu.Lock()
	sp.detail = s
	sp.mu.Unlock()
}

func (sp *spinner) end() time.Duration {
	close(sp.stop)
	<-sp.done
	return time.Since(sp.started).Round(time.Second)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func waitIfRestarted(ctx context.Context, d *docker.Client, st *store.Store, name string, since time.Time) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if noWait || s.Status != domain.StatusRunning || s.LastStartedAt.Before(since) {
		return nil
	}
	if err := waitForReady(ctx, d, s, s.LastStartedAt); err != nil {
		if isStartFailed(err) {
			_ = server.Halt(context.Background(), d, st, name)
			return fmt.Errorf("%w\n\nStopped %s so it doesn't keep restarting.", err, name)
		}
		return err
	}
	return nil
}

func waitForReady(ctx context.Context, d *docker.Client, s *domain.Server, since time.Time) error {
	timeout := readyTimeout
	if s.HasModpack() {
		timeout = 2 * readyTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var sp *spinner
	var onLine func(string)
	if !quiet && !jsonOut {
		sp = startSpinner("Starting Minecraft")
		onLine = func(line string) {
			sp.setDetail(logPrefix.ReplaceAllString(strings.TrimSpace(line), ""))
		}
	}
	recent, err := server.WaitReady(ctx, d, s, since, onLine)
	var took time.Duration
	if sp != nil {
		took = sp.end()
	}

	switch {
	case err == nil:
		if !quiet && !jsonOut {
			fmt.Printf("Ready in %s\n", took)
		}
		return nil
	case errors.Is(err, context.Canceled):
		fmt.Fprintf(os.Stderr, "Stopped waiting; %s is still starting in the background. Follow with: minectl logs %s -f\n", s.Name, s.Name)
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s did not finish starting within %s; check: minectl logs %s", s.Name, timeout, s.Name)
	}

	msg := fmt.Sprintf("%s failed to start", s.Name)
	if hint := server.DescribeStartFailure(recent); hint != "" {
		msg += ": " + hint
	}
	if summary := server.FailureSummary(recent); len(summary) > 0 {
		msg += "\n\nKey log lines:\n  " + strings.Join(summary, "\n  ")
	}
	msg += fmt.Sprintf("\n\nFull log: %s", filepath.Join(s.DataDir, "logs", "latest.log"))
	return &startFailedError{msg: msg}
}
