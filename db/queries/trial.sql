-- Exposure snapshots: posisi house berkala (untuk trial + alert).

-- name: InsertExposureSnapshot :one
INSERT INTO exposure_snapshots (asset, net_position, limit_used)
VALUES ($1, $2, $3) RETURNING id, created_at;

-- name: LatestExposure :one
SELECT id, asset, net_position, limit_used, created_at
FROM exposure_snapshots WHERE asset = $1 ORDER BY created_at DESC LIMIT 1;

-- Semua telegram_id user aktif (untuk /broadcast).
-- name: ListActiveTelegramIDs :many
SELECT telegram_id FROM users WHERE status = 'active' ORDER BY id;

-- Trial metrics: quote_age per trade (proxy slippage risk) + reject count.
-- name: TradeQuoteAges :many
SELECT t.id, t.side, t.asset, t.amount_idr,
       EXTRACT(EPOCH FROM (t.created_at - q.created_at))::BIGINT AS quote_age_seconds,
       q.price_age_ms, q.rate_age_ms
FROM trades t JOIN quotes q ON q.id = t.quote_id
ORDER BY t.id DESC LIMIT $1;

-- name: CountRiskEventsByKind :many
SELECT kind, COUNT(*)::BIGINT AS n FROM risk_events GROUP BY kind ORDER BY n DESC;

-- name: CountQuotesByStatus :many
SELECT status, COUNT(*)::BIGINT AS n FROM quotes GROUP BY status;

-- Slippage real per trade: (price_used - mid_used) / mid_used (signed).
-- FIX debug #8: data sudah ada di trades tapi tidak pernah dihitung.
-- name: TradeSlippage :many
SELECT id, side, asset,
  CASE WHEN mid_used = 0 THEN 0::NUMERIC(36,18)
       ELSE ((price_used - mid_used) / mid_used)::NUMERIC(36,18) END AS slip_pct
FROM trades ORDER BY id DESC LIMIT $1;
