-- +goose Up
-- Crypto Station Fase 3: admin audit trail (plan v0.5 bab 8.3).
-- Settings pause_<coin> + admin_allowlist TIDAK di-seed (ditulis via /setlimit + env).

CREATE TABLE IF NOT EXISTS admin_audit (
  id BIGSERIAL PRIMARY KEY,
  admin_telegram_id BIGINT NOT NULL,
  action TEXT NOT NULL,
  target_id BIGINT,
  before TEXT NOT NULL DEFAULT '',
  after TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_admin_audit_action ON admin_audit(action, created_at DESC);

-- Rate limit default per perintah (per akun, per menit). Angka di Settings (tanpa restart).
INSERT INTO settings (key, value) VALUES
  ('rate_limit_per_min', '30'),
  ('admin_allowlist', '')
ON CONFLICT (key) DO NOTHING;
