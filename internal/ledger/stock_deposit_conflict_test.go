package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/shopspring/decimal"
)

func uniqueStockDepositRef(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("TEST-STOCK-CONFLICT-%s-%d", t.Name(), time.Now().UnixNano())
}

func stockDepositConflictInput(t *testing.T) RecordStockDepositInput {
	t.Helper()
	in := stockDepositInput("SOL", decimal.RequireFromString("12.5"))
	in.TxRef = uniqueStockDepositRef(t)
	in.ToAddress = "HOT-CONFLICT-ADDRESS"
	return in
}

func stockDepositJournalCount(t *testing.T, ctx context.Context, svc *Service, depositID int64) int {
	t.Helper()
	var count int
	if err := svc.pool.QueryRow(ctx, "SELECT COUNT(*) FROM journals WHERE ref_table='admin_stock_deposits' AND ref_id=$1", depositID).Scan(&count); err != nil {
		t.Fatalf("hitung jurnal: %v", err)
	}
	return count
}

func requireStockDepositConflict(t *testing.T, err error, field string) {
	t.Helper()
	if !errors.Is(err, ErrStockDepositRefConflict) {
		t.Fatalf("error=%v, mau ErrStockDepositRefConflict", err)
	}
	var conflict *StockDepositConflictError
	if !errors.As(err, &conflict) || conflict.Field != field {
		t.Fatalf("conflict=%+v, mau field %q", conflict, field)
	}
}

func TestRecordStockDepositRetryIdenticalReturnsAlreadyDone(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	before, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil {
		t.Fatalf("float awal: %v", err)
	}
	first, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("record pertama: %v", err)
	}
	second, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("retry identik: %v", err)
	}
	if !second.AlreadyDone || second.JournalID != first.JournalID {
		t.Fatalf("retry bukan AlreadyDone jurnal sama: pertama=%+v kedua=%+v", first, second)
	}
	assertStockDepositJournalZero(t, ctx, svc, first.JournalID)
	after, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil || !after.Sub(before).Equal(in.Units) {
		t.Fatalf("float %s→%s, mau naik %s (err=%v)", before, after, in.Units, err)
	}
}

func TestRecordStockDepositSameRefDifferentAdminConflicts(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	first, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("record pertama: %v", err)
	}
	before, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil {
		t.Fatalf("float sebelum konflik: %v", err)
	}
	other := in
	other.AdminTelegramID++
	if _, err := svc.RecordAdminStockDeposit(ctx, other); !errors.Is(err, ErrStockDepositUnauthorized) {
		t.Fatalf("error=%v, mau ErrStockDepositUnauthorized", err)
	}
	after, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil || !after.Equal(before) {
		t.Fatalf("float berubah %s→%s (err=%v)", before, after, err)
	}
	if count := stockDepositJournalCount(t, ctx, svc, first.DepositID); count != 1 {
		t.Fatalf("jurnal=%d, mau 1", count)
	}
}

func TestRecordStockDepositSameRefDifferentAssetConflicts(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	if _, err := svc.RecordAdminStockDeposit(ctx, in); err != nil {
		t.Fatalf("record pertama: %v", err)
	}
	other := in
	other.Asset = "BNB"
	_, err := svc.RecordAdminStockDeposit(ctx, other)
	requireStockDepositConflict(t, err, "asset")
}

func TestRecordStockDepositSameRefDifferentUnitsConflicts(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	if _, err := svc.RecordAdminStockDeposit(ctx, in); err != nil {
		t.Fatalf("record pertama: %v", err)
	}
	other := in
	other.Units = decimal.RequireFromString("12.6")
	_, err := svc.RecordAdminStockDeposit(ctx, other)
	requireStockDepositConflict(t, err, "units")
}

func TestRecordStockDepositSameRefDifferentAddressConflicts(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	if _, err := svc.RecordAdminStockDeposit(ctx, in); err != nil {
		t.Fatalf("record pertama: %v", err)
	}
	other := in
	other.ToAddress = "HOT-OTHER-ADDRESS"
	_, err := svc.RecordAdminStockDeposit(ctx, other)
	requireStockDepositConflict(t, err, "to_address")
}

