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
INSERT INTO payouts (user_id, trade_id, asset, units, fee_units, to_address, network, tx_hash, status, confirms)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, created_at;

-- name: GetPayoutByTrade :one
SELECT id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network, tx_hash, status, confirms, created_at
FROM payouts WHERE trade_id=$1;

-- name: RecentPayoutsByUser :many
SELECT id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network, tx_hash, status, confirms,
       mode, attempt_count, refund_journal_id, created_at
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

-- Phase 4 (revisi V.1 A2): payout lifecycle durable.

-- name: CreatePayoutQueued :one
INSERT INTO payouts (user_id, trade_id, asset, units, fee_units, to_address, network, mode, status, confirms)
VALUES ($1,$2,$3,$4,$5,$6,$7,'sandbox','queued',0)
ON CONFLICT (trade_id) DO NOTHING
RETURNING id;

-- name: GetPayout :one
SELECT id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network, tx_hash, status, confirms,
       mode, attempt_count, last_error, refund_journal_id, broadcast_at, finalized_at, created_at
FROM payouts WHERE id=$1;

-- name: ClaimQueuedPayouts :many
SELECT id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network, tx_hash, status, confirms,
       mode, attempt_count, last_error, refund_journal_id, broadcast_at, finalized_at, created_at
FROM payouts WHERE status IN ('queued','broadcasting')
ORDER BY id LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: LockPayoutForRefund :one
SELECT id, user_id, trade_id, asset, status, refund_journal_id
FROM payouts WHERE id=$1
FOR UPDATE;

-- name: ClaimQueuedPayoutAtomic :one
UPDATE payouts SET status='broadcasting', attempt_count=attempt_count+1,
       broadcast_at=now(), claim_token=$2, claimed_at=now()
WHERE id=$1 AND status='queued'
RETURNING id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network,
       tx_hash, status, confirms, mode, attempt_count, last_error, refund_journal_id,
       broadcast_at, finalized_at, created_at;

-- name: ReclaimStaleBroadcastingAtomic :one
UPDATE payouts SET attempt_count=attempt_count+1,
       broadcast_at=now(), claim_token=$2, claimed_at=now()
WHERE id=$1 AND status='broadcasting' AND (claimed_at IS NULL OR claimed_at < $3)
RETURNING id, user_id, trade_id, asset, units, fee_units, sent_units, to_address, network,
       tx_hash, status, confirms, mode, attempt_count, last_error, refund_journal_id,
       broadcast_at, finalized_at, created_at;

-- name: ListReclaimablePayoutIDs :many
SELECT id FROM payouts
WHERE status='queued'
   OR (status='broadcasting' AND (claimed_at IS NULL OR claimed_at < $1))
ORDER BY id LIMIT $2;

-- name: MarkPayoutBroadcasting :execrows
UPDATE payouts SET status='broadcasting', attempt_count=attempt_count+1, broadcast_at=now()
WHERE id=$1 AND status IN ('queued','broadcasting','unknown');

-- name: MarkPayoutConfirmed :execrows
UPDATE payouts SET status='confirmed', tx_hash=$2, confirms=$3, finalized_at=now()
WHERE id=$1 AND status IN ('queued','broadcasting','unknown');

-- name: MarkPayoutFailed :execrows
UPDATE payouts SET status='failed', last_error=$2, finalized_at=now()
WHERE id=$1 AND status IN ('queued','broadcasting','unknown') AND refund_journal_id IS NULL;

-- name: MarkPayoutUnknown :execrows
UPDATE payouts SET status='unknown', last_error=$2
WHERE id=$1 AND status IN ('queued','broadcasting');

-- name: SetPayoutRefund :execrows
UPDATE payouts SET refund_journal_id=$2 WHERE id=$1 AND refund_journal_id IS NULL;
