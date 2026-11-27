// Package jobs runs long server operations in the background and keeps their
// progress so clients can follow them live or catch up after a reload.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/store"
	"github.com/google/uuid"
)

const maxEvents = 400

const (
	EventProgress = "progress"
	EventStage    = "stage"
	EventLog      = "log"
	EventInfo     = "info"
	EventWarn     = "warn"
	EventDone     = "done"
	EventError    = "error"
)

// Func does the work. The string it returns becomes the final "done" message.
type Func func(ctx context.Context, ev service.Events) (string, error)

type BusyError struct {
	Server string
	JobID  string
	Kind   string
}

func (e *BusyError) Error() string {
	return fmt.Sprintf("%s is busy: %s is running (job %s)", e.Server, e.Kind, e.JobID)
}

var ErrNotFound = errors.New("job not found")

type Runner struct {
	st       *store.Store
	ctx      context.Context
	mu       sync.Mutex
	active   map[string]*job
	byServer map[string]*job
	wg       sync.WaitGroup
}

type job struct {
	mu       sync.Mutex
	rec      domain.Job
	progress *domain.JobEvent
	seq      int64
	changed  chan struct{}
	done     bool
}

// New marks jobs a previous process left running as failed. ctx is passed to
// every job and should be cancelled on shutdown.
func New(ctx context.Context, st *store.Store) (*Runner, error) {
	if err := st.FailRunningJobs("interrupted: the panel restarted while this was running"); err != nil {
		return nil, err
	}
	return &Runner{st: st, ctx: ctx, active: map[string]*job{}, byServer: map[string]*job{}}, nil
}

func (r *Runner) Start(kind, server string, userID int64, fn Func) (*domain.Job, error) {
	r.mu.Lock()
	if cur, ok := r.byServer[server]; ok {
		r.mu.Unlock()
		return nil, &BusyError{Server: server, JobID: cur.rec.ID, Kind: cur.rec.Kind}
	}
	j := &job{
		rec: domain.Job{
			ID:        uuid.NewString(),
			Kind:      kind,
			Server:    server,
			UserID:    userID,
			Status:    domain.JobRunning,
			CreatedAt: time.Now().UTC(),
		},
		changed: make(chan struct{}),
	}
	if err := r.st.InsertJob(&j.rec); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	r.active[j.rec.ID] = j
	r.byServer[server] = j
	r.wg.Add(1)
	r.mu.Unlock()

	go r.run(j, fn)
	return j.snapshot(), nil
}

func (r *Runner) run(j *job, fn Func) {
	defer r.wg.Done()
	msg, err := func() (msg string, err error) {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("job %s (%s %s) panicked: %v", j.rec.ID, j.rec.Kind, j.rec.Server, p)
				err = fmt.Errorf("internal error: %v", p)
			}
		}()
		return fn(r.ctx, &events{j: j})
	}()

	j.mu.Lock()
	now := time.Now().UTC()
	j.rec.FinishedAt = &now
	if err != nil {
		j.rec.Status, j.rec.Error = domain.JobFailed, err.Error()
		j.appendLocked(EventError, 0, err.Error())
	} else {
		j.rec.Status = domain.JobSucceeded
		j.appendLocked(EventDone, 0, msg)
	}
	rec := j.rec
	rec.Events = append([]domain.JobEvent(nil), j.rec.Events...)
	j.mu.Unlock()

	// Save and release the server before followers are told the job is
	// over, so a client can start the next job as soon as it sees the end.
	if err := r.st.FinishJob(&rec); err != nil {
		log.Printf("save job %s: %v", rec.ID, err)
	}
	r.mu.Lock()
	delete(r.active, rec.ID)
	if r.byServer[rec.Server] == j {
		delete(r.byServer, rec.Server)
	}
	r.mu.Unlock()

	j.mu.Lock()
	j.done = true
	j.notifyLocked()
	j.mu.Unlock()
}

// Wait blocks until every running job has finished.
func (r *Runner) Wait() { r.wg.Wait() }

func (r *Runner) live(id string) *job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active[id]
}

// Running counts the jobs that haven't finished.
func (r *Runner) Running() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.active)
}

// Active returns the running job for a server, if any.
func (r *Runner) Active(server string) *domain.Job {
	r.mu.Lock()
	j := r.byServer[server]
	r.mu.Unlock()
	if j == nil {
		return nil
	}
	return j.snapshot()
}

func (r *Runner) Get(id string) (*domain.Job, error) {
	if j := r.live(id); j != nil {
		return j.snapshot(), nil
	}
	rec, err := r.st.GetJob(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, ErrNotFound
	}
	return rec, nil
}

// Follow sends every event after afterSeq, then keeps sending new ones until
// the job ends or ctx is cancelled. Progress updates are sent with Seq 0.
func (r *Runner) Follow(ctx context.Context, id string, afterSeq int64, send func(domain.JobEvent) error) error {
	j := r.live(id)
	if j == nil {
		rec, err := r.Get(id)
		if err != nil {
			return err
		}
		for _, e := range rec.Events {
			if e.Seq > afterSeq {
				if err := send(e); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var lastProgress *domain.JobEvent
	for {
		j.mu.Lock()
		var pending []domain.JobEvent
		for _, e := range j.rec.Events {
			if e.Seq > afterSeq {
				pending = append(pending, e)
			}
		}
		progress, changed, done := j.progress, j.changed, j.done
		j.mu.Unlock()

		for _, e := range pending {
			if err := send(e); err != nil {
				return err
			}
			afterSeq = e.Seq
		}
		if progress != nil && progress != lastProgress && !done {
			lastProgress = progress
			if err := send(*progress); err != nil {
				return err
			}
		}
		if done {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (j *job) snapshot() *domain.Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	rec := j.rec
	rec.Events = append([]domain.JobEvent(nil), j.rec.Events...)
	return &rec
}

func (j *job) notifyLocked() {
	close(j.changed)
	j.changed = make(chan struct{})
}

func (j *job) appendLocked(typ string, percent int, msg string) {
	j.seq++
	j.rec.Events = append(j.rec.Events, domain.JobEvent{Seq: j.seq, Time: time.Now().UTC(), Type: typ, Percent: percent, Message: msg})
	if len(j.rec.Events) > maxEvents {
		j.rec.Events = append([]domain.JobEvent(nil), j.rec.Events[len(j.rec.Events)-maxEvents:]...)
	}
	j.notifyLocked()
}

func (j *job) add(typ, msg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.appendLocked(typ, 0, msg)
}

type events struct{ j *job }

func (e *events) Progress(percent int, msg string) {
	j := e.j
	j.mu.Lock()
	defer j.mu.Unlock()
	if p := j.progress; p != nil && p.Percent == percent && p.Message == msg {
		return
	}
	j.progress = &domain.JobEvent{Time: time.Now().UTC(), Type: EventProgress, Percent: percent, Message: msg}
	j.notifyLocked()
}

func (e *events) Stage(label string) { e.j.add(EventStage, label) }
func (e *events) Log(line string)    { e.j.add(EventLog, line) }
func (e *events) Info(msg string)    { e.j.add(EventInfo, msg) }
func (e *events) Warn(msg string)    { e.j.add(EventWarn, msg) }
