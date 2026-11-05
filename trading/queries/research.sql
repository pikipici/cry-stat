-- name: CreateResearchRun :one
INSERT INTO research_runs (skill_package_id, skill_artifact_sha256, config_sha256, status, created_at) VALUES ($1,$2,$3,$4,$5) RETURNING *;
-- name: CreateMarketSnapshot :one
INSERT INTO market_snapshots (research_run_id, venue, asset_id, observed_at, payload_sha256, payload) VALUES ($1,$2,$3,$4,$5,$6) RETURNING *;
-- name: CreateEvidenceItem :one
INSERT INTO evidence_items (research_run_id, market_snapshot_id, kind, observed_at, payload_sha256, payload) VALUES ($1,$2,$3,$4,$5,$6) RETURNING *;
-- name: CreateGateResult :one
INSERT INTO gate_results (research_run_id, gate, status, created_at) VALUES ($1,$2,$3,$4) RETURNING *;
-- name: CreateAuditEvent :one
INSERT INTO audit_events (action, target_type, target_id, payload_sha256, payload, created_at) VALUES ($1,$2,$3,$4,$5,$6) RETURNING *;
