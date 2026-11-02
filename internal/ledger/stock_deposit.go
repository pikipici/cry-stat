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

var (
	// ErrStockDepositRefConflict menandakan TxRef sudah terikat pada payload lain.
	ErrStockDepositRefConflict = errors.New("stock deposit reference conflict")
	// ErrStockDepositCancelled menandakan TxRef pernah dibatalkan dan tidak dapat digunakan ulang.
	ErrStockDepositCancelled = errors.New("stock deposit reference cancelled")
	// ErrStockDepositUnauthorized menandakan TxRef milik admin lain.
	ErrStockDepositUnauthorized = errors.New("stock deposit unauthorized")
)

// StockDepositConflictError menjelaskan field identitas pertama yang tidak cocok.
type StockDepositConflictError struct {
	Field string
}

func (e *StockDepositConflictError) Error() string {
	return fmt.Sprintf("stock deposit reference conflict: field %s", e.Field)
}

// Unwrap memungkinkan caller memakai errors.Is(err, ErrStockDepositRefConflict).
func (e *StockDepositConflictError) Unwrap() error { return ErrStockDepositRefConflict }

// Is mempertahankan kecocokan sentinel saat error dibandingkan secara langsung.
func (e *StockDepositConflictError) Is(target error) bool {
	return target == ErrStockDepositRefConflict
}

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
	DepositID        int64
	JournalID        int64
	Asset            string
	Network          string
	Units            decimal.Decimal
	TxRef            string
	Balance          decimal.Decimal
	BalanceAvailable bool
	AlreadyDone      bool
}

type StockDepositPreviewState string

const (
	StockDepositPreviewCreated          StockDepositPreviewState = "created"
	StockDepositPreviewIdenticalPending StockDepositPreviewState = "identical_pending"
	StockDepositPreviewAlreadyRecorded  StockDepositPreviewState = "already_recorded"
	StockDepositPreviewCancelled        StockDepositPreviewState = "cancelled"
	StockDepositPreviewConflict         StockDepositPreviewState = "conflict"
	StockDepositPreviewUnauthorized     StockDepositPreviewState = "unauthorized"
)

type StockDepositPreviewResult struct {
	State   StockDepositPreviewState
	Deposit db.AdminStockDeposit
}

type stockDepositPreviewQueries interface {
	CreatePendingAdminStockDeposit(context.Context, db.CreatePendingAdminStockDepositParams) (db.AdminStockDeposit, error)
	GetAdminStockDepositByRef(context.Context, string) (db.AdminStockDeposit, error)
}

// CreateAdminStockDepositPreview owns duplicate-reference semantics. An identical pending row is reopened.
func (s *Service) CreateAdminStockDepositPreview(ctx context.Context, in RecordStockDepositInput) (StockDepositPreviewResult, error) {
	return createAdminStockDepositPreview(ctx, db.New(s.pool), in)
}

func createAdminStockDepositPreview(ctx context.Context, q stockDepositPreviewQueries, in RecordStockDepositInput) (StockDepositPreviewResult, error) {
	in.Asset = strings.ToUpper(strings.TrimSpace(in.Asset))
	in.ToAddress = strings.TrimSpace(in.ToAddress)
	in.TxRef = strings.TrimSpace(in.TxRef)
	dep, err := q.CreatePendingAdminStockDeposit(ctx, db.CreatePendingAdminStockDepositParams{AdminTelegramID: in.AdminTelegramID, Asset: in.Asset, Network: stockDepositNetwork(in.Asset), Units: in.Units, ToAddress: in.ToAddress, TxRef: in.TxRef})
	if err == nil {
		return StockDepositPreviewResult{State: StockDepositPreviewCreated, Deposit: dep}, nil
	}
	existing, lookupErr := q.GetAdminStockDepositByRef(ctx, in.TxRef)
	if lookupErr != nil {
		return StockDepositPreviewResult{}, err
	}
	if conflict := compareStockDepositIdentity(existing, in); conflict != nil {
		if conflict.Field == "admin_telegram_id" {
			return StockDepositPreviewResult{State: StockDepositPreviewUnauthorized}, nil
		}
		return StockDepositPreviewResult{State: StockDepositPreviewConflict}, nil
	}
	state := StockDepositPreviewConflict
	switch existing.Status {
	case "pending_confirm":
		state = StockDepositPreviewIdenticalPending
	case "recorded_mock":
		state = StockDepositPreviewAlreadyRecorded
	case "cancelled":
		state = StockDepositPreviewCancelled
	}
	return StockDepositPreviewResult{State: state, Deposit: existing}, nil
}

