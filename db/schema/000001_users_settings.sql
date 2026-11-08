-- +goose Up
-- Crypto Station Fase 0: users + settings (bab 8 plan, subset)
-- Full skema ledger (journals/entries/wallets/dll) masuk Fase 1.

CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  telegram_id BIGINT UNIQUE NOT NULL,
  mode TEXT NOT NULL DEFAULT 'sandbox' CHECK (mode IN ('sandbox','real')),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','blocked')),
  pin_hash TEXT,
  daily_topup_total BIGINT NOT NULL DEFAULT 0,
  daily_withdraw_total BIGINT NOT NULL DEFAULT 0,
  last_limit_reset DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed default Fase 0 (angka dari plan v0.5)
INSERT INTO settings (key, value) VALUES
  ('spread_buy_bnb', '0.01'), ('spread_sell_bnb', '0.01'),
  ('spread_buy_eth', '0.01'), ('spread_sell_eth', '0.01'),
  ('spread_buy_sol', '0.01'), ('spread_sell_sol', '0.01'),
  ('min_topup_idr', '50000'), ('min_buy_idr', '25000'), ('min_withdraw_idr', '100000'),
  ('daily_topup_limit_idr', '5000000'), ('daily_withdraw_limit_idr', '5000000'),
  ('withdraw_approval_threshold_idr', '1000000'),
  ('quote_lock_seconds', '30'),
  ('staleness_coin_ms', '15000'), ('staleness_fx_ms', '60000'),
  ('sandbox_starting_balance_idr', '1000000')
ON CONFLICT (key) DO NOTHING;
