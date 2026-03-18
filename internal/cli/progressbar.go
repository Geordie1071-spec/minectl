package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/minectl/minectl/internal/server"
)

type ProgressBar struct {
	mu         sync.Mutex
	last       int
	done       bool
	width      int
	prefix     string
	finalized  bool
}

func NewProgressBar(prefix string) *ProgressBar {
	return &ProgressBar{
		last:  0,
		done:  false,
		width: 30,
		prefix: strings.TrimSpace(prefix),
	}
}

func (p *ProgressBar) End() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finalized {
		return
	}
	fmt.Fprintln(os.Stderr)
	p.finalized = true
}

func (p *ProgressBar) Callback(percent int, msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.done {
		return
	}

	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	if percent == p.last && msg == "" {
		return
	}
	p.last = percent

	filled := int(float64(percent) / 100.0 * float64(p.width))
	if filled < 0 {
		filled = 0
	}
	if filled > p.width {
		filled = p.width
	}

	bar := strings.Repeat("#", filled) + strings.Repeat("-", p.width-filled)
	suffix := strings.TrimSpace(msg)
	if p.prefix != "" && suffix != "" {
		suffix = p.prefix + ": " + suffix
	} else if p.prefix != "" {
		suffix = p.prefix
	}

	if percent >= 100 {
		fmt.Fprintf(os.Stderr, "\r[%s] %3d%% %s", bar, percent, suffix)
		fmt.Fprintln(os.Stderr)
		p.done = true
		p.finalized = true
		return
	}

	fmt.Fprintf(os.Stderr, "\r[%s] %3d%% %s", bar, percent, suffix)
}

func (p *ProgressBar) ServerProgress() server.ProgressFunc {
	return p.Callback
}

