-- name: GetUserByTelegramID :one
SELECT id, telegram_id, mode, status, pin_hash,
       daily_topup_total, daily_withdraw_total, last_limit_reset, created_at
FROM users WHERE telegram_id = $1;

-- name: CreateUser :one
INSERT INTO users (telegram_id, mode) VALUES ($1, $2)
RETURNING id, telegram_id, mode, status, created_at;

-- name: GetSetting :one
SELECT key, value FROM settings WHERE key = $1;

-- name: ListSettings :many
SELECT key, value FROM settings ORDER BY key;
