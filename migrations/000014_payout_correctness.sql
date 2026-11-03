-- +goose Up
-- Crypto Station Task 2 (CORRECTNESS_FIX_PLAN CF-01/CF-02/CF-03):
-- enforce payout refund identity + durable claim foundation.
-- Forward/additive. Tidak hapus data. Kompatibel goose parser repo
-- (tanpa dollar-quote, StatementBegin/End hanya untuk function body).

-- 1. Unique refund journal identity: satu payout = maksimal satu refund journal.
-- Partial unique: hanya untuk type='refund' + ref_table='payouts'.
-- Mencegah double-jurnal walau application-level guard jebol (race/crash).
CREATE UNIQUE INDEX IF NOT EXISTS idx_journals_refund_payout_unique
  ON journals(ref_id)
  WHERE type = 'refund' AND ref_table = 'payouts';

-- 2. Claim metadata minimal untuk durable worker claim (CF-03 foundation).
-- claimed_at: kapan worker memenangkan claim; claim_token: pemenang.
-- Dipakai Task 4 atomic claim (UPDATE ... RETURNING). Nullable = belum diklaim.
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS claim_token TEXT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ;

-- Index untuk recovery scan: queued/stale broadcasting yang belum diklaim.
CREATE INDEX IF NOT EXISTS idx_payouts_claim_recovery
  ON payouts(status, claimed_at, id)
  WHERE status IN ('queued','broadcasting');

-- +goose Down
DROP INDEX IF EXISTS idx_payouts_claim_recovery;
ALTER TABLE payouts DROP COLUMN IF EXISTS claimed_at;
ALTER TABLE payouts DROP COLUMN IF EXISTS claim_token;
DROP INDEX IF EXISTS idx_journals_refund_payout_unique;
