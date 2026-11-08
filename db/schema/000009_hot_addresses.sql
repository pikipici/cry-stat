-- +goose Up
-- Crypto Station sistem stok v0.13: alamat hot per coin (publik, sandbox = mock).
-- Key ASLI tidak di DB — cuma env (HOTWALLET_KEY_EVM/SOL, baru Fase 10).

INSERT INTO settings (key, value) VALUES
  ('hot_address_bnb', 'MOCK-HOT-BNB'),
  ('hot_address_eth', 'MOCK-HOT-ETH'),
  ('hot_address_sol', 'MOCK-HOT-SOL')
ON CONFLICT (key) DO NOTHING;
