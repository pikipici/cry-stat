-- Admin audit: setiap aksi admin catat before/after (bab 9 plan).

-- name: InsertAdminAudit :one
INSERT INTO admin_audit (admin_telegram_id, action, target_id, before, after)
VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at;

-- name: RecentAdminAudit :many
SELECT id, admin_telegram_id, action, target_id, before, after, created_at
FROM admin_audit ORDER BY id DESC LIMIT $1;

-- Settings tulis (admin tanpa restart).

-- name: UpsertSetting :one
INSERT INTO settings (key, value, updated_by) VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING key, value;
