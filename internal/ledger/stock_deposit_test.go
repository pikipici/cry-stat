// Test deposit stok admin v0.14 — butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func stockDepositInput(asset string, units decimal.Decimal) RecordStockDepositInput {
	return RecordStockDepositInput{
		AdminTelegramID: 9000000000 + time.Now().UnixNano()%999999,
		Asset:           asset,
		Units:           units,
		ToAddress:       "HOT-ADDRESS-TEST",
		TxRef:           fmt.Sprintf("TEST-STOCK-%d", time.Now().UnixNano()),
	}
}

// TestRecordAdminStockDepositValid membuat satu deposit, satu jurnal, dan dua kaki nol-sum.
func TestRecordAdminStockDepositValid(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositInput("sol", decimal.RequireFromString("12.345678901234567890"))
	res, err := svc.RecordAdminStockDeposit(ctx, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if res.AlreadyDone || res.Asset != "SOL" || res.Network != "SOL" || !res.Units.Equal(in.Units) {
		t.Fatalf("hasil aneh: %+v", res)
	}
	dep, err := db.New(svc.pool).GetAdminStockDepositByRef(ctx, in.TxRef)
	if err != nil {
		t.Fatalf("get deposit: %v", err)
	}
	if dep.ID != res.DepositID || dep.Status != "recorded_mock" || !dep.JournalID.Valid || dep.JournalID.Int64 != res.JournalID {
		t.Fatalf("deposit tidak tercatat benar: %+v", dep)
	}
	var legs int
	if err := svc.pool.QueryRow(ctx, "SELECT COUNT(*) FROM ledger_entries WHERE journal_id=$1", res.JournalID).Scan(&legs); err != nil {
		t.Fatalf("hitung kaki: %v", err)
	}
	if legs != 2 {
		t.Fatalf("kaki jurnal=%d, mau 2", legs)
	}
	sums, err := db.New(svc.pool).JournalSums(ctx, res.JournalID)
	if err != nil {
		t.Fatalf("journal sums: %v", err)
	}
	if len(sums) != 1 || !sums[0].Total.IsZero() {
		t.Fatalf("sum jurnal tidak nol: %+v", sums)
	}
}

// TestRecordAdminStockDepositMovesColdToFloat memastikan kedua saldo house bergerak berlawanan.
func TestRecordAdminStockDepositMovesColdToFloat(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	asset := "ETH"
	units := decimal.NewFromInt(7)
	beforeFloat, _, err := svc.FloatStatus(ctx, asset)
	if err != nil {
		t.Fatalf("float awal: %v", err)
	}
	beforeCold := ownerAssetTotal(t, ctx, q, houseOwner("cold"), asset)
	if _, err := svc.RecordAdminStockDeposit(ctx, stockDepositInput(asset, units)); err != nil {
		t.Fatalf("record: %v", err)
	}
	afterFloat, _, err := svc.FloatStatus(ctx, asset)
	if err != nil {
		t.Fatalf("float akhir: %v", err)
	}
	afterCold := ownerAssetTotal(t, ctx, q, houseOwner("cold"), asset)
	if !afterFloat.Sub(beforeFloat).Equal(units) || !afterCold.Sub(beforeCold).Equal(units.Neg()) {
		t.Fatalf("float %s→%s, cold %s→%s, mau ±%s", beforeFloat, afterFloat, beforeCold, afterCold, units)
	}
}

// TestRecordAdminStockDepositIdempotent memastikan retry TxRef tidak menambah jurnal atau float.
func TestRecordAdminStockDepositIdempotent(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositInput("BNB", decimal.NewFromInt(3))
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
		t.Fatalf("record kedua: %v", err)
	}
	if !second.AlreadyDone || second.DepositID != first.DepositID || second.JournalID != first.JournalID {
		t.Fatalf("idempotensi gagal: pertama=%+v kedua=%+v", first, second)
	}
	if !second.Balance.Equal(first.Balance) || !first.Balance.Sub(before).Equal(in.Units) {
		t.Fatalf("float berubah lagi: awal=%s pertama=%s kedua=%s", before, first.Balance, second.Balance)
	}
	sums, err := db.New(svc.pool).JournalSums(ctx, first.JournalID)
	if err != nil || len(sums) != 1 || !sums[0].Total.IsZero() {
		t.Fatalf("sum jurnal retry: %+v, %v", sums, err)
	}
}

