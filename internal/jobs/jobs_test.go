package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

func newRunner(t *testing.T) (*Runner, *store.Store) {
	t.Helper()
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := store.New()
	t.Cleanup(func() { st.Close() })
	r, err := New(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	return r, st
}

func collect(t *testing.T, r *Runner, id string, after int64) []domain.JobEvent {
	t.Helper()
	var got []domain.JobEvent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Follow(ctx, id, after, func(e domain.JobEvent) error { got = append(got, e); return nil }); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestJobLifecycle(t *testing.T) {
	r, st := newRunner(t)
	release := make(chan struct{})
	j, err := r.Start("start", "alpha", 7, func(ctx context.Context, ev service.Events) (string, error) {
		ev.Stage("Starting")
		ev.Progress(50, "pulling")
		<-release
		ev.Log("Done (3.2s)!")
		return "ready", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Start("stop", "alpha", 7, nil); err == nil {
		t.Fatal("second job on the same server should be refused")
	} else {
		var busy *BusyError
		if !errors.As(err, &busy) || busy.JobID != j.ID {
			t.Fatalf("want BusyError for %s, got %v", j.ID, err)
		}
	}
	if r.Active("alpha") == nil {
		t.Fatal("job should be active")
	}

	done := make(chan []domain.JobEvent)
	go func() { done <- collect(t, r, j.ID, 0) }()
	time.Sleep(50 * time.Millisecond)
	close(release)
	got := <-done

	var types []string
	for _, e := range got {
		types = append(types, e.Type)
	}
	want := fmt.Sprint([]string{EventStage, EventProgress, EventLog, EventDone})
	if fmt.Sprint(types) != want {
		t.Fatalf("events %v, want %v", types, want)
	}
	r.Wait()

	rec, err := st.GetJob(j.ID)
	if err != nil || rec == nil {
		t.Fatal(err)
	}
	if rec.Status != domain.JobSucceeded || rec.FinishedAt == nil || len(rec.Events) != 3 {
		t.Fatalf("stored job wrong: %+v", rec)
	}
	if r.Active("alpha") != nil {
		t.Fatal("job should no longer be active")
	}
	if replay := collect(t, r, j.ID, 1); len(replay) != 2 {
		t.Fatalf("replay after seq 1 got %d events", len(replay))
	}
}

func TestJobFailureAndPanic(t *testing.T) {
	r, _ := newRunner(t)
	a, _ := r.Start("upgrade", "a", 0, func(context.Context, service.Events) (string, error) {
		return "", errors.New("boom")
	})
	b, _ := r.Start("upgrade", "b", 0, func(context.Context, service.Events) (string, error) {
		var m map[string]int
		m["x"] = 1
		return "", nil
	})
	r.Wait()
	for id, want := range map[string]string{a.ID: "boom", b.ID: "internal error"} {
		rec, err := r.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Status != domain.JobFailed || len(rec.Error) < len(want) || rec.Error[:len(want)] != want {
			t.Fatalf("job %s: %+v", id, rec)
		}
	}
	if _, err := r.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestEventCapAndInterrupted(t *testing.T) {
	r, st := newRunner(t)
	j, _ := r.Start("create", "big", 0, func(_ context.Context, ev service.Events) (string, error) {
		for i := 0; i < maxEvents+50; i++ {
			ev.Log(fmt.Sprint(i))
		}
		return "", nil
	})
	r.Wait()
	rec, _ := r.Get(j.ID)
	if len(rec.Events) != maxEvents || rec.Events[len(rec.Events)-1].Type != EventDone {
		t.Fatalf("got %d events", len(rec.Events))
	}

	stale := &domain.Job{ID: "stale", Kind: "start", Server: "x", Status: domain.JobRunning, CreatedAt: time.Now()}
	if err := st.InsertJob(stale); err != nil {
		t.Fatal(err)
	}
	if _, err := New(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetJob("stale")
	if got.Status != domain.JobFailed {
		t.Fatalf("stale job not failed: %+v", got)
	}
}
