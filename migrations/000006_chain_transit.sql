-- +goose Up
-- Crypto Station Fase 8 (plan v0.12): transit + auto-kirim.
-- WalletAddresses (1 aktif per user per coin) + Payouts (txhash mock di sandbox)
-- + Deposits (Fase 9, tabel duluan biar skema utuh) + ChainEvents + FloatSnapshots.
-- Quotes +FeeUnits +ToAddress (quote 3 baris).

CREATE TABLE IF NOT EXISTS wallet_addresses (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  address TEXT NOT NULL,
  network TEXT NOT NULL CHECK (network IN ('BSC','ERC20','SOL')),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','replaced')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, asset, status)
);
CREATE INDEX IF NOT EXISTS idx_walletaddr_user ON wallet_addresses(user_id, asset);

CREATE TABLE IF NOT EXISTS payouts (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  trade_id BIGINT UNIQUE NOT NULL REFERENCES trades(id),
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  units NUMERIC(36,18) NOT NULL,
  fee_units NUMERIC(36,18) NOT NULL,
  sent_units NUMERIC(36,18) NOT NULL,
  to_address TEXT NOT NULL,
  network TEXT NOT NULL,
  tx_hash TEXT UNIQUE,
  status TEXT NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued','broadcasting','confirmed','failed','unknown')),
  confirms INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payouts_user_time ON payouts(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payouts_status ON payouts(status);

CREATE TABLE IF NOT EXISTS deposits (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT REFERENCES users(id),
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  from_address TEXT NOT NULL DEFAULT '',
  tx_hash TEXT UNIQUE NOT NULL,
  units NUMERIC(36,18) NOT NULL,
  confirms INT NOT NULL DEFAULT 0,
  required_confirms INT NOT NULL DEFAULT 15,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','credited','manual')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_deposits_status ON deposits(status);

CREATE TABLE IF NOT EXISTS chain_events (
  id BIGSERIAL PRIMARY KEY,
  asset TEXT NOT NULL,
  tx_hash TEXT NOT NULL,
  event_type TEXT NOT NULL,
  raw_payload TEXT NOT NULL DEFAULT '',
  confirms INT NOT NULL DEFAULT 0,
  processed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chainevents_tx ON chain_events(tx_hash);

CREATE TABLE IF NOT EXISTS float_snapshots (
  id BIGSERIAL PRIMARY KEY,
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  float_balance NUMERIC(36,18) NOT NULL,
  float_min NUMERIC(36,18) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_float_asset_time ON float_snapshots(asset, created_at DESC);

-- Quotes: kolom fee + alamat tujuan (quote 3 baris v0.12).
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS fee_units NUMERIC(36,18) NOT NULL DEFAULT 0;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS to_address TEXT NOT NULL DEFAULT '';

-- Seed Fase 8: fee mock per coin (units), float min mock, confirms, fee tolak.
INSERT INTO settings (key, value) VALUES
  ('fee_mock_bnb', '0.0005'),
  ('fee_mock_eth', '0.001'),
  ('fee_mock_sol', '0.00001'),
  ('float_min_bnb', '1'),
  ('float_min_eth', '10'),
  ('float_min_sol', '100'),
  ('deposit_confirms_bnb', '15'),
  ('deposit_confirms_eth', '12'),
  ('deposit_confirms_sol', '32'),
  ('fee_reject_pct', '0.2'),
  ('deposit_address_bnb', 'MOCK-DEPOSIT-BNB'),
  ('deposit_address_eth', 'MOCK-DEPOSIT-ETH'),
  ('deposit_address_sol', 'MOCK-DEPOSIT-SOL')
ON CONFLICT (key) DO NOTHING;
