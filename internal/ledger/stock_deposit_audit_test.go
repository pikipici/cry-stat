// Test audit transaksi deposit stok — butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pikipici/crypto-station/internal/db"
)

func TestRecordStockDepositAuditFailureRollsBackAll(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	dep := createPendingStockDeposit(t, ctx, q, in)
	before, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil {
		t.Fatalf("float awal: %v", err)
	}
	svc.SetStockDepositTestHook(func(stage string) error {
		if stage == "before-audit" {
			return errors.New("audit dipaksa gagal")
		}
		return nil
	})
	t.Cleanup(func() { svc.SetStockDepositTestHook(nil) })
	if _, err := svc.RecordAdminStockDeposit(ctx, in); err == nil {
		t.Fatal("audit gagal harus menggagalkan record")
	}
	stored, err := q.GetAdminStockDepositByRef(ctx, in.TxRef)
	if err != nil || stored.Status != "pending_confirm" || stored.JournalID.Valid {
		t.Fatalf("deposit setelah rollback=%+v err=%v", stored, err)
	}
	if count := stockDepositJournalCount(t, ctx, svc, dep.ID); count != 0 {
		t.Fatalf("jurnal setelah rollback=%d, mau 0", count)
	}
	after, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil || !after.Equal(before) {
		t.Fatalf("float berubah %s→%s (err=%v)", before, after, err)
	}
	if count := stockDepositAuditCount(t, ctx, q, "wallet:stock_deposit:record", dep.ID); count != 0 {
		t.Fatalf("audit rollback=%d, mau 0", count)
	}
	svc.SetStockDepositTestHook(nil)
	res, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("retry setelah rollback: %v", err)
	}
	if res.AlreadyDone {
		t.Fatal("retry pertama setelah rollback tidak boleh AlreadyDone")
	}
	if count := stockDepositAuditCount(t, ctx, q, "wallet:stock_deposit:record", dep.ID); count != 1 {
		t.Fatalf("audit retry=%d, mau 1", count)
	}
}

func TestCancelStockDepositAuditFailureRollsBack(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	dep := createPendingStockDeposit(t, ctx, q, in)
	svc.SetStockDepositTestHook(func(stage string) error {
		if stage == "before-audit" {
			return errors.New("audit dipaksa gagal")
		}
		return nil
	})
	t.Cleanup(func() { svc.SetStockDepositTestHook(nil) })
	if err := svc.CancelAdminStockDeposit(ctx, dep.ID, in.AdminTelegramID); err == nil {
		t.Fatal("audit gagal harus menggagalkan cancel")
	}
	stored, err := q.GetAdminStockDepositByID(ctx, dep.ID)
	if err != nil || stored.Status != "pending_confirm" {
		t.Fatalf("deposit setelah rollback cancel=%+v err=%v", stored, err)
	}
	if count := stockDepositAuditCount(t, ctx, q, "wallet:stock_deposit:cancel", dep.ID); count != 0 {
		t.Fatalf("audit cancel rollback=%d, mau 0", count)
	}
	svc.SetStockDepositTestHook(nil)
	if err := svc.CancelAdminStockDeposit(ctx, dep.ID, in.AdminTelegramID); err != nil {
		t.Fatalf("cancel setelah rollback: %v", err)
	}
	if _, err := svc.RecordAdminStockDeposit(ctx, in); !errors.Is(err, ErrStockDepositCancelled) {
		t.Fatalf("confirm cancelled error=%v, mau ErrStockDepositCancelled", err)
	}
}

func TestRecordStockDepositWritesSuccessAudit(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	in.ToAddress = "MOCK-HOT-ADDRESS-YANG-TIDAK-BOLEH-MASUK-AUDIT"
	in.TxRef = uniqueStockDepositRef(t) + "-PANJANG-UNTUK-UJI-SHORT-REFERENCE"
	before, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil {
		t.Fatalf("float awal: %v", err)
	}
	res, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	after, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil || !after.Sub(before).Equal(in.Units) {
		t.Fatalf("float %s→%s, mau naik %s (err=%v)", before, after, in.Units, err)
	}
	auditBefore, auditAfter, count := stockDepositAudit(t, ctx, q, "wallet:stock_deposit:record", res.DepositID)
	if count != 1 {
		t.Fatalf("audit success=%d, mau 1", count)
	}
	if auditBefore != "pending SOL 12.5" {
		t.Fatalf("before audit=%q", auditBefore)
	}
	if !strings.Contains(auditAfter, "recorded_mock SOL 12.5 journal=") || !strings.Contains(auditAfter, "ref="+shortStockDepositRef(in.TxRef)) {
		t.Fatalf("after audit=%q", auditAfter)
	}
	if strings.Contains(auditBefore, in.ToAddress) || strings.Contains(auditAfter, in.ToAddress) {
		t.Fatalf("audit membocorkan alamat: before=%q after=%q", auditBefore, auditAfter)
	}
}

func TestCancelAdminStockDepositUnauthorized(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	dep := createPendingStockDeposit(t, ctx, q, in)
	if err := svc.CancelAdminStockDeposit(ctx, dep.ID, in.AdminTelegramID+1); !errors.Is(err, ErrStockDepositUnauthorized) {
		t.Fatalf("cancel error=%v, mau ErrStockDepositUnauthorized", err)
	}
	stored, err := q.GetAdminStockDepositByID(ctx, dep.ID)
	if err != nil || stored.Status != "pending_confirm" {
		t.Fatalf("deposit unauthorized cancel=%+v err=%v", stored, err)
	}
	if count := stockDepositAuditCount(t, ctx, q, "wallet:stock_deposit:cancel", dep.ID); count != 0 {
		t.Fatalf("audit unauthorized=%d, mau 0", count)
	}
}

func createPendingStockDeposit(t *testing.T, ctx context.Context, q *db.Queries, in RecordStockDepositInput) db.AdminStockDeposit {
	t.Helper()
	dep, err := q.CreatePendingAdminStockDeposit(ctx, db.CreatePendingAdminStockDepositParams{
		AdminTelegramID: in.AdminTelegramID,
		Asset:           in.Asset,
		Network:         stockDepositNetwork(in.Asset),
		Units:           in.Units,
		ToAddress:       in.ToAddress,
		TxRef:           in.TxRef,
	})
	if err != nil {
		t.Fatalf("buat pending: %v", err)
	}
	return dep
}

func stockDepositAuditCount(t *testing.T, ctx context.Context, q *db.Queries, action string, depositID int64) int {
	t.Helper()
	_, _, count := stockDepositAudit(t, ctx, q, action, depositID)
	return count
}

func stockDepositAudit(t *testing.T, ctx context.Context, q *db.Queries, action string, depositID int64) (before, after string, count int) {
	t.Helper()
	// Query exact action+target instead of scanning a moving "latest 50" window.
	// db.Queries intentionally does not expose its pool, so use the service pool
	// from the test fixture through the existing query's underlying DB is not possible;
	// add a dedicated sqlc query and use it here after regeneration.
	rows, err := q.AdminAuditByActionTarget(ctx, db.AdminAuditByActionTargetParams{
		Action:   action,
		TargetID: pgtype.Int8{Int64: depositID, Valid: true},
	})
	if err != nil {
		t.Fatalf("baca audit exact: %v", err)
	}
	for _, row := range rows {
		before, after = row.Before, row.After
		count++
	}
	return before, after, count
}
