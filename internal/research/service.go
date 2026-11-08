package research

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }
func scanJob(row interface{ Scan(...any) error }) (Job, error) {
	var j Job
	var status string
	var token *string
	err := row.Scan(&j.ID, &j.SkillPackageID, &j.SkillArtifactSHA256, &j.ConfigSHA256, &status, &j.MaxCandidates, &j.MaxCalls, &j.MaxSeconds, &j.CandidateCount, &j.ProcessedCount, &j.RequestCalls, &token)
	j.Status = Status(status)
	if token != nil {
		j.ClaimToken = *token
	}
	return j, err
}

const returningJob = ` RETURNING id,skill_package_id,skill_artifact_sha256,config_sha256,status,max_candidates,max_calls,max_seconds,candidate_count,processed_count,request_calls,claim_token`

func (s *Service) Queue(ctx context.Context, r QueueRequest) (Job, error) {
	if err := ValidateBounds(r); err != nil {
		return Job{}, err
	}
	return scanJob(s.pool.QueryRow(ctx, `INSERT INTO research_jobs(skill_package_id,skill_artifact_sha256,config_sha256,status,max_candidates,max_calls,max_seconds) VALUES($1,$2,$3,'queued',$4,$5,$6)`+returningJob, r.SkillPackageID, r.SkillArtifactSHA256, r.ConfigSHA256, r.MaxCandidates, r.MaxCalls, r.MaxSeconds))
}
func (s *Service) Claim(ctx context.Context, id int64, token string, lease time.Duration) (Job, error) {
	if token == "" || lease <= 0 {
		return Job{}, ErrNotClaimable
	}
	j, err := scanJob(s.pool.QueryRow(ctx, `UPDATE research_jobs SET status='claimed',claim_token=$2,claim_generation=claim_generation+1,claimed_at=now(),lease_expires_at=now()+($3*interval '1 millisecond'),updated_at=now() WHERE id=$1 AND (status='queued' OR (status IN ('claimed','running') AND lease_expires_at<now()))`+returningJob, id, token, lease.Milliseconds()))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotClaimable
	}
	if err != nil {
		return Job{}, fmt.Errorf("claim research job: %w", err)
	}
	return j, nil
}
func (s *Service) Start(ctx context.Context, id int64, token string) (Job, error) {
	j, err := scanJob(s.pool.QueryRow(ctx, `UPDATE research_jobs SET status='running',updated_at=now() WHERE id=$1 AND claim_token=$2 AND status='claimed' AND lease_expires_at>now()`+returningJob, id, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrStaleClaim
	}
	if err != nil {
		return Job{}, fmt.Errorf("start research job: %w", err)
	}
	return j, nil
}
func (s *Service) Resume(ctx context.Context, id int64, token string, lease time.Duration) (Job, error) {
	var status string
	if err := s.pool.QueryRow(ctx, `SELECT status FROM research_jobs WHERE id=$1`, id).Scan(&status); err != nil {
		return Job{}, fmt.Errorf("read research job for resume: %w", err)
	}
	if IsTerminal(Status(status)) {
		return Job{}, ErrTerminalJob
	}
	return s.Claim(ctx, id, token, lease)
}

func (s *Service) Renew(ctx context.Context, id int64, token string, lease time.Duration) error {
	tag, err := s.pool.Exec(ctx, `UPDATE research_jobs SET lease_expires_at=now()+($3*interval '1 millisecond'),updated_at=now() WHERE id=$1 AND claim_token=$2 AND status='running' AND lease_expires_at>now()`, id, token, lease.Milliseconds())
	if err != nil {
		return fmt.Errorf("renew research job: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleClaim
	}
	return nil
}
func (s *Service) Finalize(ctx context.Context, id int64, token string, status Status, candidates, processed, calls int, failure string) (Job, error) {
	if !IsTerminal(status) || candidates < 0 || processed < 0 || processed > candidates || calls < 0 {
		return Job{}, errors.New("invalid final state")
	}
	j, err := scanJob(s.pool.QueryRow(ctx, `UPDATE research_jobs SET status=$3,candidate_count=$4,processed_count=$5,request_calls=$6,failure_reason=$7,updated_at=now() WHERE id=$1 AND claim_token=$2 AND status='running' AND lease_expires_at>now() AND $4<=max_candidates AND $6<=max_calls`+returningJob, id, token, string(status), candidates, processed, calls, failure))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrStaleClaim
	}
	if err != nil {
		return Job{}, fmt.Errorf("finalize research job: %w", err)
	}
	return j, nil
}
