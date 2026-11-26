package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/jobs"
)

const heartbeat = 15 * time.Second

// sse writes server-sent events. Writes are serialised so the heartbeat
// can't interleave with an event.
type sse struct {
	mu sync.Mutex
	w  http.ResponseWriter
	rc *http.ResponseController
}

func newSSE(ctx context.Context, w http.ResponseWriter) *sse {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	s := &sse{w: w, rc: http.NewResponseController(w)}
	_ = s.rc.Flush()
	go func() {
		t := time.NewTicker(heartbeat)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.mu.Lock()
				_, err := fmt.Fprint(s.w, ": ping\n\n")
				if err == nil {
					err = s.rc.Flush()
				}
				s.mu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()
	return s
}

func (s *sse) send(event string, id int64, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id > 0 {
		if _, err := fmt.Fprintf(s.w, "id: %d\n", id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return s.rc.Flush()
}

func (a *API) streamLogs(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	if !a.needDocker(w) {
		return
	}
	if c.server.ContainerID == "" {
		writeErr(w, http.StatusConflict, c.server.Name+" has no container yet; start it first")
		return
	}
	tail := 200
	if t, err := strconv.Atoi(r.URL.Query().Get("tail")); err == nil && t >= 0 && t <= 5000 {
		tail = t
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	s := newSSE(ctx, w)

	lines := make(chan string, 256)
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.m.D.FollowLogs(ctx, c.server.ContainerID, tail, lines)
		close(lines)
	}()
	for line := range lines {
		if err := s.send("log", 0, line); err != nil {
			return
		}
	}
	if err := <-errCh; err != nil && ctx.Err() == nil {
		_ = s.send("error", 0, err.Error())
		return
	}
	_ = s.send("end", 0, c.server.Name+" stopped")
}

type statsView struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryBytes   uint64  `json:"memory_bytes"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
}

func (a *API) streamStats(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	if !a.running(w, c) {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	s := newSSE(ctx, w)
	err := a.m.D.StreamStats(ctx, c.server.ContainerID, func(st *docker.ContainerStats) {
		if err := s.send("stats", 0, statsView{st.CPUPercent, st.MemoryUsage, st.MemoryLimit, st.MemoryPercent}); err != nil {
			cancel()
		}
	})
	if err != nil && ctx.Err() == nil {
		_ = s.send("error", 0, err.Error())
		return
	}
	if ctx.Err() == nil {
		_ = s.send("end", 0, c.server.Name+" stopped")
	}
}

// jobFor loads a job the caller may see. Everyone else gets 404.
func (a *API) jobFor(w http.ResponseWriter, c *reqCtx, id string) *domain.Job {
	j, err := a.jobs.Get(id)
	if errors.Is(err, jobs.ErrNotFound) || (err == nil && !a.canView(c, j.Server)) {
		writeErr(w, http.StatusNotFound, "job not found")
		return nil
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return nil
	}
	return j
}

func (a *API) getJob(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	if j := a.jobFor(w, c, r.PathValue("id")); j != nil {
		writeJSON(w, http.StatusOK, j)
	}
}

// jobEvents streams a job's events. Reconnecting clients resume after
// Last-Event-ID (or ?after=); the stream ends with an "end" event carrying
// the final job.
func (a *API) jobEvents(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	id := r.PathValue("id")
	if a.jobFor(w, c, id) == nil {
		return
	}
	after, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if q := r.URL.Query().Get("after"); q != "" {
		after, _ = strconv.ParseInt(q, 10, 64)
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	s := newSSE(ctx, w)
	err := a.jobs.Follow(ctx, id, after, func(e domain.JobEvent) error {
		return s.send(e.Type, e.Seq, e)
	})
	if err != nil {
		return
	}
	if j, err := a.jobs.Get(id); err == nil {
		j.Events = nil
		_ = s.send("end", 0, j)
	}
}

func (a *API) serverJobs(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	list, err := a.st.ListJobs(c.server.Name, 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) listJobs(w http.ResponseWriter, r *http.Request, c *reqCtx) {
	list, err := a.st.ListJobs("", 100)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}