func TestRecordStockDepositCancelledRefCannotConfirm(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	dep, err := q.CreatePendingAdminStockDeposit(ctx, db.CreatePendingAdminStockDepositParams{
		AdminTelegramID: in.AdminTelegramID, Asset: in.Asset, Network: stockDepositNetwork(in.Asset),
		Units: in.Units, ToAddress: in.ToAddress, TxRef: in.TxRef,
	})
	if err != nil {
		t.Fatalf("buat pending: %v", err)
	}
	if _, err := q.MarkAdminStockDepositCancelled(ctx, dep.ID); err != nil {
		t.Fatalf("batalkan: %v", err)
	}
	if _, err := svc.RecordAdminStockDeposit(ctx, in); !errors.Is(err, ErrStockDepositCancelled) {
		t.Fatalf("error=%v, mau ErrStockDepositCancelled", err)
	}
	if count := stockDepositJournalCount(t, ctx, svc, dep.ID); count != 0 {
		t.Fatalf("jurnal cancelled=%d, mau 0", count)
	}
}

func TestRecordStockDepositConcurrentConfirmOneJournal(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	in.TxRef = fmt.Sprintf("MOCK-CONC-%d", time.Now().UnixNano())
	before, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil {
		t.Fatalf("float awal: %v", err)
	}
	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make([]RecordStockDepositResult, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ready <- struct{}{}
			<-start
			results[i], errs[i] = svc.RecordAdminStockDeposit(ctx, in)
		}(i)
	}
	<-ready
	<-ready
	close(start)
	wg.Wait()
	success, done := 0, 0
	for i, err := range errs {
		if err != nil {
			t.Fatalf("confirm %d: %v", i, err)
		}
		if results[i].AlreadyDone {
			done++
		} else {
			success++
		}
	}
	if success != 1 || done != 1 {
		t.Fatalf("hasil concurrent: success=%d alreadyDone=%d (%+v)", success, done, results)
	}
	dep, err := db.New(svc.pool).GetAdminStockDepositByRef(ctx, in.TxRef)
	if err != nil || !dep.JournalID.Valid {
		t.Fatalf("deposit concurrent=%+v err=%v", dep, err)
	}
	if count := stockDepositJournalCount(t, ctx, svc, dep.ID); count != 1 {
		t.Fatalf("jurnal=%d, mau 1", count)
	}
	assertStockDepositJournalZero(t, ctx, svc, dep.JournalID.Int64)
	after, _, err := svc.FloatStatus(ctx, in.Asset)
	if err != nil || !after.Sub(before).Equal(in.Units) {
		t.Fatalf("float %s→%s, mau naik %s (err=%v)", before, after, in.Units, err)
	}
}

func TestRecordStockDepositEighteenDecimalCanonicalRetry(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	in.Units = decimal.RequireFromString("7.5")
	first, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("record pertama: %v", err)
	}
	retry := in
	retry.Units = decimal.RequireFromString("7.500000000000000000")
	second, err := svc.RecordAdminStockDeposit(ctx, retry)
	if err != nil {
		t.Fatalf("retry canonical: %v", err)
	}
	if !second.AlreadyDone || second.JournalID != first.JournalID {
		t.Fatalf("retry canonical gagal: pertama=%+v kedua=%+v", first, second)
	}
}

func TestStockDepositConflictErrorNoLeak(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositConflictInput(t)
	in.Units = decimal.RequireFromString("11.25")
	in.ToAddress = "OLD-SECRET-ADDRESS"
	first, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("record pertama: %v", err)
	}
	other := in
	other.Units = decimal.RequireFromString("12.25")
	err = func() error {
		_, err := svc.RecordAdminStockDeposit(ctx, other)
		return err
	}()
	requireStockDepositConflict(t, err, "units")
	message := err.Error()
	for _, secret := range []string{"11.25", "OLD-SECRET-ADDRESS", fmt.Sprint(first.JournalID)} {
		if strings.Contains(message, secret) {
			t.Fatalf("error membocorkan %q: %q", secret, message)
		}
	}
}

func assertStockDepositJournalZero(t *testing.T, ctx context.Context, svc *Service, journalID int64) {
	t.Helper()
	sums, err := db.New(svc.pool).JournalSums(ctx, journalID)
	if err != nil || len(sums) != 1 || !sums[0].Total.IsZero() {
		t.Fatalf("sum jurnal=%+v err=%v", sums, err)
	}
}
