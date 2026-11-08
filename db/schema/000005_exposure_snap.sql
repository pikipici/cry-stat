-- +goose Up
-- Crypto Station Fase 4: exposure snapshots + trial metrics (plan v0.5 bab 7.3, 8.3).

CREATE TABLE IF NOT EXISTS exposure_snapshots (
  id BIGSERIAL PRIMARY KEY,
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  net_position NUMERIC(36,18) NOT NULL,
  limit_used NUMERIC(10,6) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exposure_asset_time ON exposure_snapshots(asset, created_at DESC);

-- Slippage warn threshold: quote_age di atas ini (detik) masuk perhatian trial.
INSERT INTO settings (key, value) VALUES
  ('trial_quote_age_warn_seconds', '20')
ON CONFLICT (key) DO NOTHING;
