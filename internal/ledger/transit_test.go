// Test buy transit Fase 8 (plan v0.12).
// Butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/shopspring/decimal"
)

// TestBuyTransitNoUserCoin: buy Fase 8 TIDAK menambah coin user.
// Jurnal: user IDR −, house:cash IDR +, house:float coin −sent, house:fee-collect +fee.
func TestBuyTransitNoUserCoin(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	fee := decimal.NewFromFloat(0.00001)
	qr, err := svc.CreateQuoteBuy(ctx, uid, "SOL", 200000, fee, "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA", testSnap())
	if err != nil {
		t.Fatalf("quote buy: %v", err)
	}
	if !qr.SentUnits.Equal(qr.Units.Sub(fee)) {
		t.Fatalf("sent %s != units-fee %s", qr.SentUnits, qr.Units.Sub(fee))
	}
	tr, err := svc.ExecuteTrade(ctx, uid, qr.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	bal, _ := svc.Balances(ctx, uid)
	if !bal["SOL"].IsZero() {
		t.Fatalf("SOL user %s — harus NOL (transit)!", bal["SOL"])
	}
	if !tr.SentUnits.Equal(qr.SentUnits) {
		t.Fatalf("trade sent %s != quote sent %s", tr.SentUnits, qr.SentUnits)
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// TestBuyNoAddress: buy tanpa alamat = ditolak.
func TestBuyNoAddress(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	fee := decimal.NewFromFloat(0.00001)
	if _, err := svc.CreateQuoteBuy(ctx, uid, "SOL", 200000, fee, "", testSnap()); err == nil {
		t.Fatal("buy tanpa alamat LOLOS!")
	}
}

// TestBuyFeeTooBig: fee > 20% units = ditolak (nominal terlalu kecil).
func TestBuyFeeTooBig(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	// Beli 25rb SOL ~0.015 units; fee 0.01 = >20% → tolak.
	fee := decimal.NewFromFloat(0.01)
	if _, err := svc.CreateQuoteBuy(ctx, uid, "SOL", 25000, fee, "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA", testSnap()); err == nil {
		t.Fatal("fee raksasa LOLOS!")
	}
}
