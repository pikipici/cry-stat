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
// v0.13 stok: float tipis JUGA pause beli → test topup float dulu biar guard float lolos.
func TestExposureShortOnly(t *testing.T) {
	svc, _, ledgerSvc := riskDeps(t)
	ctx := context.Background()
	// Siapin float SOL di atas min (jurnal float-in) — kalau tidak, guard float
	// yang pause duluan dan test exposure jadi tidak menguji apa-apa.
	if _, _, err := ledgerSvc.FloatStatus(ctx, "SOL"); err == nil {
		_, _ = ledgerSvc.FloatTopup(ctx, "SOL", decimal.NewFromInt(1000))
	}
	// Tanpa trade: net 0 → beli boleh.
	if err := svc.CheckBeforeQuote(ctx, "SOL", "buy"); err != nil {
		t.Fatalf("net 0 harus boleh beli: %v", err)
	}
	// Jual selalu boleh tanpa pause manual (tidak ada spike_stale).
	if err := svc.CheckBeforeQuote(ctx, "SOL", "sell"); err != nil {
		t.Fatalf("jual harus boleh: %v", err)
	}
}

// TestFloatThinPauseBuy: float di bawah min = beli pause, jual tetap jalan (plan #16).
// Deterministik: set float_min_bnb raksasa via admin → beli pause; balikin → beli jalan lagi.
func TestFloatThinPauseBuy(t *testing.T) {
	svc, q, ledgerSvc := riskDeps(t)
	ctx := context.Background()
	_ = ledgerSvc
	// Naikkan min BNB jauh di atas float aktual.
	if _, err := q.UpsertSetting(ctx, db.UpsertSettingParams{Key: "float_min_bnb", Value: "999999999"}); err != nil {
		t.Fatalf("naikkan min: %v", err)
	}
	// Reload cache settings biar guard baca angka baru.
	set := settings.New(q)
	if err := set.Reload(ctx); err != nil {
		t.Fatalf("reload: %v", err)
	}
	svcThin := New(q, set)
	if err := svcThin.CheckBeforeQuote(ctx, "BNB", "buy"); err == nil {
		t.Fatal("float tipis harus pause BELI")
	}
	// Jual tetap boleh walau float tipis (jual = nambah float).
	if err := svc.CheckBeforeQuote(ctx, "BNB", "sell"); err != nil {
		t.Fatalf("jual harus tetap boleh: %v", err)
	}
	// Balikin min ke seed (1).
	if _, err := q.UpsertSetting(ctx, db.UpsertSettingParams{Key: "float_min_bnb", Value: "1"}); err != nil {
		t.Fatalf("balikin min: %v", err)
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
