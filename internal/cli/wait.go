package cli

import (
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
)

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
		line := fmt.Sprintf("%s %s %s", paintFor(os.Stderr, sgrAccent, frames[i%len(frames)]), sp.label, paintFor(os.Stderr, sgrDim, "("+elapsed.String()+")"))
		if sp.detail != "" {
			line += "  " + truncate(sp.detail, 70)
		}
		sp.mu.Unlock()
		fmt.Fprintf(os.Stderr, "\r\033[K%s", line)
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