// TestRecordAdminStockDepositRejectInvalidAsset tidak membuat row atau jurnal untuk aset tak dikenal.
func TestRecordAdminStockDepositRejectInvalidAsset(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	in := stockDepositInput("DOGE", decimal.NewFromInt(1))
	if _, err := svc.RecordAdminStockDeposit(ctx, in); err == nil {
		t.Fatal("DOGE harus ditolak")
	}
	if _, err := db.New(svc.pool).GetAdminStockDepositByRef(ctx, in.TxRef); !errorsIsNoRows(err) {
		t.Fatalf("aset invalid membuat row: %v", err)
	}
}

// TestRecordAdminStockDepositRejectNonPositive tidak meninggalkan pending row saat validasi units gagal.
func TestRecordAdminStockDepositRejectNonPositive(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	for _, units := range []decimal.Decimal{decimal.Zero, decimal.NewFromInt(-1)} {
		in := stockDepositInput("SOL", units)
		if _, err := svc.RecordAdminStockDeposit(ctx, in); err == nil {
			t.Fatalf("units %s harus ditolak", units)
		}
		if _, err := db.New(svc.pool).GetAdminStockDepositByRef(ctx, in.TxRef); !errorsIsNoRows(err) {
			t.Fatalf("units invalid meninggalkan row: %v", err)
		}
	}
}

// TestRecordAdminStockDepositRejectEmptyRef menolak TxRef kosong sebelum transaksi.
func TestRecordAdminStockDepositRejectEmptyRef(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	for _, ref := range []string{"", "   "} {
		in := stockDepositInput("SOL", decimal.NewFromInt(1))
		in.TxRef = ref
		if _, err := svc.RecordAdminStockDeposit(ctx, in); err == nil {
			t.Fatalf("TxRef %q harus ditolak", ref)
		}
	}
}

// TestRecordAdminStockDepositDifferentRefs membuat jurnal terpisah, bukan false dedup berdasarkan aset.
func TestRecordAdminStockDepositDifferentRefs(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	firstIn := stockDepositInput("SOL", decimal.NewFromInt(2))
	secondIn := stockDepositInput("SOL", decimal.NewFromInt(2))
	first, err := svc.RecordAdminStockDeposit(ctx, firstIn)
	if err != nil {
		t.Fatalf("pertama: %v", err)
	}
	second, err := svc.RecordAdminStockDeposit(ctx, secondIn)
	if err != nil {
		t.Fatalf("kedua: %v", err)
	}
	if first.DepositID == second.DepositID || first.JournalID == second.JournalID || second.AlreadyDone {
		t.Fatalf("false dedup: pertama=%+v kedua=%+v", first, second)
	}
}

func ownerAssetTotal(t *testing.T, ctx context.Context, q *db.Queries, owner, asset string) decimal.Decimal {
	t.Helper()
	for _, sum := range mustLedgerSums(t, ctx, q) {
		if sum.Owner == owner && sum.Asset == asset {
			return sum.Total
		}
	}
	return decimal.Zero
}

func mustLedgerSums(t *testing.T, ctx context.Context, q *db.Queries) []db.SumLedgerByOwnerAssetRow {
	t.Helper()
	sums, err := q.SumLedgerByOwnerAsset(ctx)
	if err != nil {
		t.Fatalf("sum ledger: %v", err)
	}
	return sums
}

func errorsIsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
