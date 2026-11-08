package research

import "errors"

type Status string

const (
	StatusQueued            Status = "queued"
	StatusClaimed           Status = "claimed"
	StatusRunning           Status = "running"
	StatusCompleted         Status = "completed"
	StatusCompletedWithGaps Status = "completed_with_gaps"
	StatusPartial           Status = "partial"
	StatusFailed            Status = "failed"
)

var (
	ErrNotClaimable = errors.New("research job not claimable")
	ErrStaleClaim   = errors.New("research job stale claim")
	ErrTerminalJob  = errors.New("research job already terminal")
)

type QueueRequest struct {
	SkillPackageID                      int64
	SkillArtifactSHA256, ConfigSHA256   string
	MaxCandidates, MaxCalls, MaxSeconds int
}
type Job struct {
	ID                                           int64
	Status                                       Status
	SkillPackageID                               int64
	SkillArtifactSHA256, ConfigSHA256            string
	MaxCandidates, MaxCalls, MaxSeconds          int
	CandidateCount, ProcessedCount, RequestCalls int
	ClaimToken                                   string
}

func ValidateBounds(r QueueRequest) error {
	if r.SkillPackageID <= 0 || len(r.SkillArtifactSHA256) != 64 || len(r.ConfigSHA256) != 64 {
		return errors.New("invalid job provenance")
	}
	if r.MaxCandidates < 1 || r.MaxCandidates > 10 || r.MaxCalls < 1 || r.MaxCalls > 1000 || r.MaxSeconds < 1 || r.MaxSeconds > 3600 {
		return errors.New("invalid job bounds")
	}
	return nil
}
func IsTerminal(s Status) bool {
	return s == StatusCompleted || s == StatusCompletedWithGaps || s == StatusPartial || s == StatusFailed
}
