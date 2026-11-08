package research

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tradingdb "github.com/pikipici/crypto-station/internal/tradingdb"
)

// ClaimLister is the read side the poller needs: it returns IDs of jobs that
// are either freshly queued or whose lease has expired. Implemented by
// *tradingdb.Queries, but kept as an interface so the poller can be tested
// without a database.
type ClaimLister interface {
	ListClaimableResearchJobs(ctx context.Context, limit int32) ([]int64, error)
}

// Poller drains claimable research jobs by feeding them to a Worker. It is the
// missing consumer for trading/queries/jobs.sql: the query existed but nothing
// called it, so queued jobs sat forever.
type Poller struct {
	Jobs         ClaimLister
	Worker       Worker
	Batch        int32
	IdleInterval time.Duration
	TokenPrefix  string
	// OnJob is optional telemetry; it fires after every Process attempt with
	// the resulting job (zero value on error) and the error.
	OnJob func(context.Context, int64, Job, error)
}

// NewPoller builds a Poller backed by a live pool. batch and idle are clamped
// to sane defaults so a misconfiguration cannot spin the CPU or claim the whole
// queue at once.
func NewPoller(pool *pgxpool.Pool, worker Worker, batch int32, idle time.Duration) *Poller {
	if batch < 1 {
		batch = 1
	}
	if batch > 25 {
		batch = 25
	}
	if idle <= 0 {
		idle = 5 * time.Second
	}
	return &Poller{
		Jobs:         tradingdb.New(pool),
		Worker:       worker,
		Batch:        batch,
		IdleInterval: idle,
		TokenPrefix:  "poller",
	}
}

// Tick performs exactly one polling round: list claimable IDs, then process each
// one. It returns how many jobs were attempted and how many completed without
// error. Per-job failures are reported through OnJob (or collected in errs when
// OnJob is nil) and never abort the round, because one poisoned job must not
// starve the rest of the queue.
func (p *Poller) Tick(ctx context.Context) (attempted, succeeded int, errs []error) {
	return p.tickProcess(ctx, p.Worker.Process)
}

// tickProcess is Tick with the per-job processing step injected. Tests use it to
// exercise polling semantics without a live database or a real Worker.
func (p *Poller) tickProcess(ctx context.Context, process func(context.Context, int64, string) (Job, error)) (attempted, succeeded int, errs []error) {
	ids, err := p.Jobs.ListClaimableResearchJobs(ctx, p.batchSize())
	if err != nil {
		return 0, 0, []error{fmt.Errorf("list claimable research jobs: %w", err)}
	}
	if process == nil {
		return 0, 0, []error{errors.New("research job processor is nil")}
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			return attempted, succeeded, errs
		}
		token, tokenErr := p.newToken()
		if tokenErr != nil {
			errs = append(errs, tokenErr)
			continue
		}
		attempted++
		job, processErr := process(ctx, id, token)
		if p.OnJob != nil {
			p.OnJob(ctx, id, job, processErr)
		}
		if processErr != nil {
			if p.OnJob == nil {
				errs = append(errs, fmt.Errorf("process job %d: %w", id, processErr))
			}
			continue
		}
		succeeded++
	}
	return attempted, succeeded, errs
}

// Run polls until ctx is cancelled. When a round finds work it loops again
// immediately; when the queue is empty it waits IdleInterval between rounds.
// The returned error is always ctx.Err(), or nil if ctx ended cleanly.
func (p *Poller) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempted, _, _ := p.Tick(ctx)
		if attempted > 0 {
			continue
		}
		timer := time.NewTimer(p.idleInterval())
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (p *Poller) batchSize() int32 {
	if p.Batch < 1 {
		return 1
	}
	if p.Batch > 25 {
		return 25
	}
	return p.Batch
}

func (p *Poller) idleInterval() time.Duration {
	if p.IdleInterval <= 0 {
		return 5 * time.Second
	}
	return p.IdleInterval
}

// newToken builds a claim token that is unique per attempt: prefix + random hex.
// Token uniqueness matters because Service.Claim bumps claim_generation and the
// protect_research_job trigger rejects a renewal that reuses a stale token.
func (p *Poller) newToken() (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate claim token: %w", err)
	}
	prefix := p.TokenPrefix
	if prefix == "" {
		prefix = "poller"
	}
	return prefix + "-" + hex.EncodeToString(buf[:]), nil
}

// ErrNoRunner is returned by FuncRunner when the function itself is nil.
var ErrNoRunner = errors.New("research runner function is nil")

// FuncRunner adapts a plain function to the Runner interface so callers can
// wire a runner without declaring a type.
type FuncRunner func(context.Context, Job, Budget) (Status, int, int, int, error)

func (f FuncRunner) Run(ctx context.Context, job Job, budget Budget) (Status, int, int, int, error) {
	if f == nil {
		return StatusFailed, 0, 0, 0, ErrNoRunner
	}
	return f(ctx, job, budget)
}
