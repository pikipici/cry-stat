// Package ledger: SATU-SATUNYA pintu mutasi uang (bab 11.2 plan).
// Semua mutasi dalam 1 tx Postgres. Lock order: users → ledger_accounts → wallets.
// Fase 1: grant saldo sandbox (/start) + rekonsiliasi. ExecuteTrade (beli/jual) masuk Fase 2.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var ErrUserBlocked = errors.New("user diblokir")

type Service struct {
	pool               *pgxpool.Pool
	set                *settings.Service
	stockDepositHookMu sync.RWMutex
	stockDepositHook   func(stage string) error
}

func New(pool *pgxpool.Pool, set *settings.Service) *Service {
	return &Service{pool: pool, set: set}
}

// SetStockDepositTestHook installs a per-service failure seam for transaction tests.
func (s *Service) SetStockDepositTestHook(hook func(stage string) error) {
	s.stockDepositHookMu.Lock()
	defer s.stockDepositHookMu.Unlock()
	s.stockDepositHook = hook
}

func userOwner(id int64) string { return fmt.Sprintf("user:%d", id) }

// houseOwner: house:cash | house:inventory | house:withdraw-hold | house:provider-clearing.
// house:revenue TIDAK dipakai untuk kas spread di MVP (bab 8.3 plan).
func houseOwner(kind string) string { return "house:" + kind }

// externalOwner: boundary account transit (revisi V.1 Phase 3).
// external:payout = coin keluar boundary ke wallet user (buy).
// external:deposit = coin masuk boundary dari wallet user (jual/deposit).
// Bukan saldo internal — penyeimbang agar sum per asset = 0.
func externalOwner(kind string) string { return "external:" + kind }

// validateLegs: sum amount per asset HARUS nol sebelum tulis jurnal.
// Dipanggil pre-commit di setiap mutasi (buy/deposit/stock). O(legs), murah.
func validateLegs(legs []leg) error {
	sums := map[string]decimal.Decimal{}
	for _, l := range legs {
		a := strings.ToUpper(l.asset)
		s, ok := sums[a]
		if !ok {
			s = decimal.Zero
		}
		sums[a] = s.Add(l.amount)
	}
	for a, s := range sums {
		if !s.IsZero() {
			return fmt.Errorf("jurnal tidak balance: asset %s sum=%s", a, s.String())
		}
	}
	return nil
}

// WIBToday: tanggal reset limit ikut Asia/Jakarta (bab 6 FLOW 0), bukan jam server.
func WIBToday() string {
	return timeWIB("2006-01-02")
}

func timeWIB(layout string) string {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.Now().UTC().Format(layout)
	}
	return time.Now().In(loc).Format(layout)
}

// GrantSandbox: kasih modal virtual ke user baru (jurnal grant, bukan update lepas).
// Kaki jurnal (templat bab 8.2, arah grant): house:cash −X, user IDR +X. Sum per aset = 0.
// Idempotent: cek ADA jurnal grant untuk user (FIX debug #4 — sebelumnya cek balance nonzero,
// user yang ALL-IN sampai 0 lalu /start lagi dapat grant dobel).
func (s *Service) GrantSandbox(ctx context.Context, userID int64, amountIDR int64) error {
	amount := decimal.NewFromInt(amountIDR)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	u, err := q.GetUserForUpdate(ctx, userID)
	if err != nil {
		return err
	}
	if u.Status == "blocked" {
		return ErrUserBlocked
	}
	var granted int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM journals WHERE type='grant' AND ref_table='users' AND ref_id=$1`, userID).Scan(&granted); err != nil {
		return err
	}
	if granted > 0 {
		return tx.Commit(ctx)
	}
	j, err := q.CreateJournal(ctx, db.CreateJournalParams{
		Type:     "grant",
		RefTable: pgtype.Text{String: "users", Valid: true},
		RefID:    pgtype.Int8{Int64: userID, Valid: true},
	})
	if err != nil {
		return err
	}
	house, err := ensureAccount(ctx, q, houseOwner("cash"), "IDR")
	if err != nil {
		return err
	}
	userAcct, err := ensureAccount(ctx, q, userOwner(userID), "IDR")
	if err != nil {
		return err
	}
	if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{JournalID: j.ID, AccountID: house.ID, Asset: "IDR", Amount: amount.Neg()}); err != nil {
		return err
	}
	if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{JournalID: j.ID, AccountID: userAcct.ID, Asset: "IDR", Amount: amount}); err != nil {
		return err
	}
	// Cache turunan: wallet user = total jurnal user saat ini (grant pertama = amount).
	if err := refreshWalletCache(ctx, q, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Balances: baca cache wallets (sumber kebenaran tetap jurnal; cache untuk tampil).
func (s *Service) Balances(ctx context.Context, userID int64) (map[string]decimal.Decimal, error) {
	q := db.New(s.pool)
	rows, err := q.ListWalletsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	m := map[string]decimal.Decimal{"IDR": decimal.Zero, "BNB": decimal.Zero, "ETH": decimal.Zero, "SOL": decimal.Zero}
	for _, r := range rows {
		m[strings.ToUpper(r.Asset)] = r.Balance
	}
	return m, nil
}

// Reconcile: sum jurnal per (owner,aset) == sum cache wallets per (user,aset) untuk akun user.
// Selisih ≠ 0 → error (pemanggil: alert + pause beli+jual, bab 8.1).
// FIX debug #2: dua query dibaca DALAM 1 tx (repeatable read) supaya trade yang commit
// di antara dua query tidak menimbulkan false positive selisih.
func (s *Service) Reconcile(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ`); err != nil {
		return err
	}
	q := db.New(s.pool).WithTx(tx)
	sums, err := q.SumLedgerByOwnerAsset(ctx)
	if err != nil {
		return err
	}
	wallets, err := q.SumWalletsByUserAsset(ctx)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	truth := map[string]decimal.Decimal{}
	for _, r := range sums {
		if !strings.HasPrefix(r.Owner, "user:") {
			continue
		}
		var uid int64
		if _, err := fmt.Sscanf(r.Owner, "user:%d", &uid); err != nil {
			continue
		}
		truth[fmt.Sprintf("%d/%s", uid, strings.ToUpper(r.Asset))] = r.Total
	}
	for _, w := range wallets {
		k := fmt.Sprintf("%d/%s", w.UserID, strings.ToUpper(w.Asset))
		if t, ok := truth[k]; !ok {
			if !w.Total.IsZero() {
				return fmt.Errorf("rekonsiliasi gagal: wallet %s tanpa jurnal (cache=%s)", k, w.Total)
			}
		} else if !t.Equal(w.Total) {
			return fmt.Errorf("rekonsiliasi gagal: %s jurnal=%s cache=%s", k, t, w.Total)
		}
		delete(truth, k)
	}
	for k, t := range truth {
		if !t.IsZero() {
			return fmt.Errorf("rekonsiliasi gagal: %s ada jurnal %s tapi tanpa wallet", k, t)
		}
	}
	return nil
}

