package price

import (
	"context"
	"os"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Verifikasi sementara FIX debug #1: throttledSnapshot nulis beneran + throttle 1/menit.
func TestThrottledSnapshotWrites(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("no db")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)
	set := settings.New(q)
	if err := set.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc := New(set, q)
	svc.SetFX(decimal.NewFromInt(16000), "verify")
	svc.handleMessage([]byte(`{"stream":"solusdt@bookTicker","data":{"s":"SOLUSDT","b":"96.50","B":"1","a":"96.51","A":"1"}}`))
	var n int
	pool.QueryRow(context.Background(), `select count(*) from price_snapshots where asset='SOL'`).Scan(&n)
	if n < 1 {
		t.Fatalf("snapshot tidak ditulis!")
	}
	t.Logf("snapshots SOL=%d OK", n)
	svc.handleMessage([]byte(`{"stream":"solusdt@bookTicker","data":{"s":"SOLUSDT","b":"96.50","B":"1","a":"96.51","A":"1"}}`))
	var n2 int
	pool.QueryRow(context.Background(), `select count(*) from price_snapshots where asset='SOL'`).Scan(&n2)
	if n2 != n {
		t.Fatalf("throttle bocor: %d -> %d", n, n2)
	}
	t.Logf("throttle OK (%d tetap)", n2)
}
