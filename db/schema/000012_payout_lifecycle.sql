-- +goose Up
-- Crypto Station Phase 4 (revisi V.1 A2): kolom lifecycle payout durable.
-- Forward/additive. Mode sandbox vs real dicatat per row (fail-closed G2).

ALTER TABLE payouts ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'sandbox'
  CHECK (mode IN ('sandbox','real'));
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0
  CHECK (attempt_count >= 0);
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS refund_journal_id BIGINT UNIQUE REFERENCES journals(id);
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS broadcast_at TIMESTAMPTZ;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS finalized_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_payouts_retryable
  ON payouts(status, id) WHERE status IN ('queued','broadcasting');

