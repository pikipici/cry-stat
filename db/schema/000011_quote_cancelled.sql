-- +goose Up
-- Crypto Station Phase 2 (revisi V.1 A1): status quote 'cancelled' + index active.
-- Forward/additive: tidak mengubah baris existing, hanya melonggarkan CHECK.

ALTER TABLE quotes DROP CONSTRAINT IF EXISTS quotes_status_check;
ALTER TABLE quotes ADD CONSTRAINT quotes_status_check
  CHECK (status IN ('active','used','expired','cancelled'));

CREATE INDEX IF NOT EXISTS idx_quotes_user_active
  ON quotes(user_id) WHERE status = 'active';