// RefundPayoutTx: reversal eksplisit payout failed (Phase 4 A2).
// Satu payout = maksimal satu refund journal (guard refund_journal_id di caller).
// unknown TIDAK pernah memanggil ini — hold tetap.
func (s *Service) RefundPayoutTx(ctx context.Context, userID int64, amountIDR int64, tradeID int64) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)
	u, err := q.GetUserForUpdate(ctx, userID)
	if err != nil {
		return 0, err
	}
	if u.Status == "blocked" {
		return 0, ErrUserBlocked
	}
	amt := decimal.NewFromInt(amountIDR)
	legs := []leg{
		{owner: userOwner(userID), asset: "IDR", amount: amt},
		{owner: houseOwner("cash"), asset: "IDR", amount: amt.Neg()},
	}
	if err := validateLegs(legs); err != nil {
		return 0, err
	}
	j, err := q.CreateJournal(ctx, db.CreateJournalParams{
		Type:     "refund",
		RefTable: pgtype.Text{String: "payouts", Valid: true},
		RefID:    pgtype.Int8{Int64: tradeID, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	for _, l := range legs {
		acct, err := ensureAccount(ctx, q, l.owner, l.asset)
		if err != nil {
			return 0, err
		}
		if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			JournalID: j.ID, AccountID: acct.ID, Asset: l.asset, Amount: l.amount,
		}); err != nil {
			return 0, err
		}
	}
	if err := refreshWalletCache(ctx, q, userID); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return j.ID, nil
}

// ReconcileGlobal: invariant ledger-level — sum SEMUA entries per asset = 0.
// Mencakup house + external boundary (Reconcile lama hanya user-vs-wallet).
// Dipakai test Phase 3 D2; trial/report tetap memakai Reconcile.
// Catatan: workspace DB berisi jurnal lama imbalance (SOL -54.69 @ 2026-11-03)
// sehingga fungsi ini FAIL di data historis — itu bukti A3/A4, bukan bug fungsi.
func (s *Service) ReconcileGlobal(ctx context.Context) error {
	type row struct {
		Asset string
		Total decimal.Decimal
	}
	rows, err := s.pool.Query(ctx, `SELECT asset, SUM(amount) FROM ledger_entries GROUP BY asset`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var bad []string
	for rows.Next() {
		var asset string
		var total decimal.Decimal
		if err := rows.Scan(&asset, &total); err != nil {
			return err
		}
		if !total.IsZero() {
			bad = append(bad, asset+"="+total.String())
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("rekonsiliasi global gagal: %s", strings.Join(bad, ", "))
	}
	return nil
}

func ensureAccount(ctx context.Context, q *db.Queries, owner, asset string) (db.LedgerAccount, error) {
	if a, err := q.GetLedgerAccountForUpdate(ctx, db.GetLedgerAccountForUpdateParams{Owner: owner, Asset: asset}); err == nil {
		return a, nil
	}
	// Belum ada → buat (ON CONFLICT aman untuk race paralel: create ulang → ambil lagi).
	a, err := q.CreateLedgerAccount(ctx, db.CreateLedgerAccountParams{Owner: owner, Asset: asset})
	if err != nil {
		return q.GetLedgerAccountForUpdate(ctx, db.GetLedgerAccountForUpdateParams{Owner: owner, Asset: asset})
	}
	return a, nil
}

// refreshWalletCache: hitung ulang cache user dari jurnal (DALAM tx yang sama).
// Phase 9 (A26): 1 query targeted per owner (O(user)), menggantikan full scan.
// Semua asset user di-refresh sekaligus — caller tidak perlu 2x panggil.
func refreshWalletCache(ctx context.Context, q *db.Queries, userID int64) error {
	sums, err := q.SumLedgerByOwner(ctx, userOwner(userID))
	if err != nil {
		return err
	}
	for _, r := range sums {
		if _, err := q.UpsertWalletBalance(ctx, db.UpsertWalletBalanceParams{UserID: userID, Asset: strings.ToUpper(r.Asset), Balance: r.Total}); err != nil {
			return err
		}
	}
	return nil
}
