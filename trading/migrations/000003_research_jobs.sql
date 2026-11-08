-- +goose Up
CREATE TABLE research_jobs (
 id BIGSERIAL PRIMARY KEY,
 skill_package_id BIGINT NOT NULL,
 skill_artifact_sha256 TEXT NOT NULL CHECK (skill_artifact_sha256 ~ '^[0-9a-f]{64}$'),
 config_sha256 TEXT NOT NULL CHECK (config_sha256 ~ '^[0-9a-f]{64}$'),
 status TEXT NOT NULL CHECK (status IN ('queued','claimed','running','completed','completed_with_gaps','partial','failed')),
 max_candidates INTEGER NOT NULL CHECK (max_candidates BETWEEN 1 AND 10),
 max_calls INTEGER NOT NULL CHECK (max_calls BETWEEN 1 AND 1000),
 max_seconds INTEGER NOT NULL CHECK (max_seconds BETWEEN 1 AND 3600),
 candidate_count INTEGER NOT NULL DEFAULT 0,
 processed_count INTEGER NOT NULL DEFAULT 0,
 request_calls INTEGER NOT NULL DEFAULT 0,
 claim_token TEXT, claim_generation BIGINT NOT NULL DEFAULT 0,
 claimed_at TIMESTAMPTZ, lease_expires_at TIMESTAMPTZ,
 failure_reason TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY (skill_package_id, skill_artifact_sha256) REFERENCES skill_packages(id, artifact_sha256),
 CHECK (candidate_count BETWEEN 0 AND max_candidates),
 CHECK (processed_count BETWEEN 0 AND candidate_count),
 CHECK (request_calls BETWEEN 0 AND max_calls),
 CHECK ((status='queued' AND claim_token IS NULL AND claimed_at IS NULL AND lease_expires_at IS NULL) OR
        (status IN ('claimed','running') AND claim_token IS NOT NULL AND claimed_at IS NOT NULL AND lease_expires_at IS NOT NULL) OR
        (status IN ('completed','completed_with_gaps','partial','failed') AND claim_token IS NOT NULL AND claimed_at IS NOT NULL AND lease_expires_at IS NOT NULL))
);
CREATE INDEX idx_research_jobs_claim ON research_jobs(status, lease_expires_at, id);
-- +goose StatementBegin
CREATE FUNCTION protect_research_job() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.skill_package_id<>OLD.skill_package_id OR NEW.skill_artifact_sha256<>OLD.skill_artifact_sha256 OR NEW.config_sha256<>OLD.config_sha256 OR NEW.max_candidates<>OLD.max_candidates OR NEW.max_calls<>OLD.max_calls OR NEW.max_seconds<>OLD.max_seconds OR NEW.created_at<>OLD.created_at THEN
  RAISE EXCEPTION 'immutable research job config' USING ERRCODE='55000';
 END IF;
 IF NOT ((OLD.status='queued' AND NEW.status='claimed') OR (OLD.status='claimed' AND NEW.status='running') OR (OLD.status='running' AND NEW.status='running' AND NEW.claim_token=OLD.claim_token AND NEW.lease_expires_at>OLD.lease_expires_at) OR (OLD.status='running' AND NEW.status IN ('completed','completed_with_gaps','partial','failed')) OR (OLD.status IN ('claimed','running') AND OLD.lease_expires_at<now() AND NEW.status='claimed')) THEN
  RAISE EXCEPTION 'illegal research job transition' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER research_jobs_guard BEFORE UPDATE ON research_jobs FOR EACH ROW EXECUTE FUNCTION protect_research_job();

-- +goose Down
DROP TABLE IF EXISTS research_jobs;
DROP FUNCTION IF EXISTS protect_research_job();
