-- +goose Up
-- Crypto Station Fase 1: ledger double-entry + price snapshots (plan v0.5 bab 8.3, subset Fase 1).
-- Stage 1 sandbox: akun house = house:cash, house:inventory, house:withdraw-hold, house:provider-clearing.
-- Tidak ada house:revenue untuk kas spread di MVP (spread = P&L dihitung, bukan kaki jurnal).

CREATE TABLE IF NOT EXISTS ledger_accounts (
  id BIGSERIAL PRIMARY KEY,
  owner TEXT NOT NULL,
  asset TEXT NOT NULL CHECK (asset IN ('IDR','BNB','ETH','SOL')),
  UNIQUE (owner, asset)
);

CREATE TABLE IF NOT EXISTS journals (
  id BIGSERIAL PRIMARY KEY,
  type TEXT NOT NULL CHECK (type IN ('grant','topup','buy','sell','withdraw','refund','adjust')),
  ref_table TEXT,
  ref_id BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ledger_entries (
  id BIGSERIAL PRIMARY KEY,
  journal_id BIGINT NOT NULL REFERENCES journals(id),
  account_id BIGINT NOT NULL REFERENCES ledger_accounts(id),
  asset TEXT NOT NULL CHECK (asset IN ('IDR','BNB','ETH','SOL')),
  amount NUMERIC(36,18) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_journal ON ledger_entries(journal_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_account ON ledger_entries(account_id, created_at);

-- Cache turunan dari ledger (bukan sumber kebenaran). IDR disimpan sebagai NUMERIC juga
-- supaya satu tipe dengan coin; nilai IDR selalu bilangan bulat (ditegakkan di service).
CREATE TABLE IF NOT EXISTS wallets (
  user_id BIGINT NOT NULL REFERENCES users(id),
  asset TEXT NOT NULL CHECK (asset IN ('IDR','BNB','ETH','SOL')),
  balance NUMERIC(36,18) NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, asset)
);

CREATE TABLE IF NOT EXISTS price_snapshots (
  id BIGSERIAL PRIMARY KEY,
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  binance_price NUMERIC(36,18) NOT NULL,
  spread_buy NUMERIC(10,6) NOT NULL,
  spread_sell NUMERIC(10,6) NOT NULL,
  ask NUMERIC(36,18) NOT NULL,
  bid NUMERIC(36,18) NOT NULL,
  rate_usdt_idr NUMERIC(36,18) NOT NULL,
  rate_source TEXT NOT NULL,
  rate_age_ms BIGINT NOT NULL DEFAULT 0,
  price_age_ms BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_price_snapshots_asset_time ON price_snapshots(asset, created_at DESC);
