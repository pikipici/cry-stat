-- +goose Up
-- Crypto Station Fase 2: quotes mengikat + trades + risk events (plan v0.5 bab 8.3).

CREATE TABLE IF NOT EXISTS quotes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  side TEXT NOT NULL CHECK (side IN ('buy','sell')),
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  amount_idr BIGINT NOT NULL CHECK (amount_idr > 0),
  quoted_units NUMERIC(36,18) NOT NULL CHECK (quoted_units > 0),
  bid NUMERIC(36,18) NOT NULL,
  ask NUMERIC(36,18) NOT NULL,
  mid NUMERIC(36,18) NOT NULL,
  spread_used NUMERIC(10,6) NOT NULL,
  rate_usdt_idr NUMERIC(36,18) NOT NULL,
  rate_source TEXT NOT NULL,
  rate_age_ms BIGINT NOT NULL DEFAULT 0,
  price_age_ms BIGINT NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','used','expired')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_quotes_user_status ON quotes(user_id, status);

CREATE TABLE IF NOT EXISTS trades (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  quote_id BIGINT UNIQUE NOT NULL REFERENCES quotes(id),
  side TEXT NOT NULL CHECK (side IN ('buy','sell')),
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  amount_idr BIGINT NOT NULL,
  units NUMERIC(36,18) NOT NULL,
  price_used NUMERIC(36,18) NOT NULL,
  mid_used NUMERIC(36,18) NOT NULL,
  spread_pnl_idr NUMERIC(36,18) NOT NULL,
  journal_id BIGINT NOT NULL REFERENCES journals(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_trades_user_time ON trades(user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_trades_asset_side ON trades(asset, side);

-- Event anti-abuse: Stage 1 nulis, Stage 2 menghukum (bab 9 plan).
CREATE TABLE IF NOT EXISTS risk_events (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT REFERENCES users(id),
  telegram_id BIGINT NOT NULL,
  kind TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_risk_events_user ON risk_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_events_kind ON risk_events(kind, created_at DESC);

-- Batas eksposur sandbox (coin units, bukan IDR). TIDAK dibawa ke Stage 2 (bab 7.3 plan).
INSERT INTO settings (key, value) VALUES
  ('exposure_limit_bnb', '5'),
  ('exposure_limit_eth', '50'),
  ('exposure_limit_sol', '500')
ON CONFLICT (key) DO NOTHING;
