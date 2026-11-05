-- +goose Up
CREATE TABLE skill_packages (
 id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL CHECK (btrim(name) <> ''), version TEXT NOT NULL CHECK (btrim(version) <> ''),
 artifact_sha256 TEXT NOT NULL UNIQUE CHECK (artifact_sha256 ~ '^[0-9a-f]{64}$'),
 strategy_contract_version TEXT NOT NULL CHECK (btrim(strategy_contract_version) <> ''),
 research_only BOOLEAN NOT NULL CHECK (research_only), executes_transactions BOOLEAN NOT NULL CHECK (NOT executes_transactions),
 is_buy_signal BOOLEAN NOT NULL CHECK (NOT is_buy_signal), created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE (id, artifact_sha256)
);
CREATE TABLE research_runs (
 id BIGSERIAL PRIMARY KEY, skill_package_id BIGINT NOT NULL REFERENCES skill_packages(id),
 skill_artifact_sha256 TEXT NOT NULL CHECK (skill_artifact_sha256 ~ '^[0-9a-f]{64}$'),
 config_sha256 TEXT NOT NULL CHECK (config_sha256 ~ '^[0-9a-f]{64}$'),
 status TEXT NOT NULL CHECK (status IN ('queued','claimed','running','completed','completed_with_gaps','partial','failed')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY (skill_package_id, skill_artifact_sha256) REFERENCES skill_packages(id, artifact_sha256)
);
CREATE INDEX idx_research_runs_skill ON research_runs(skill_package_id);
CREATE INDEX idx_research_runs_status_created ON research_runs(status, created_at, id);
CREATE TABLE market_snapshots (
 id BIGSERIAL PRIMARY KEY, research_run_id BIGINT NOT NULL REFERENCES research_runs(id), venue TEXT NOT NULL CHECK (btrim(venue) <> ''),
 asset_id TEXT NOT NULL CHECK (btrim(asset_id) <> ''), observed_at TIMESTAMPTZ NOT NULL,
 payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'), payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(id, research_run_id)
);
CREATE INDEX idx_market_snapshots_run ON market_snapshots(research_run_id, id);
CREATE TABLE evidence_items (
 id BIGSERIAL PRIMARY KEY, research_run_id BIGINT NOT NULL REFERENCES research_runs(id), market_snapshot_id BIGINT NOT NULL,
 kind TEXT NOT NULL CHECK (btrim(kind) <> ''), observed_at TIMESTAMPTZ NOT NULL,
 payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'), payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY (market_snapshot_id, research_run_id) REFERENCES market_snapshots(id, research_run_id)
);
CREATE INDEX idx_evidence_run ON evidence_items(research_run_id, id);
CREATE INDEX idx_evidence_snapshot ON evidence_items(market_snapshot_id, id);
CREATE TABLE gate_results (
 id BIGSERIAL PRIMARY KEY, research_run_id BIGINT NOT NULL REFERENCES research_runs(id), gate TEXT NOT NULL CHECK (btrim(gate) <> ''),
 status TEXT NOT NULL CHECK (status IN ('PASS','FAIL','UNKNOWN','BLOCKED')), created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(research_run_id, gate)
);
CREATE TABLE audit_events (
 id BIGSERIAL PRIMARY KEY, action TEXT NOT NULL CHECK (btrim(action) <> ''), target_type TEXT NOT NULL CHECK (btrim(target_type) <> ''), target_id BIGINT NOT NULL,
 payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'), payload JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementBegin
CREATE FUNCTION reject_research_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable research record' USING ERRCODE = '55000'; END
$$;
-- +goose StatementEnd
CREATE TRIGGER skill_packages_immutable BEFORE UPDATE OR DELETE ON skill_packages FOR EACH ROW EXECUTE FUNCTION reject_research_immutable_change();
CREATE TRIGGER market_snapshots_immutable BEFORE UPDATE OR DELETE ON market_snapshots FOR EACH ROW EXECUTE FUNCTION reject_research_immutable_change();
CREATE TRIGGER evidence_items_immutable BEFORE UPDATE OR DELETE ON evidence_items FOR EACH ROW EXECUTE FUNCTION reject_research_immutable_change();
CREATE TRIGGER gate_results_immutable BEFORE UPDATE OR DELETE ON gate_results FOR EACH ROW EXECUTE FUNCTION reject_research_immutable_change();
CREATE TRIGGER audit_events_immutable BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_research_immutable_change();
-- +goose StatementBegin
CREATE FUNCTION protect_research_run_provenance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.skill_package_id <> OLD.skill_package_id OR NEW.skill_artifact_sha256 <> OLD.skill_artifact_sha256 OR NEW.config_sha256 <> OLD.config_sha256 OR NEW.created_at <> OLD.created_at THEN
  RAISE EXCEPTION 'immutable research run provenance' USING ERRCODE = '55000';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER research_runs_provenance_immutable BEFORE UPDATE ON research_runs FOR EACH ROW EXECUTE FUNCTION protect_research_run_provenance();
CREATE TRIGGER research_runs_no_delete BEFORE DELETE ON research_runs FOR EACH ROW EXECUTE FUNCTION reject_research_immutable_change();

-- +goose Down
DROP TABLE IF EXISTS audit_events, gate_results, evidence_items, market_snapshots, research_runs, skill_packages CASCADE;
DROP FUNCTION IF EXISTS protect_research_run_provenance();
DROP FUNCTION IF EXISTS reject_research_immutable_change();
