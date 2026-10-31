// Test deposit mock Fase 9 (plan v0.12).
// Butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/shopspring/decimal"
)

// TestMockDepositCredit: deposit mock → IDR masuk, float +, idempotent tx.
func TestMockDepositCredit(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	units := decimal.NewFromFloat(0.03)
	txHash := fmt.Sprintf("MOCK-TEST-%d", time.Now().UnixNano())
	dep, err := svc.MockDeposit(ctx, uid, "SOL", units, txHash, testSnap())
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if dep.AmountIDR <= 0 {
		t.Fatalf("IDR nol: %+v", dep)
	}
	bal, _ := svc.Balances(ctx, uid)
	// Grant 1jt + deposit kredit.
	want := decimal.NewFromInt(1000000 + dep.AmountIDR)
	if !bal["IDR"].Equal(want) {
		t.Fatalf("IDR=%s, mau %s", bal["IDR"], want)
	}
	// Idempotent: tx sama = hasil sama, tidak dobel.
	dep2, err := svc.MockDeposit(ctx, uid, "SOL", units, txHash, testSnap())
	if err != nil {
		t.Fatalf("deposit ulang: %v", err)
	}
	if dep2.DepositID != dep.DepositID {
		t.Fatalf("idempotent jebol: %d vs %d", dep2.DepositID, dep.DepositID)
	}
	bal2, _ := svc.Balances(ctx, uid)
	if !bal2["IDR"].Equal(want) {
		t.Fatalf("dobel kredit! IDR=%s, mau %s", bal2["IDR"], want)
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// TestFloatStatus: float mock terbaca + minimum dari settings.
func TestFloatStatus(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	bal, min, err := svc.FloatStatus(ctx, "SOL")
	if err != nil {
		t.Fatalf("float: %v", err)
	}
	if min.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("float min nol — seed 000006 belum jalan?")
	}
	_ = bal
}
