// Deposit mock Fase 9 (plan v0.12): user kirim coin ke deposit address →
// nunggu mock-konfirmasi → IDR masuk. Tanpa chain beneran (Fase 10).
package ledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/price"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// DepositResult: hasil simulasi deposit masuk.
type DepositResult struct {
	DepositID int64
	Asset     string
	Units     decimal.Decimal
	AmountIDR int64
}

// MockDeposit: catat deposit + langsung credited + jurnal (float +, user IDR +).
// Idempotent by tx_hash (kirim ulang tx sama = baris sama, bukan dobel kredit).
// minConfirms dibaca dari settings deposit_confirms_* (mock langsung lolos).
func (s *Service) MockDeposit(ctx context.Context, userID int64, asset string, units decimal.Decimal, txHash string, snap price.Snapshot) (DepositResult, error) {
	asset = strings.ToUpper(asset)
	if !price.ValidAsset(asset) {
		return DepositResult{}, fmt.Errorf("coin tidak dikenal")
	}
	if units.LessThanOrEqual(decimal.Zero) {
		return DepositResult{}, fmt.Errorf("units harus positif")
	}
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return DepositResult{}, fmt.Errorf("tx hash kosong")
	}
	// Idempotent: tx sudah ada = kembalikan yang lama.
	if d, err := db.New(s.pool).GetDepositByTx(ctx, txHash); err == nil {
		return DepositResult{DepositID: d.ID, Asset: d.Asset, Units: d.Units, AmountIDR: d.CreditedIdr}, nil
	}
	// Nilai IDR = units × bid − fee mock (sama kayak payout, arah kebalik).
	fee, _ := s.set.SafeDecimal("fee_mock_" + strings.ToLower(asset))
	net := units.Sub(fee)
	if net.LessThanOrEqual(decimal.Zero) {
		return DepositResult{}, fmt.Errorf("fee menelan seluruh deposit")
	}
	amountIDR := net.Mul(snap.Bid).Round(0).IntPart()
	if amountIDR < s.set.Int64("min_buy_idr") {
		return DepositResult{}, fmt.Errorf("%w: minimum Rp%s", ErrBelowMinimum, s.set.MustString("min_buy_idr"))
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DepositResult{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	if u, err := q.GetUserForUpdate(ctx, userID); err != nil || u.Status == "blocked" {
		if err == nil {
			return DepositResult{}, ErrUserBlocked
		}
		return DepositResult{}, err
	}
	confirms := 15
	if v, err := s.set.SafeInt64("deposit_confirms_" + strings.ToLower(asset)); err == nil {
		confirms = int(v)
	}
	var depID int64
	err = tx.QueryRow(ctx, `INSERT INTO deposits
	  (user_id, asset, from_address, tx_hash, units, confirms, required_confirms, status, credited_idr)
	  VALUES ($1,$2,'MOCK-WALLET',$3,$4,$5,$5,'credited',$6)
	  ON CONFLICT (tx_hash) DO NOTHING
	  RETURNING id`, userID, asset, txHash, units.String(), confirms, amountIDR).Scan(&depID)
	if err != nil {
		// Race: orang lain insert duluan → ambil yang ada.
		if d, err2 := db.New(s.pool).GetDepositByTx(ctx, txHash); err2 == nil {
			return DepositResult{DepositID: d.ID, Asset: d.Asset, Units: d.Units, AmountIDR: d.CreditedIdr}, nil
		}
		return DepositResult{}, err
	}
	// Jurnal deposit: house:float +units, house:cash −IDR, user IDR +IDR.
	j, err := q.CreateJournal(ctx, db.CreateJournalParams{
		Type: "deposit", RefTable: pgtype.Text{String: "deposits", Valid: true},
		RefID: pgtype.Int8{Int64: depID, Valid: true},
	})
	if err != nil {
		return DepositResult{}, err
	}
	amt := decimal.NewFromInt(amountIDR)
	for _, l := range []leg{
		{owner: houseOwner("float"), asset: asset, amount: units},
		{owner: houseOwner("cash"), asset: "IDR", amount: amt.Neg()},
		{owner: userOwner(userID), asset: "IDR", amount: amt},
	} {
		acct, err := ensureAccount(ctx, q, l.owner, l.asset)
		if err != nil {
			return DepositResult{}, err
		}
		if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			JournalID: j.ID, AccountID: acct.ID, Asset: l.asset, Amount: l.amount,
		}); err != nil {
			return DepositResult{}, err
		}
	}
	if err := refreshWalletCache(ctx, q, userID, "IDR"); err != nil {
		return DepositResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DepositResult{}, err
	}
	return DepositResult{DepositID: depID, Asset: asset, Units: units, AmountIDR: amountIDR}, nil
}

// FloatStatus: float mock per coin vs minimum (buat pause beli + /trial).
func (s *Service) FloatStatus(ctx context.Context, asset string) (bal, min decimal.Decimal, err error) {
	asset = strings.ToUpper(asset)
	// Float mock = total house:float dari jurnal (tanpa chain beneran).
	row := s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(e.amount),0) FROM ledger_entries e
	  JOIN ledger_accounts a ON a.id=e.account_id
	  WHERE a.owner='house:float' AND e.asset=$1`, asset)
	var sumStr string
	if err := row.Scan(&sumStr); err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	bal, err = decimal.NewFromString(sumStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	min, err = s.set.SafeDecimal("float_min_" + strings.ToLower(asset))
	if err != nil {
		return bal, decimal.Zero, nil
	}
	return bal, min, nil
}
