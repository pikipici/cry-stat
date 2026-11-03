// Phase 5 D4 (revisi V.1 A14/A17/A18): deposit conflict, address audit, SOL base58.
// DB tests jalan di VPS. SOL vectors pure unit (tanpa DB).
package ledger

import (
	"context"
	"strings"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/price"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func p5Setup(t *testing.T) (*Service, *db.Queries) {
	t.Helper()
	svc, q, _ := tradeDeps(t)
	if svc == nil {
		t.Skip("DATABASE_URL kosong — test DB hanya di VPS")
	}
	return svc, q
}

func p5User(t *testing.T, svc *Service, q *db.Queries) int64 {
	t.Helper()
	return tradeUser(t, svc, q)
}

func p5Snap() price.Snapshot {
	return testSnap()
}

// Identical retry = exactly once, tanpa jurnal kedua.
func TestDepositIdenticalRetryOnce(t *testing.T) {
	svc, q := p5Setup(t)
	ctx := context.Background()
	uid := p5User(t, svc, q)
	txh := "MOCK-P5-IDENT-" + strings.ReplaceAll(decimal.NewFromInt(int64(uid)).String(), ".", "")
	r1, err := svc.MockDeposit(ctx, uid, "SOL", decimal.RequireFromString("0.05"), txh, p5Snap())
	if err != nil {
		t.Fatalf("deposit1: %v", err)
	}
	r2, err := svc.MockDeposit(ctx, uid, "SOL", decimal.RequireFromString("0.05"), txh, p5Snap())
	if err != nil {
		t.Fatalf("retry identik harus sukses: %v", err)
	}
	if r1.DepositID != r2.DepositID {
		t.Fatalf("retry harus ID sama: %d vs %d", r1.DepositID, r2.DepositID)
	}
}

// Same hash beda user = conflict, tanpa bocor.
func TestDepositConflictUser(t *testing.T) {
	svc, q := p5Setup(t)
	ctx := context.Background()
	a := p5User(t, svc, q)
	b := p5User(t, svc, q)
	txh := "MOCK-P5-USERCONFLICT"
	if _, err := svc.MockDeposit(ctx, a, "SOL", decimal.RequireFromString("0.05"), txh, p5Snap()); err != nil {
		t.Fatalf("deposit A: %v", err)
	}
	_, err := svc.MockDeposit(ctx, b, "SOL", decimal.RequireFromString("0.05"), txh, p5Snap())
	ce, ok := err.(*DepositConflictError)
	if !ok || ce.Field != "user" {
		t.Fatalf("mau conflict user, dapat: %v", err)
	}
}

// Same hash beda asset/units = conflict.
func TestDepositConflictAssetUnits(t *testing.T) {
	svc, q := p5Setup(t)
	ctx := context.Background()
	uid := p5User(t, svc, q)
	txh := "MOCK-P5-ASSETUNITS"
	if _, err := svc.MockDeposit(ctx, uid, "SOL", decimal.RequireFromString("0.05"), txh, p5Snap()); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if _, err := svc.MockDeposit(ctx, uid, "ETH", decimal.RequireFromString("0.05"), txh, p5Snap()); err == nil {
		t.Fatal("beda asset harus conflict")
	} else if ce, ok := err.(*DepositConflictError); !ok || ce.Field != "asset" {
		t.Fatalf("mau conflict asset, dapat: %v", err)
	}
	if _, err := svc.MockDeposit(ctx, uid, "SOL", decimal.RequireFromString("0.06"), txh, p5Snap()); err == nil {
		t.Fatal("beda units harus conflict")
	} else if ce, ok := err.(*DepositConflictError); !ok || ce.Field != "units" {
		t.Fatalf("mau conflict units, dapat: %v", err)
	}
}

// Replace address: first set before kosong, replace before aktual, audit atomik.
func TestReplaceAddressAudit(t *testing.T) {
	svc, q := p5Setup(t)
	ctx := context.Background()
	uid := p5User(t, svc, q)
	tg := int64(8005001)
	addr1 := "4wBqpZM9xaSheZzJSMawUKKwhdpChKbZ5eu5ky4Vigw"
	addr2 := "a5mErVwME4geXptAs3SmoFRmbCeAoFmUMmENTfNVbbG"
	b1, a1, err := svc.ReplaceWalletAddress(ctx, uid, tg, "SOL", addr1, "SOL")
	if err != nil {
		t.Fatalf("first set: %v", err)
	}
	if b1 != "(kosong)" {
		t.Fatalf("first before mau (kosong), dapat %q", b1)
	}
	if !strings.HasSuffix(a1, addr1) {
		t.Fatalf("after mau addr1, dapat %q", a1)
	}
	b2, a2, err := svc.ReplaceWalletAddress(ctx, uid, tg, "SOL", addr2, "SOL")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !strings.HasSuffix(b2, addr1) {
		t.Fatalf("replace before mau addr1, dapat %q", b2)
	}
	if !strings.HasSuffix(a2, addr2) {
		t.Fatalf("replace after mau addr2, dapat %q", a2)
	}
	// Audit rows actual before/after.
	rows, err := q.AdminAuditByActionTarget(ctx, db.AdminAuditByActionTargetParams{Action: "alamat", TargetID: pgtype.Int8{Int64: uid, Valid: true}})
	if err != nil || len(rows) < 2 {
		t.Fatalf("audit alamat: %v n=%d", err, len(rows))
	}
	last := rows[len(rows)-1]
	if !strings.HasSuffix(last.Before, addr1) || !strings.HasSuffix(last.After, addr2) {
		t.Fatalf("audit last before/after salah: %q -> %q", last.Before, last.After)
	}
}
