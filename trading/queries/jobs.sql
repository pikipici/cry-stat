-- name: ListClaimableResearchJobs :many
SELECT id FROM research_jobs
WHERE status='queued' OR (status IN ('claimed','running') AND lease_expires_at < now())
ORDER BY id
LIMIT $1;
