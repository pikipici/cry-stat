-- Quotes: konsumsi atomik adalah jantung Fase 2.
-- UPDATE ... RETURNING: 0 baris = sudah dipakai/expired/bukan milik user → batalkan tx.

-- name: CreateQuote :one
INSERT INTO quotes
  (user_id, side, asset, amount_idr, quoted_units, bid, ask, mid, spread_used,
   rate_usdt_idr, rate_source, rate_age_ms, price_age_ms, expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
RETURNING id, expires_at, created_at;

-- name: ConsumeQuote :one
UPDATE quotes SET status = 'used'
WHERE id = $1 AND user_id = $2 AND status = 'active' AND expires_at > now()
RETURNING id, user_id, side, asset, amount_idr, quoted_units, bid, ask, mid,
          spread_used, rate_usdt_idr, rate_source, expires_at;

-- name: GetQuote :one
SELECT id, user_id, side, asset, amount_idr, quoted_units, bid, ask, mid,
       spread_used, rate_usdt_idr, rate_source, rate_age_ms, price_age_ms,
       expires_at, status, created_at
FROM quotes WHERE id = $1;

-- name: ExpireUserQuotes :exec
UPDATE quotes SET status = 'expired'
WHERE user_id = $1 AND status = 'active' AND expires_at <= now();

-- name: ActiveQuoteForUser :one
SELECT id, user_id, side, asset, amount_idr, quoted_units, bid, ask, mid,
       spread_used, rate_usdt_idr, rate_source, rate_age_ms, price_age_ms,
       expires_at, status, created_at
FROM quotes WHERE user_id = $1 AND status = 'active' AND expires_at > now()
ORDER BY id DESC LIMIT 1;

-- Trades.

-- name: CreateTrade :one
INSERT INTO trades
  (user_id, quote_id, side, asset, amount_idr, units, price_used, mid_used, spread_pnl_idr, journal_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, created_at;

-- name: RecentTradesByUser :many
SELECT id, side, asset, amount_idr, units, price_used, journal_id, created_at
FROM trades WHERE user_id = $1 ORDER BY id DESC LIMIT $2;

-- Eksposur: net position per coin = terjual ke user − terbeli dari user (dalam units).
-- Beli user menambah posisi house negatif (house kasih coin), jual user mengurangi.
-- name: NetPositionByAsset :many
SELECT asset,
  SUM(CASE WHEN side = 'sell' THEN units ELSE -units END)::NUMERIC(36,18) AS net
FROM trades GROUP BY asset;

-- Risk events.

-- name: InsertRiskEvent :one
INSERT INTO risk_events (user_id, telegram_id, kind, payload)
VALUES ($1, $2, $3, $4) RETURNING id, created_at;