// RecordAdminStockDeposit mencatat perpindahan stok cold→float secara idempotent oleh TxRef.
// Deposit yang sudah recorded_mock dikembalikan tanpa membuat jurnal kedua.
func (s *Service) RecordAdminStockDeposit(ctx context.Context, in RecordStockDepositInput) (RecordStockDepositResult, error) {
	in.Asset = strings.ToUpper(strings.TrimSpace(in.Asset))
	in.ToAddress = strings.TrimSpace(in.ToAddress)
	in.TxRef = strings.TrimSpace(in.TxRef)
	if in.AdminTelegramID == 0 {
		return RecordStockDepositResult{}, fmt.Errorf("admin telegram id wajib diisi")
	}
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
		if err == nil {
			res.Balance = bal
			res.BalanceAvailable = true
		}
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
	if conflict := compareStockDepositIdentity(dep, in); conflict != nil {
		// Jangan mengembalikan detail deposit lama kepada admin lain.
		if conflict.Field == "admin_telegram_id" {
			return RecordStockDepositResult{}, false, fmt.Errorf("%w", ErrStockDepositUnauthorized)
		}
		return RecordStockDepositResult{}, false, conflict
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
	if dep.Status == "cancelled" {
		return RecordStockDepositResult{}, false, fmt.Errorf("%w", ErrStockDepositCancelled)
	}
	if dep.Status != "pending_confirm" {
		return RecordStockDepositResult{}, false, fmt.Errorf("status deposit %q tidak dapat diproses", dep.Status)
	}

	if err := s.runStockDepositHook("before-journal"); err != nil {
		return RecordStockDepositResult{}, false, err
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
	if err := s.runStockDepositHook("before-mark-recorded"); err != nil {
		return RecordStockDepositResult{}, false, err
	}
	if _, err := q.MarkAdminStockDepositRecorded(ctx, db.MarkAdminStockDepositRecordedParams{
		ID: dep.ID, JournalID: pgtype.Int8{Int64: j.ID, Valid: true},
	}); err != nil {
		return RecordStockDepositResult{}, false, err
	}
	if err := s.runStockDepositHook("before-audit"); err != nil {
		return RecordStockDepositResult{}, false, fmt.Errorf("insert audit success: %w", err)
	}
	if _, err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
		AdminTelegramID: in.AdminTelegramID,
		Action:          "wallet:stock_deposit:record",
		TargetID:        pgtype.Int8{Int64: dep.ID, Valid: true},
		Before:          fmt.Sprintf("pending %s %s", dep.Asset, dep.Units.String()),
		After:           fmt.Sprintf("recorded_mock %s %s journal=%d ref=%s", dep.Asset, dep.Units.String(), j.ID, shortStockDepositRef(dep.TxRef)),
	}); err != nil {
		return RecordStockDepositResult{}, false, fmt.Errorf("insert audit success: %w", err)
	}
	if err := s.runStockDepositHook("before-commit"); err != nil {
		return RecordStockDepositResult{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RecordStockDepositResult{}, false, err
	}
	result.JournalID = j.ID
	return result, false, nil
}

// CancelAdminStockDeposit membatalkan deposit pending milik admin dan auditnya secara atomik.
func (s *Service) CancelAdminStockDeposit(ctx context.Context, depositID, adminTelegramID int64) error {
	if depositID <= 0 {
		return fmt.Errorf("deposit id wajib positif")
	}
	if adminTelegramID == 0 {
		return fmt.Errorf("admin telegram id wajib diisi")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.pool).WithTx(tx)

	dep, err := q.GetAdminStockDepositForUpdate(ctx, depositID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w", ErrStockDepositUnauthorized)
		}
		return err
	}
	if dep.AdminTelegramID != adminTelegramID {
		return fmt.Errorf("%w", ErrStockDepositUnauthorized)
	}
	if dep.Status == "cancelled" {
		return fmt.Errorf("%w", ErrStockDepositCancelled)
	}
	if dep.Status != "pending_confirm" {
		return fmt.Errorf("status deposit %q tidak dapat dibatalkan", dep.Status)
	}
	if _, err := q.MarkAdminStockDepositCancelled(ctx, dep.ID); err != nil {
		return err
	}
	if err := s.runStockDepositHook("before-audit"); err != nil {
		return fmt.Errorf("insert audit cancel: %w", err)
	}
	if _, err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
		AdminTelegramID: adminTelegramID,
		Action:          "wallet:stock_deposit:cancel",
		TargetID:        pgtype.Int8{Int64: dep.ID, Valid: true},
		Before:          fmt.Sprintf("pending %s %s", dep.Asset, dep.Units.String()),
		After:           "cancelled",
	}); err != nil {
		return fmt.Errorf("insert audit cancel: %w", err)
	}
	if err := s.runStockDepositHook("before-commit"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) runStockDepositHook(stage string) error {
	s.stockDepositHookMu.RLock()
	hook := s.stockDepositHook
	s.stockDepositHookMu.RUnlock()
	if hook == nil {
		return nil
	}
	return hook(stage)
}

func shortStockDepositRef(ref string) string {
	if len(ref) <= 24 {
		return ref
	}
	return ref[:16] + "..."
}

// compareStockDepositIdentity membandingkan identitas immutable setelah row dikunci.
// Alamat hanya di-trim; kebijakan ini sengaja tidak mengubah huruf alamat Solana maupun EVM.
func compareStockDepositIdentity(existing db.AdminStockDeposit, in RecordStockDepositInput) *StockDepositConflictError {
	if existing.AdminTelegramID != in.AdminTelegramID {
		return &StockDepositConflictError{Field: "admin_telegram_id"}
	}
	if existing.Asset != in.Asset {
		return &StockDepositConflictError{Field: "asset"}
	}
	if existing.Network != stockDepositNetwork(in.Asset) {
		return &StockDepositConflictError{Field: "network"}
	}
	if !existing.Units.Equal(in.Units) {
		return &StockDepositConflictError{Field: "units"}
	}
	if strings.TrimSpace(existing.ToAddress) != in.ToAddress {
		return &StockDepositConflictError{Field: "to_address"}
	}
	return nil
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
