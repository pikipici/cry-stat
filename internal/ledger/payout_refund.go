// Package ledger: atomic payout refund (Task 3 CORRECTNESS_FIX_PLAN, CF-01/CF-02).
// Satu payout gagal = maksimal satu refund journal; jurnal + linkage commit atomik
// dalam satu DB transaction. Refund journal identity: ref_table='payouts', ref_id=payoutID.
package ledger

import (
	"context"
	"errors"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// RefundResult: hasil typed RefundPayout (Task 3).
type RefundResult int

const (
	// RefundCreated: jurnal refund baru dibuat + linkage tercatat (satu tx).
	RefundCreated RefundResult = iota + 1
	// RefundAlreadyDone: payout sudah punya refund_journal_id — return hasil lama.
	RefundAlreadyDone
)

var (
	// ErrPayoutNotRefundable: payout bukan failed / bukan milik user / trade mismatch.
	ErrPayoutNotRefundable = errors.New("payout tidak dapat di-refund (bukan failed)")
	// ErrRefundRaceLost: linkage UPDATE kena 0 row (stale/racing transition, Task 5 partial).
	ErrRefundRaceLost = errors.New("refund race lost: linkage tidak menang")
)

// RefundPayout: atomic refund satu payout dalam satu transaction.
//
// BEGIN
//
//	SELECT payout FOR UPDATE (LockPayoutForRefund)
//	validate: milik user, status=failed, refund_journal_id NULL (else AlreadyDone/NotRefundable)
//	create refund journal(ref_table=payouts, ref_id=payoutID) + balanced IDR legs
//	refresh wallet cache
//	UPDATE payout SET refund_journal_id=? WHERE id=? AND refund_journal_id IS NULL
//	 (zero-row → rollback + ErrRefundRaceLost)
//
// # COMMIT
//
// Unknown/queued/broadcasting/confirmed → ErrPayoutNotRefundable, tanpa jurnal.
// Crash window CF-01 tertutup: jurnal tidak bisa commit tanpa linkage.
func (s *Service) RefundPayout(ctx context.Context, userID, payoutID int64) (journalID int64, result RefundResult, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	// Lock payout dulu — penentu exactly-once (bukan guard di caller).
	p, err := q.LockPayoutForRefund(ctx, payoutID)
	if err != nil {
		return 0, 0, err
	}
	if p.UserID != userID {
		return 0, 0, ErrPayoutNotRefundable
	}
	if p.RefundJournalID.Valid {
		return p.RefundJournalID.Int64, RefundAlreadyDone, nil
	}
	if p.Status != "failed" {
		return 0, 0, ErrPayoutNotRefundable
	}

	// Amount: ambil dari trade (IDR) — payout row tidak simpan amountIDR.
	var amountIDR int64
	if err := tx.QueryRow(ctx, `SELECT amount_idr FROM trades WHERE id=$1`, p.TradeID).Scan(&amountIDR); err != nil {
		return 0, 0, err
	}

	u, err := q.GetUserForUpdate(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if u.Status == "blocked" {
		return 0, 0, ErrUserBlocked
	}

	amt := decimal.NewFromInt(amountIDR)
	legs := []leg{
		{owner: userOwner(userID), asset: "IDR", amount: amt},
		{owner: houseOwner("cash"), asset: "IDR", amount: amt.Neg()},
	}
	if err := validateLegs(legs); err != nil {
		return 0, 0, err
	}
	// CF-02: identity payoutID, bukan tradeID.
	j, err := q.CreateJournal(ctx, db.CreateJournalParams{
		Type:     "refund",
		RefTable: pgtype.Text{String: "payouts", Valid: true},
		RefID:    pgtype.Int8{Int64: payoutID, Valid: true},
	})
	if err != nil {
		return 0, 0, err
	}
	for _, l := range legs {
		acct, err := ensureAccount(ctx, q, l.owner, l.asset)
		if err != nil {
			return 0, 0, err
		}
		if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			JournalID: j.ID, AccountID: acct.ID, Asset: l.asset, Amount: l.amount,
		}); err != nil {
			return 0, 0, err
		}
	}
	if err := refreshWalletCache(ctx, q, userID); err != nil {
		return 0, 0, err
	}
	// Linkage dalam tx yang sama. Zero-row = race lost → rollback, bukan sukses palsu.
	n, err := q.SetPayoutRefund(ctx, db.SetPayoutRefundParams{
		ID: payoutID, RefundJournalID: pgtype.Int8{Int64: j.ID, Valid: true},
	})
	if err != nil {
		return 0, 0, err
	}
	if n == 0 {
		return 0, 0, ErrRefundRaceLost
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return j.ID, RefundCreated, nil
}
