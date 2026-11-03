// Package chain: durable worker claim (Task 4 CORRECTNESS_FIX_PLAN, CF-03).
// Satu payout hanya dimenangkan satu worker pada satu attempt via atomic
// UPDATE ... RETURNING (bukan SELECT lock yang langsung lepas).
package chain

import (
	"context"
	"errors"
	"time"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrPayoutClaimLost: payout sudah diklaim worker lain / bukan reclaimable.
var ErrPayoutClaimLost = errors.New("payout claim lost: sudah diklaim worker lain")

// StaleBroadcastingAge: broadcasting dengan claimed_at lebih tua dari ini
// boleh direclaim worker/recovery. Fresh broadcasting = milik worker aktif.
const StaleBroadcastingAge = 5 * time.Minute

// ClaimPayout: menangkan satu payout queued secara atomik. Token unik per worker-attempt
// (misal workerID + payoutID + attempt). Bukan queued / sudah diklaim → ErrPayoutClaimLost.
// Untuk reclaim stale broadcasting pakai ClaimIDs/ClaimBatch dengan stale threshold.
func (s *Service) ClaimPayout(ctx context.Context, payoutID int64, token string) (db.ClaimQueuedPayoutAtomicRow, error) {
	row, err := s.q.ClaimQueuedPayoutAtomic(ctx, db.ClaimQueuedPayoutAtomicParams{
		ID: payoutID, ClaimToken: pgtype.Text{String: token, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ClaimQueuedPayoutAtomicRow{}, ErrPayoutClaimLost
		}
		return db.ClaimQueuedPayoutAtomicRow{}, err
	}
	return row, nil
}

// ClaimBatch: klaim maksimal limit payout reclaimable. Token prefix unik per worker
// (misal "w1-<nanos>"). Payout yang kalah dilewati (skip), bukan error.
// Return daftar payout yang dimenangkan worker ini — tidak overlap antar worker.
func (s *Service) ClaimBatch(ctx context.Context, limit int32, tokenPrefix string, staleBefore time.Time) ([]db.ClaimQueuedPayoutAtomicRow, error) {
	ids, err := s.q.ListReclaimablePayoutIDs(ctx, db.ListReclaimablePayoutIDsParams{
		ClaimedAt: pgtype.Timestamptz{Time: staleBefore, Valid: true},
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	return s.ClaimIDs(ctx, ids, tokenPrefix, staleBefore)
}

// ClaimIDs: klaim daftar payout spesifik (untuk test isolasi / worker assignment).
// Fresh queued dicoba dulu; bila lost, coba reclaim stale broadcasting.
func (s *Service) ClaimIDs(ctx context.Context, ids []int64, tokenPrefix string, staleBefore time.Time) ([]db.ClaimQueuedPayoutAtomicRow, error) {
	var won []db.ClaimQueuedPayoutAtomicRow
	for _, id := range ids {
		row, err := s.claimOne(ctx, id, tokenPrefix+"-"+int64ToStr(id), staleBefore)
		if err != nil {
			if errors.Is(err, ErrPayoutClaimLost) {
				continue // dimenangkan worker lain — skip
			}
			return won, err
		}
		won = append(won, row)
	}
	return won, nil
}

// claimOne: fresh queued dulu, lalu stale broadcasting reclaim.
func (s *Service) claimOne(ctx context.Context, payoutID int64, token string, staleBefore time.Time) (db.ClaimQueuedPayoutAtomicRow, error) {
	row, err := s.q.ClaimQueuedPayoutAtomic(ctx, db.ClaimQueuedPayoutAtomicParams{
		ID: payoutID, ClaimToken: pgtype.Text{String: token, Valid: true},
	})
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.ClaimQueuedPayoutAtomicRow{}, err
	}
	// Bukan queued — coba reclaim stale broadcasting.
	srow, serr := s.q.ReclaimStaleBroadcastingAtomic(ctx, db.ReclaimStaleBroadcastingAtomicParams{
		ID: payoutID, ClaimToken: pgtype.Text{String: token, Valid: true},
		ClaimedAt: pgtype.Timestamptz{Time: staleBefore, Valid: true},
	})
	if serr != nil {
		if errors.Is(serr, pgx.ErrNoRows) {
			return db.ClaimQueuedPayoutAtomicRow{}, ErrPayoutClaimLost
		}
		return db.ClaimQueuedPayoutAtomicRow{}, serr
	}
	// ReclaimStale row beda tipe — konversi manual ke ClaimQueued row.
	return db.ClaimQueuedPayoutAtomicRow{
		ID: srow.ID, UserID: srow.UserID, TradeID: srow.TradeID, Asset: srow.Asset,
		Units: srow.Units, FeeUnits: srow.FeeUnits, SentUnits: srow.SentUnits,
		ToAddress: srow.ToAddress, Network: srow.Network, TxHash: srow.TxHash,
		Status: srow.Status, Confirms: srow.Confirms, Mode: srow.Mode,
		AttemptCount: srow.AttemptCount, LastError: srow.LastError,
		RefundJournalID: srow.RefundJournalID, BroadcastAt: srow.BroadcastAt,
		FinalizedAt: srow.FinalizedAt, CreatedAt: srow.CreatedAt,
	}, nil
}

// RecoveryScan: ambil work untuk restart recovery — queued semua + stale broadcasting.
// Fresh broadcasting (milik worker aktif) TIDAK diambil.
func (s *Service) RecoveryScan(ctx context.Context, limit int32) ([]int64, error) {
	return s.q.ListReclaimablePayoutIDs(ctx, db.ListReclaimablePayoutIDsParams{
		ClaimedAt: pgtype.Timestamptz{Time: time.Now().Add(-StaleBroadcastingAge), Valid: true},
		Limit:     limit,
	})
}

func int64ToStr(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	pos := len(b)
	for v > 0 {
		pos--
		b[pos] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
