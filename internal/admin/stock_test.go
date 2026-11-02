// Test stok admin v0.13 (SetFloat/SetFee/SetHotAddress) — butuh DATABASE_URL (VPS).
package admin

import (
	"context"
	"testing"
)

func TestSetFloatFeeHot(t *testing.T) {
	svc, q, _ := adminDeps(t)
	ctx := context.Background()
	// SetFloat roundtrip.
	if _, after, err := svc.SetFloat(ctx, 1, "sol", "123"); err != nil || after != "123" {
		t.Fatalf("setfloat: %v after=%q", err, after)
	}
	// SetFee roundtrip.
	if _, after, err := svc.SetFee(ctx, 1, "sol", "0.00002"); err != nil || after == "" {
		t.Fatalf("setfee: %v after=%q", err, after)
	}
	// SetHot MOCK lolos.
	if _, after, err := svc.SetHotAddress(ctx, 1, "sol", "MOCK-HOT-TEST"); err != nil || after != "MOCK-HOT-TEST" {
		t.Fatalf("sethot mock: %v after=%q", err, after)
	}
	// SetHot EVM asli-format lolos, ngawur ditolak.
	evm := "0x1234567890abcdef1234567890abcdef12345678"
	if _, _, err := svc.SetHotAddress(ctx, 1, "bnb", evm); err != nil {
		t.Fatalf("sethot evm: %v", err)
	}
	if _, _, err := svc.SetHotAddress(ctx, 1, "bnb", "0xNGGAKVALID"); err == nil {
		t.Fatal("alamat ngawur lolos!")
	}
	if _, _, err := svc.SetFloat(ctx, 1, "doge", "1"); err == nil {
		t.Fatal("coin doge lolos!")
	}
	if _, _, err := svc.SetFloat(ctx, 1, "sol", "-5"); err == nil {
		t.Fatal("float negatif lolos!")
	}
	if _, _, err := svc.SetFee(ctx, 1, "sol", "ngawur"); err == nil {
		t.Fatal("fee ngawur lolos!")
	}
	// Audit tercatat (setfloat/setfee/sethot).
	rows, err := q.RecentAdminAudit(ctx, 8)
	if err != nil || len(rows) == 0 {
		t.Fatalf("audit kosong: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.Action] = true
	}
	for _, a := range []string{"setfloat", "setfee", "sethot"} {
		if !seen[a] {
			t.Fatalf("audit %q tidak ketemu", a)
		}
	}
	// Kembalikan seed.
	if _, _, err := svc.SetFloat(ctx, 1, "sol", "100"); err != nil {
		t.Fatalf("kembalikan float: %v", err)
	}
	if _, _, err := svc.SetFee(ctx, 1, "sol", "0.00001"); err != nil {
		t.Fatalf("kembalikan fee: %v", err)
	}
	if _, _, err := svc.SetHotAddress(ctx, 1, "sol", "MOCK-DEPOSIT-SOL"); err != nil {
		t.Fatalf("kembalikan hot sol: %v", err)
	}
	if _, _, err := svc.SetHotAddress(ctx, 1, "bnb", "MOCK-DEPOSIT-BNB"); err != nil {
		t.Fatalf("kembalikan hot bnb: %v", err)
	}
}
