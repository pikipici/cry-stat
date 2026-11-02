-- +goose Up
-- Crypto Station v0.14: catatan deposit stok admin ke hot wallet (sandbox/mock).
-- Tidak menyimpan private key, seed, atau secret.

CREATE TABLE admin_stock_deposits (
  id BIGSERIAL PRIMARY KEY,
  admin_telegram_id BIGINT NOT NULL,
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  network TEXT NOT NULL CHECK (network IN ('BSC','ERC20','SOL')),
  units NUMERIC(36,18) NOT NULL CHECK (units > 0),
  to_address TEXT NOT NULL,
  tx_ref TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK (status IN ('pending_confirm','recorded_mock','cancelled','failed')),
  journal_id BIGINT UNIQUE REFERENCES journals(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  recorded_at TIMESTAMPTZ
);

CREATE INDEX idx_admin_stock_deposits_created
  ON admin_stock_deposits(created_at DESC, id DESC);
CREATE INDEX idx_admin_stock_deposits_asset_created
  ON admin_stock_deposits(asset, created_at DESC, id DESC);
