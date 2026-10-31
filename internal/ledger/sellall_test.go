// Test jual-SEMUA via CreateQuoteUnits (fix trial-live #1).
// Butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/shopspring/decimal"
)

// TestSellAllUnitsExact: beli → jual SEMUA pakai units dari saldo.
// Quote units HARUS sama persis dengan saldo (bukan roundtrip IDR).
// Sisa SOL setelah jual < dust 1e-6, reconcile tetap 0.
func TestSellAllUnitsExact(t *testing.T) {
	t.Skip("pivot transit v0.12: tidak ada saldo coin, ALL mati")
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 400000, testSnap())
	if err != nil {
		t.Fatalf("quote buy: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
		t.Fatalf("buy: %v", err)
	}
	balBefore, err := svc.Balances(ctx, uid)
	if err != nil {
		t.Fatalf("balances: %v", err)
	}
	snap := testSnap()
	qr2, err := svc.CreateQuoteUnits(ctx, uid, "SOL", balBefore["SOL"], snap)
	if err != nil {
		t.Fatalf("quote sellall: %v", err)
	}
	// Units quote = saldo persis (bukan hasil roundtrip).
	if !qr2.Units.Equal(balBefore["SOL"].Round(18)) {
		t.Fatalf("quote units %s != saldo %s", qr2.Units, balBefore["SOL"])
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr2.ID); err != nil {
		t.Fatalf("sell: %v", err)
	}
	bal, _ := svc.Balances(ctx, uid)
	if bal["SOL"].Abs().GreaterThan(dustTolerance()) {
		t.Fatalf("SOL sisa %s, melebihi dust %s", bal["SOL"], dustTolerance())
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// TestSellDustClamp: quote units 1e-7 di atas saldo (artefak rounding gaya lama)
// tetap bisa dieksekusi via clamp, bukan ErrNoBalance.
// Timpa via SQL langsung (test-only, tidak ada query prod untuk ini).
func TestSellDustClamp(t *testing.T) {
	t.Skip("pivot transit v0.12: tidak ada saldo coin, clamp sell-bandar mati")
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 200000, testSnap())
	if err != nil {
		t.Fatalf("quote buy: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
		t.Fatalf("buy: %v", err)
	}
	balBefore, _ := svc.Balances(ctx, uid)
	// Simulasi quote gaya lama: units sedikit di atas saldo.
	over := balBefore["SOL"].Add(decimal.NewFromFloat(0.00000014))
	qr2, err := svc.CreateQuoteUnits(ctx, uid, "SOL", balBefore["SOL"], testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if _, err := svc.pool.Exec(ctx, `UPDATE quotes SET quoted_units=$1 WHERE id=$2`, over.String(), qr2.ID); err != nil {
		t.Fatalf("timpa units: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr2.ID); err != nil {
		t.Fatalf("sell clamp: %v", err)
	}
}

// TestSellRealShort: jual melebihi saldo 1 SOL beneran = tetap ErrNoBalance
// (dust clamp tidak boleh menutupi kekurangan saldo asli).
func TestSellRealShort(t *testing.T) {
	t.Skip("pivot transit v0.12: tidak ada saldo coin")
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 200000, testSnap())
	if err != nil {
		t.Fatalf("quote buy: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
		t.Fatalf("buy: %v", err)
	}
	balBefore, _ := svc.Balances(ctx, uid)
	fake := balBefore["SOL"].Add(decimal.NewFromInt(1))
	qr2, err := svc.CreateQuoteUnits(ctx, uid, "SOL", balBefore["SOL"], testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if _, err := svc.pool.Exec(ctx, `UPDATE quotes SET quoted_units=$1 WHERE id=$2`, fake.String(), qr2.ID); err != nil {
		t.Fatalf("timpa units: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr2.ID); err == nil {
		t.Fatal("jual 1 SOL di atas saldo LOLOS — clamp bocor!")
	}
}
