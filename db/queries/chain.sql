-- Chain transit Fase 8 (plan v0.12): alamat + payout + deposit + float.

-- name: UpsertWalletAddress :one
INSERT INTO wallet_addresses (user_id, asset, address, network, status)
VALUES ($1,$2,$3,$4,'active')
ON CONFLICT (user_id, asset, status) DO UPDATE SET address=EXCLUDED.address, network=EXCLUDED.network, created_at=now()
RETURNING id, address, network, created_at;

-- name: GetWalletAddress :one
SELECT id, user_id, asset, address, network, status, created_at
FROM wallet_addresses WHERE user_id=$1 AND asset=$2 AND status='active';

-- name: ListWalletAddresses :many
SELECT id, user_id, asset, address, network, status, created_at
FROM wallet_addresses WHERE user_id=$1 AND status='active' ORDER BY asset;

-- name: CreatePayout :one
INSERT INTO payouts (user_id, trade_id, asset, units, fee_units, sent_units, to_address, network, tx_hash, status, confirms)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, created_at;

-- name: GetPayoutByTrade :one
SELECT id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network, tx_hash, status, confirms, created_at
FROM payouts WHERE trade_id=$1;

-- name: RecentPayoutsByUser :many
SELECT id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network, tx_hash, status, confirms, created_at
FROM payouts WHERE user_id=$1 ORDER BY id DESC LIMIT $2;

-- name: InsertChainEvent :one
INSERT INTO chain_events (asset, tx_hash, event_type, raw_payload, confirms)
VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at;

-- name: InsertFloatSnapshot :one
INSERT INTO float_snapshots (asset, float_balance, float_min)
VALUES ($1,$2,$3) RETURNING id, created_at;

-- name: GetDepositByTx :one
SELECT id, user_id, asset, from_address, tx_hash, units, confirms, required_confirms, status, credited_idr, created_at
FROM deposits WHERE tx_hash=$1;

-- name: RecentDepositsByUser :many
SELECT id, user_id, asset, from_address, tx_hash, units, confirms, required_confirms, status, credited_idr, created_at
FROM deposits WHERE user_id=$1 ORDER BY id DESC LIMIT $2;
