package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/update"
)

// updateCheckEvery limits GitHub lookups; it allows 60 an hour per address.
const updateCheckEvery = 6 * time.Hour

type updateView struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"`
	// CanApply is false when the panel can't replace its own binary, for
	// example when it's owned by root; the UI then shows the command to run.
	CanApply bool   `json:"can_apply"`
	Error    string `json:"error,omitempty"`
}

type updateCache struct {
	mu      sync.Mutex
	checked time.Time
	rel     *update.Release
	err     error
}

func (a *API) latestRelease(ctx context.Context, refresh bool) (*update.Release, error) {
	c := &a.updates
	c.mu.Lock()
	defer c.mu.Unlock()
	if refresh || time.Since(c.checked) > updateCheckEvery {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		c.rel, c.err = update.Latest(ctx, "")
		c.checked = time.Now()
	}
	return c.rel, c.err
}

func (a *API) getUpdate(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	v := updateView{Current: config.Version}
	rel, err := a.latestRelease(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		v.Error = err.Error()
		writeJSON(w, http.StatusOK, v)
		return
	}
	v.Latest, v.URL = rel.Version, rel.URL
	v.Available = update.Compare(config.Version, rel.Version) < 0
	if exe, err := update.Executable(); err == nil {
		v.CanApply = a.cfg.Restart != nil && update.CanReplace(exe)
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) applyUpdate(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	if a.cfg.Restart == nil {
		writeErr(w, http.StatusConflict, "this panel can't restart itself; run: sudo minectl update")
		return
	}
	if n := a.jobs.Running(); n > 0 {
		writeErr(w, http.StatusConflict, "wait for the running jobs to finish first")
		return
	}
	rel, err := a.latestRelease(r.Context(), true)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if update.Compare(config.Version, rel.Version) >= 0 {
		writeErr(w, http.StatusConflict, "minectl "+config.Version+" is already the latest release")
		return
	}
	exe, err := update.Executable()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !update.CanReplace(exe) {
		writeErr(w, http.StatusConflict, "the panel can't replace "+exe+"; run on the machine: sudo minectl update")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	if err := update.Apply(ctx, rel, exe); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	a.audit(r, c, "panel.update", "", config.Version+" → "+rel.Version)
	writeJSON(w, http.StatusAccepted, map[string]string{"version": rel.Version})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	// Restart after this reply has gone out.
	go func() {
		time.Sleep(500 * time.Millisecond)
		a.cfg.Restart()
	}()
}
