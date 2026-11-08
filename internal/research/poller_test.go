package research

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeLister struct {
	mu    sync.Mutex
	calls int
	ids   []int64
	err   error
}

func (f *fakeLister) ListClaimableResearchJobs(ctx context.Context, limit int32) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if int32(len(f.ids)) > limit {
		return f.ids[:limit], nil
	}
	return f.ids, nil
}

func (f *fakeLister) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestPollerTickProcessesEveryClaimableJob(t *testing.T) {
	lister := &fakeLister{ids: []int64{11, 22, 33}}
	var processed []int64
	p := &Poller{
		Jobs: lister,
		Worker: Worker{Runner: FuncRunner(func(ctx context.Context, job Job, b Budget) (Status, int, int, int, error) {
			processed = append(processed, job.ID)
			return StatusCompleted, 1, 1, 0, nil
		})},
		Batch:        10,
		IdleInterval: time.Millisecond,
	}
	// Wire the worker's Service call path to the fake worker instead of Postgres.
	p.Worker = Worker{}
	attempted, succeeded, errs := p.tickProcess(context.Background(), func(ctx context.Context, id int64, token string) (Job, error) {
		processed = append(processed, id)
		return Job{ID: id, Status: StatusCompleted}, nil
	})

	if attempted != 3 || succeeded != 3 {
		t.Fatalf("attempted=%d succeeded=%d, want 3/3", attempted, succeeded)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(processed) != 3 || processed[0] != 11 || processed[2] != 33 {
		t.Fatalf("processed=%v, want [11 22 33]", processed)
	}
	if lister.callCount() != 1 {
		t.Fatalf("list calls=%d, want 1", lister.callCount())
	}
}

func TestPollerTickIsolatesPerJobFailures(t *testing.T) {
	lister := &fakeLister{ids: []int64{1, 2, 3}}
	boom := errors.New("runner exploded")
	attempted, succeeded, errs := (&Poller{Jobs: lister, Batch: 5}).tickProcess(context.Background(), func(ctx context.Context, id int64, token string) (Job, error) {
		if id == 2 {
			return Job{}, boom
		}
		return Job{ID: id, Status: StatusCompleted}, nil
	})

	if attempted != 3 || succeeded != 2 {
		t.Fatalf("attempted=%d succeeded=%d, want 3/2", attempted, succeeded)
	}
	if len(errs) != 1 || !errors.Is(errs[0], boom) {
		t.Fatalf("errs=%v, want single wrapped boom", errs)
	}
}

func TestPollerTickSurfacesListError(t *testing.T) {
	lister := &fakeLister{err: errors.New("db down")}
	attempted, succeeded, errs := (&Poller{Jobs: lister}).tickProcess(context.Background(), func(ctx context.Context, id int64, token string) (Job, error) {
		t.Fatal("process must not be called when listing fails")
		return Job{}, nil
	})
	if attempted != 0 || succeeded != 0 {
		t.Fatalf("attempted=%d succeeded=%d, want 0/0", attempted, succeeded)
	}
	if len(errs) != 1 {
		t.Fatalf("errs=%v, want 1", errs)
	}
}

func TestPollerTokensAreUniquePerAttempt(t *testing.T) {
	p := &Poller{Jobs: &fakeLister{}, Batch: 1}
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		token, err := p.newToken()
		if err != nil {
			t.Fatal(err)
		}
		if token == "" {
			t.Fatal("empty token")
		}
		if seen[token] {
			t.Fatalf("duplicate token %q", token)
		}
		seen[token] = true
	}
}

func TestPollerClampsBatchAndIdle(t *testing.T) {
	p := &Poller{Batch: 0, IdleInterval: 0}
	if p.batchSize() != 1 {
		t.Fatalf("batchSize=%d, want 1", p.batchSize())
	}
	if p.idleInterval() != 5*time.Second {
		t.Fatalf("idleInterval=%v, want 5s", p.idleInterval())
	}
	p.Batch = 500
	if p.batchSize() != 25 {
		t.Fatalf("batchSize=%d, want 25", p.batchSize())
	}
}

func TestPollerRunStopsOnContextCancel(t *testing.T) {
	lister := &fakeLister{}
	p := &Poller{Jobs: lister, Batch: 1, IdleInterval: 2 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := p.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v, want DeadlineExceeded", err)
	}
	if lister.callCount() < 2 {
		t.Fatalf("list calls=%d, want repeated polling", lister.callCount())
	}
}

func TestFuncRunnerNilIsFailure(t *testing.T) {
	var r FuncRunner
	status, _, _, _, err := r.Run(context.Background(), Job{}, Budget{})
	if !errors.Is(err, ErrNoRunner) || status != StatusFailed {
		t.Fatalf("status=%v err=%v, want failed/ErrNoRunner", status, err)
	}
}
