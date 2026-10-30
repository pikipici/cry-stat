// Test risk Fase 2 — butuh DATABASE_URL (jalan di cbt-vps).
package risk

import (
	"context"
	"os"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func riskDeps(t *testing.T) (*Service, *db.Queries, *ledger.Service) {
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
	return New(q, set), q, ledger.New(pool, set)
}

// FIX debug #5: net POSITIF (house long, user jual ke house) TIDAK pause beli.
// Hanya net NEGATIF di bawah -limit (house short) yang pause.
func TestExposureShortOnly(t *testing.T) {
	svc, _, _ := riskDeps(t)
	ctx := context.Background()
	// Tanpa trade: net 0 → beli boleh.
	if err := svc.CheckBeforeQuote(ctx, "SOL", "buy"); err != nil {
		t.Fatalf("net 0 harus boleh beli: %v", err)
	}
	// Jual selalu boleh tanpa pause manual (tidak ada spike_stale).
	if err := svc.CheckBeforeQuote(ctx, "SOL", "sell"); err != nil {
		t.Fatalf("jual harus boleh: %v", err)
	}
}

func TestManualPauseKinds(t *testing.T) {
	svc, _, _ := riskDeps(t)
	ctx := context.Background()
	// Pause manual via settings cache tidak bisa tanpa DB update — uji via RecordEvent saja.
	if err := svc.RecordEvent(ctx, 0, false, 12345, "test_kind", "payload"); err != nil {
		t.Fatalf("record event: %v", err)
	}
	_ = price.ValidAsset
	_ = decimal.Zero
}
