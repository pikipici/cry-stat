package research

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Budget struct {
	MaxCandidates, MaxCalls int
	Deadline                time.Time
}

func (b Budget) Check(candidates, calls int) error {
	if candidates > b.MaxCandidates || calls > b.MaxCalls {
		return fmt.Errorf("research budget exceeded")
	}
	if time.Now().After(b.Deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

type Runner interface {
	Run(context.Context, Job, Budget) (Status, int, int, int, error)
}
type Worker struct {
	Service *Service
	Runner  Runner
	Lease   time.Duration
}

func (w Worker) Process(ctx context.Context, id int64, token string) (Job, error) {
	claimed, err := w.Service.Claim(ctx, id, token, w.Lease)
	if err != nil {
		return Job{}, err
	}
	job, err := w.Service.Start(ctx, id, token)
	if err != nil {
		return Job{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(job.MaxSeconds)*time.Second)
	defer cancel()
	budget := Budget{MaxCandidates: job.MaxCandidates, MaxCalls: job.MaxCalls, Deadline: time.Now().Add(time.Duration(job.MaxSeconds) * time.Second)}
	renewEvery := w.Lease / 3
	if renewEvery <= 0 {
		renewEvery = time.Millisecond
	}
	renewCtx, stopRenew := context.WithCancel(runCtx)
	renewErr := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(renewEvery)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := w.Service.Renew(renewCtx, id, token, w.Lease); err != nil {
					select {
					case renewErr <- err:
					default:
					}
					cancel()
					return
				}
			}
		}
	}()
	status, candidates, processed, calls, runErr := w.Runner.Run(runCtx, job, budget)
	stopRenew()
	select {
	case err := <-renewErr:
		return Job{}, fmt.Errorf("research lease renewal failed: %w", err)
	default:
	}
	failure := ""
	if runErr != nil {
		status = StatusFailed
		failure = runErr.Error()
	}
	if err := budget.Check(candidates, calls); err != nil {
		status = StatusFailed
		failure = err.Error()
		candidates = min(candidates, job.MaxCandidates)
		processed = min(processed, candidates)
		calls = min(calls, job.MaxCalls)
	}
	final, finalErr := w.Service.Finalize(ctx, claimed.ID, token, status, candidates, processed, calls, failure)
	if finalErr != nil {
		return Job{}, finalErr
	}
	if runErr != nil {
		return final, fmt.Errorf("research runner failed: %w", runErr)
	}
	if failure != "" {
		return final, errors.New(failure)
	}
	return final, nil
}
