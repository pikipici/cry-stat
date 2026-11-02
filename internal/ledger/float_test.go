// Test sistem stok v0.13 (FloatTopup + guard float + set admin).
// Butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/shopspring/decimal"
)

// TestFloatTopupJournal: topup float → house:float +, house:cold −, sum per aset 0.
func TestFloatTopupJournal(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	before, _, err := svc.FloatStatus(ctx, "SOL")
	if err != nil {
		t.Fatalf("float awal: %v", err)
	}
	add := decimal.NewFromInt(50)
	res, err := svc.FloatTopup(ctx, "SOL", add)
	if err != nil {
		t.Fatalf("topup: %v", err)
	}
	if res.Asset != "SOL" || !res.Units.Equal(add) {
		t.Fatalf("hasil aneh: %+v", res)
	}
	after, _, err := svc.FloatStatus(ctx, "SOL")
	if err != nil {
		t.Fatalf("float akhir: %v", err)
	}
	if !after.Sub(before).Equal(add) {
		t.Fatalf("float %s → %s, mau +%s", before, after, add)
	}
	// Jurnal float-in sum per aset = 0 (cold −, float +).
	q := db.New(svc.pool)
	sums, err := q.JournalSums(ctx, res.JournalID)
	if err != nil {
		t.Fatalf("sums: %v", err)
	}
	for _, s := range sums {
		if !s.Total.IsZero() {
			t.Fatalf("jurnal %d aset %s tidak nol: %s", res.JournalID, s.Asset, s.Total)
		}
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// TestFloatTopupReject: units nol/negatif/coin ngawur ditolak.
func TestFloatTopupReject(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	for _, tc := range []struct {
		asset string
		units decimal.Decimal
	}{
		{"SOL", decimal.Zero},
		{"SOL", decimal.NewFromInt(-1)},
		{"DOGE", decimal.NewFromInt(1)},
	} {
		if _, err := svc.FloatTopup(ctx, tc.asset, tc.units); err == nil {
			t.Fatalf("harus ditolak: %s %s", tc.asset, tc.units)
		}
	}
}
