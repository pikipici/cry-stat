-- Admin wallet v0.14: deposit stok sandbox dan feed transaksi admin.

-- name: CreatePendingAdminStockDeposit :one
INSERT INTO admin_stock_deposits
  (admin_telegram_id, asset, network, units, to_address, tx_ref, status)
VALUES ($1,$2,$3,$4,$5,$6,'pending_confirm')
RETURNING id, admin_telegram_id, asset, network, units, to_address, tx_ref,
          status, journal_id, created_at, recorded_at;

-- name: GetAdminStockDepositForUpdate :one
SELECT id, admin_telegram_id, asset, network, units, to_address, tx_ref,
       status, journal_id, created_at, recorded_at
FROM admin_stock_deposits WHERE id=$1 FOR UPDATE;

-- name: GetAdminStockDepositByRef :one
SELECT id, admin_telegram_id, asset, network, units, to_address, tx_ref,
       status, journal_id, created_at, recorded_at
FROM admin_stock_deposits WHERE tx_ref=$1;

-- name: GetAdminStockDepositByID :one
SELECT id, admin_telegram_id, asset, network, units, to_address, tx_ref,
       status, journal_id, created_at, recorded_at
FROM admin_stock_deposits WHERE id=$1;

-- name: MarkAdminStockDepositRecorded :one
UPDATE admin_stock_deposits
SET status='recorded_mock', journal_id=$2, recorded_at=now()
WHERE id=$1 AND status='pending_confirm'
RETURNING id, admin_telegram_id, asset, network, units, to_address, tx_ref,
          status, journal_id, created_at, recorded_at;

-- name: MarkAdminStockDepositCancelled :one
UPDATE admin_stock_deposits
SET status='cancelled'
WHERE id=$1 AND status='pending_confirm'
RETURNING id, admin_telegram_id, asset, network, units, to_address, tx_ref,
          status, journal_id, created_at, recorded_at;

-- name: ListRecentAdminStockDeposits :many
SELECT id, admin_telegram_id, asset, network, units, to_address, tx_ref,
       status, journal_id, created_at, recorded_at
FROM admin_stock_deposits
ORDER BY created_at DESC, id DESC LIMIT $1;

-- Payout beli user untuk feed transaksi admin.
-- name: ListRecentPayoutsAdmin :many
SELECT id, user_id, asset, units, fee_units, sent_units, to_address, network,
       tx_hash, status, created_at
FROM payouts
ORDER BY created_at DESC, id DESC LIMIT $1;

-- Deposit jual user untuk feed transaksi admin.
-- name: ListRecentUserDepositsAdmin :many
SELECT id, user_id, asset, from_address, tx_hash, units, confirms,
       required_confirms, status, credited_idr, created_at
FROM deposits
ORDER BY created_at DESC, id DESC LIMIT $1;
