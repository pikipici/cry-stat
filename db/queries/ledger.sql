-- Ledger accounts: ambil (lock) atau buat.

-- name: GetLedgerAccountForUpdate :one
SELECT id, owner, asset FROM ledger_accounts
WHERE owner = $1 AND asset = $2 FOR UPDATE;

-- name: CreateLedgerAccount :one
INSERT INTO ledger_accounts (owner, asset) VALUES ($1, $2)
ON CONFLICT (owner, asset) DO UPDATE SET owner = EXCLUDED.owner
RETURNING id, owner, asset;

-- name: CreateJournal :one
INSERT INTO journals (type, ref_table, ref_id) VALUES ($1, $2, $3)
RETURNING id, type, created_at;

-- name: CreateLedgerEntry :one
INSERT INTO ledger_entries (journal_id, account_id, asset, amount)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- Journal balance check: total per aset harus 0 (dipakai test + reconcile).
-- name: JournalSums :many
SELECT asset, SUM(amount)::NUMERIC(36,18) AS total
FROM ledger_entries WHERE journal_id = $1 GROUP BY asset;

-- Wallets cache.

-- name: GetWallet :one
SELECT user_id, asset, balance FROM wallets WHERE user_id = $1 AND asset = $2;

-- Kunci row wallet sebelum cek saldo (FIX debug #3: cegah overspend dua quote paralel).
-- name: GetWalletForUpdate :one
SELECT user_id, asset, balance FROM wallets WHERE user_id = $1 AND asset = $2 FOR UPDATE;

-- name: UpsertWalletBalance :one
INSERT INTO wallets (user_id, asset, balance) VALUES ($1, $2, $3)
ON CONFLICT (user_id, asset) DO UPDATE SET balance = EXCLUDED.balance, updated_at = now()
RETURNING user_id, asset, balance;

-- Semua wallet user (untuk /saldo).
-- name: ListWalletsByUser :many
SELECT user_id, asset, balance FROM wallets WHERE user_id = $1 ORDER BY asset;

-- Total per aset dari jurnal (sumber kebenaran, untuk rekonsiliasi vs wallets).
-- name: SumLedgerByOwnerAsset :many
SELECT a.owner, e.asset, SUM(e.amount)::NUMERIC(36,18) AS total
FROM ledger_entries e JOIN ledger_accounts a ON a.id = e.account_id
GROUP BY a.owner, e.asset;

-- Total cache wallets per user+aset (untuk rekonsiliasi).
-- name: SumWalletsByUserAsset :many
SELECT user_id, asset, SUM(balance)::NUMERIC(36,18) AS total
FROM wallets GROUP BY user_id, asset;

-- Users: lock + reset limit harian (WIB dihitung di service).

-- name: GetUserForUpdate :one
SELECT id, telegram_id, mode, status, pin_hash,
       daily_topup_total, daily_withdraw_total, last_limit_reset, created_at
FROM users WHERE id = $1 FOR UPDATE;

-- name: UpdateUserLimits :exec
UPDATE users SET daily_topup_total = $2, daily_withdraw_total = $3, last_limit_reset = $4
WHERE id = $1;

-- Price snapshots: tulis tiap tick (ringan, 3 coin), baca terakhir per coin.

-- name: InsertPriceSnapshot :one
INSERT INTO price_snapshots
  (asset, binance_price, spread_buy, spread_sell, ask, bid, rate_usdt_idr, rate_source, rate_age_ms, price_age_ms)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, created_at;

-- name: LatestPriceSnapshot :one
SELECT id, asset, binance_price, spread_buy, spread_sell, ask, bid,
       rate_usdt_idr, rate_source, rate_age_ms, price_age_ms, created_at
FROM price_snapshots WHERE asset = $1 ORDER BY created_at DESC LIMIT 1;
