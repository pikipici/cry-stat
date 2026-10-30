// Test trial Fase 4 — butuh DATABASE_URL (jalan di cbt-vps).
package trial

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func trialDeps(t *testing.T) (*Service, *db.Queries) {
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
	return New(q, set, ledger.New(pool, set)), q
}

func TestSnapshotExposure(t *testing.T) {
	svc, _ := trialDeps(t)
	ctx := context.Background()
	if err := svc.SnapshotExposure(ctx); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	for _, a := range []string{"BNB", "ETH", "SOL"} {
		e, err := svc.q.LatestExposure(ctx, a)
		if err != nil {
			t.Fatalf("latest %s: %v", a, err)
		}
		t.Logf("%s net=%s limit_used=%s", a, e.NetPosition, e.LimitUsed)
	}
}

func TestReportRuns(t *testing.T) {
	svc, _ := trialDeps(t)
	rep := svc.Report(context.Background())
	if !strings.Contains(rep, "Reconcile:") {
		t.Fatalf("report tanpa reconcile:\n%s", rep)
	}
	if !strings.Contains(rep, "Exposure") {
		t.Fatalf("report tanpa exposure:\n%s", rep)
	}
	t.Logf("\n%s", rep)
}
