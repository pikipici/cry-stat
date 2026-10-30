// Test admin Fase 3 — butuh DATABASE_URL (jalan di cbt-vps).
package admin

import (
	"context"
	"os"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func adminDeps(t *testing.T) (*Service, *db.Queries, *settings.Service) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL kosong — test DB hanya di VPS")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("buka db: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	ctx := context.Background()
	q := db.New(pool)
	set := settings.New(q)
	if err := set.Reload(ctx); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return New(q, set), q, set
}

func TestAllowlistDeny(t *testing.T) {
	svc, _, _ := adminDeps(t)
	// Tanpa env + settings kosong → bukan admin.
	if svc.IsAdmin(999888777) {
		t.Fatal("ID asing dianggap admin!")
	}
	// Deny tercatat di audit.
	if err := svc.Deny(context.Background(), 999888777, "setspread"); err != nil {
		t.Fatalf("deny: %v", err)
	}
}

func TestAllowlistViaSettings(t *testing.T) {
	svc, _, set := adminDeps(t)
	ctx := context.Background()
	// Tulis allowlist via SetLimit (whitelist key).
	if _, _, err := svc.SetLimit(ctx, 1, "admin_allowlist", "111,222"); err != nil {
		t.Fatalf("set allowlist: %v", err)
	}
	if !svc.IsAdmin(111) || !svc.IsAdmin(222) {
		t.Fatal("allowlist settings tidak berlaku")
	}
	if svc.IsAdmin(333) {
		t.Fatal("ID luar allowlist lolos!")
	}
	// Bersihkan kembali.
	if _, _, err := svc.SetLimit(ctx, 1, "admin_allowlist", ""); err != nil {
		t.Fatalf("bersihkan: %v", err)
	}
	_ = set
}

func TestSetSpreadAudit(t *testing.T) {
	svc, q, _ := adminDeps(t)
	ctx := context.Background()
	before, after, err := svc.SetSpread(ctx, 1, "sol", "buy", "0.008")
	if err != nil {
		t.Fatalf("setspread: %v", err)
	}
	if before == "" || after != "0.008" {
		t.Fatalf("before=%q after=%q", before, after)
	}
	// Audit tercatat.
	rows, err := q.RecentAdminAudit(ctx, 3)
	if err != nil || len(rows) == 0 {
		t.Fatalf("audit kosong: %v", err)
	}
	if rows[0].Action != "setspread" {
		t.Fatalf("aksi audit=%q, mau setspread", rows[0].Action)
	}
	// Kembalikan 0.01.
	if _, _, err := svc.SetSpread(ctx, 1, "sol", "buy", "0.01"); err != nil {
		t.Fatalf("kembalikan: %v", err)
	}
}

func TestSetSpreadReject(t *testing.T) {
	svc, _, _ := adminDeps(t)
	ctx := context.Background()
	if _, _, err := svc.SetSpread(ctx, 1, "sol", "buy", "0.5"); err == nil {
		t.Fatal("spread 50% lolos, harusnya ditolak!")
	}
	if _, _, err := svc.SetSpread(ctx, 1, "doge", "buy", "0.01"); err == nil {
		t.Fatal("coin doge lolos, harusnya ditolak!")
	}
}

func TestSetLimitWhitelist(t *testing.T) {
	svc, _, _ := adminDeps(t)
	ctx := context.Background()
	if _, _, err := svc.SetLimit(ctx, 1, "database_url", "x"); err == nil {
		t.Fatal("key liar lolos whitelist!")
	}
	if _, _, err := svc.SetLimit(ctx, 1, "min_buy_idr", "bukan-angka"); err == nil {
		t.Fatal("nilai bukan angka lolos!")
	}
}

func TestPauseKinds(t *testing.T) {
	svc, _, _ := adminDeps(t)
	ctx := context.Background()
	if _, _, err := svc.Pause(ctx, 1, "sol", "exposure"); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if _, _, err := svc.Pause(ctx, 1, "sol", "off"); err != nil {
		t.Fatalf("unpause: %v", err)
	}
	if _, _, err := svc.Pause(ctx, 1, "sol", "ngawur"); err == nil {
		t.Fatal("jenis ngawur lolos!")
	}
}
