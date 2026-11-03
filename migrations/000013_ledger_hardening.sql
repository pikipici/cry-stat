-- +goose Up
-- Crypto Station Phase 9 (revisi V.1 A27): DB-level ledger hardening.
-- Forward/additive. Data workspace sudah kompatibel (0 user >1 active quote
-- pada 2026-11-03; sent_units historis konsisten units-fee by construction).

-- 1. Entry asset HARUS cocok dengan account asset (trigger, bukan app saja).
-- NOTE goose: badan fungsi single-quote (tanpa dollar-quote $$).
CREATE OR REPLACE FUNCTION check_entry_asset_match()
RETURNS TRIGGER AS '
BEGIN
  IF NEW.asset != (SELECT asset FROM ledger_accounts WHERE id = NEW.account_id) THEN
    RAISE EXCEPTION ''ledger asset mismatch'';
  END IF;
  RETURN NEW;
END;
' LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_entry_asset_match ON ledger_entries;
CREATE TRIGGER trg_entry_asset_match
  BEFORE INSERT OR UPDATE ON ledger_entries
  FOR EACH ROW EXECUTE FUNCTION check_entry_asset_match();

-- 2. Domain checks deposits/payouts.
ALTER TABLE deposits ADD CONSTRAINT deposits_units_positive CHECK (units > 0);
ALTER TABLE payouts ADD CONSTRAINT payouts_units_positive CHECK (units > 0);
ALTER TABLE payouts ADD CONSTRAINT payouts_fee_nonnegative CHECK (fee_units >= 0);

-- 3. sent_units SELALU = units - fee_units (by construction, bukan input).
-- Kolom existing konsisten (ditulis service dari rumus yang sama) sehingga
-- ALTER aman tanpa backfill.
ALTER TABLE payouts DROP COLUMN IF EXISTS sent_units;
ALTER TABLE payouts ADD COLUMN sent_units NUMERIC(36,18)
  GENERATED ALWAYS AS (units - fee_units) STORED;

-- 3. Satu active quote per user (partial unique — expired/used/cancelled bebas).
CREATE UNIQUE INDEX IF NOT EXISTS idx_quotes_user_single_active
  ON quotes(user_id) WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_quotes_user_single_active;
-- NOTE: GENERATED sent_units tidak di-down ke kolom biasa (data tetap valid;
-- rollback skema penuh hanya via restore drill). Constraint di bawah aman di-down.
ALTER TABLE payouts DROP CONSTRAINT IF EXISTS payouts_fee_nonnegative;
ALTER TABLE payouts DROP CONSTRAINT IF EXISTS payouts_units_positive;
ALTER TABLE deposits DROP CONSTRAINT IF EXISTS deposits_units_positive;
DROP TRIGGER IF EXISTS trg_entry_asset_match ON ledger_entries;
DROP FUNCTION IF EXISTS check_entry_asset_match();
