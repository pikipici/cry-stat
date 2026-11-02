package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/price"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// RecordStockDepositInput adalah data deposit stok manual admin yang dikonfirmasi.
type RecordStockDepositInput struct {
	AdminTelegramID int64
	Asset           string
	Units           decimal.Decimal
	ToAddress       string
	TxRef           string
}

// RecordStockDepositResult adalah hasil pencatatan deposit stok admin.
type RecordStockDepositResult struct {
	DepositID   int64
	JournalID   int64
	Asset       string
	Network     string
	Units       decimal.Decimal
	TxRef       string
	Balance     decimal.Decimal
	AlreadyDone bool
}

// RecordAdminStockDeposit mencatat perpindahan stok cold→float secara idempotent oleh TxRef.
// Deposit yang sudah recorded_mock dikembalikan tanpa membuat jurnal kedua.
func (s *Service) RecordAdminStockDeposit(ctx context.Context, in RecordStockDepositInput) (RecordStockDepositResult, error) {
	in.Asset = strings.ToUpper(strings.TrimSpace(in.Asset))
	in.ToAddress = strings.TrimSpace(in.ToAddress)
	in.TxRef = strings.TrimSpace(in.TxRef)
	if !price.ValidAsset(in.Asset) {
		return RecordStockDepositResult{}, fmt.Errorf("coin harus BNB/ETH/SOL")
	}
	if in.Units.LessThanOrEqual(decimal.Zero) {
		return RecordStockDepositResult{}, fmt.Errorf("units harus positif")
	}
	if in.ToAddress == "" {
		return RecordStockDepositResult{}, fmt.Errorf("alamat tujuan kosong")
	}
	if in.TxRef == "" {
		return RecordStockDepositResult{}, fmt.Errorf("tx ref kosong")
	}

	// Unique violation membuat transaksi Postgres aborted. Ulangi sekali agar row
	// dari transaksi pemenang dapat dikunci dan diproses sebagai idempotent.
	for attempt := 0; attempt < 2; attempt++ {
		res, retry, err := s.recordAdminStockDepositTx(ctx, in)
		if err != nil {
			return RecordStockDepositResult{}, err
		}
		if retry {
			continue
		}
		bal, err := db.New(s.pool).HouseFloatByAsset(ctx, res.Asset)
		if err != nil {
			return RecordStockDepositResult{}, fmt.Errorf("baca saldo float: %w", err)
		}
		res.Balance = bal
		return res, nil
	}
	return RecordStockDepositResult{}, fmt.Errorf("deposit stok bentrok, silakan ulangi")
}

func (s *Service) recordAdminStockDepositTx(ctx context.Context, in RecordStockDepositInput) (RecordStockDepositResult, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RecordStockDepositResult{}, false, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	dep, err := q.GetAdminStockDepositByRef(ctx, in.TxRef)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return RecordStockDepositResult{}, false, err
		}
		dep, err = q.CreatePendingAdminStockDeposit(ctx, db.CreatePendingAdminStockDepositParams{
			AdminTelegramID: in.AdminTelegramID,
			Asset:           in.Asset,
			Network:         stockDepositNetwork(in.Asset),
			Units:           in.Units,
			ToAddress:       in.ToAddress,
			TxRef:           in.TxRef,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return RecordStockDepositResult{}, true, nil
			}
			return RecordStockDepositResult{}, false, err
		}
	}
	dep, err = q.GetAdminStockDepositForUpdate(ctx, dep.ID)
	if err != nil {
		return RecordStockDepositResult{}, false, err
	}
	result := RecordStockDepositResult{
		DepositID: dep.ID, Asset: dep.Asset, Network: dep.Network,
		Units: dep.Units, TxRef: dep.TxRef,
	}
	if dep.Status == "recorded_mock" && dep.JournalID.Valid {
		result.JournalID = dep.JournalID.Int64
		result.AlreadyDone = true
		if err := tx.Commit(ctx); err != nil {
			return RecordStockDepositResult{}, false, err
		}
		return result, false, nil
	}
	if dep.Status != "pending_confirm" {
		return RecordStockDepositResult{}, false, fmt.Errorf("status deposit %q tidak dapat diproses", dep.Status)
	}

	j, err := q.CreateJournal(ctx, db.CreateJournalParams{
		Type: "float-in", RefTable: pgtype.Text{String: "admin_stock_deposits", Valid: true},
		RefID: pgtype.Int8{Int64: dep.ID, Valid: true},
	})
	if err != nil {
		return RecordStockDepositResult{}, false, err
	}
	for _, l := range []leg{
		{owner: houseOwner("cold"), asset: dep.Asset, amount: dep.Units.Neg()},
		{owner: houseOwner("float"), asset: dep.Asset, amount: dep.Units},
	} {
		acct, err := ensureAccount(ctx, q, l.owner, l.asset)
		if err != nil {
			return RecordStockDepositResult{}, false, err
		}
		if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			JournalID: j.ID, AccountID: acct.ID, Asset: l.asset, Amount: l.amount,
		}); err != nil {
			return RecordStockDepositResult{}, false, err
		}
	}
	if _, err := q.MarkAdminStockDepositRecorded(ctx, db.MarkAdminStockDepositRecordedParams{
		ID: dep.ID, JournalID: pgtype.Int8{Int64: j.ID, Valid: true},
	}); err != nil {
		return RecordStockDepositResult{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RecordStockDepositResult{}, false, err
	}
	result.JournalID = j.ID
	return result, false, nil
}

func stockDepositNetwork(asset string) string {
	switch asset {
	case "BNB":
		return "BSC"
	case "ETH":
		return "ERC20"
	default:
		return "SOL"
	}
}
