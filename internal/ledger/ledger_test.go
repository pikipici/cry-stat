// Test ledger Fase 1 — butuh DATABASE_URL ke Postgres (jalan di cbt-vps, bukan lokal).
// Skip otomatis kalau env kosong.
package ledger

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func testPool(t *testing.T) *pgxpool.Pool {
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
	return pool
}

func testSvc(t *testing.T) (*Service, *db.Queries) {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	set := settings.New(q)
	if err := set.Reload(ctx); err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	return New(pool, set), q
}

func mkUser(t *testing.T, q *db.Queries) int64 {
	t.Helper()
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		TelegramID: time.Now().UnixNano()%10000000000 + int64(os.Getpid()%100000),
		Mode:       "sandbox",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

func TestGrantSandboxIdempotent(t *testing.T) {
	svc, q := testSvc(t)
	ctx := context.Background()
	uid := mkUser(t, q)
	if err := svc.GrantSandbox(ctx, uid, 1000000); err != nil {
		t.Fatalf("grant 1: %v", err)
	}
	if err := svc.GrantSandbox(ctx, uid, 1000000); err != nil {
		t.Fatalf("grant 2: %v", err)
	}
	bal, err := svc.Balances(ctx, uid)
	if err != nil {
		t.Fatalf("balances: %v", err)
	}
	if !bal["IDR"].Equal(decimal.NewFromInt(1000000)) {
		t.Fatalf("grant dobel! IDR=%s, mau 1000000", bal["IDR"])
	}
	// Jurnal grant sum per aset harus 0: total IDR semua akun = 0 (house −1jt + user +1jt).
	sums, err := q.SumLedgerByOwnerAsset(ctx)
	if err != nil {
		t.Fatalf("sums: %v", err)
	}
	total := decimal.Zero
	for _, r := range sums {
		if r.Asset == "IDR" {
			total = total.Add(r.Total)
		}
	}
	if !total.IsZero() {
		t.Fatalf("jurnal tidak nol per aset! total IDR=%s", total)
	}
}

func TestGrantConcurrent(t *testing.T) {
	// 10 grant paralel → tetap 1x (idempotent + tx).
	svc, q := testSvc(t)
	ctx := context.Background()
	uid := mkUser(t, q)
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			errs[k] = svc.GrantSandbox(ctx, uid, 1000000)
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("grant paralel %d error: %v", i, e)
		}
	}
	bal, _ := svc.Balances(ctx, uid)
	if !bal["IDR"].Equal(decimal.NewFromInt(1000000)) {
		t.Fatalf("grant paralel bocor! IDR=%s", bal["IDR"])
	}
}

func TestReconcileAfterGrants(t *testing.T) {
	svc, q := testSvc(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		uid := mkUser(t, q)
		if err := svc.GrantSandbox(ctx, uid, 1000000); err != nil {
			t.Fatalf("grant %d: %v", i, err)
		}
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}
