// Test trade Fase 2 — butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func tradeDeps(t *testing.T) (*Service, *db.Queries, *settings.Service) {
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
	return New(pool, set), q, set
}

func tradeUser(t *testing.T, svc *Service, q *db.Queries) int64 {
	t.Helper()
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		TelegramID: time.Now().UnixNano()%10000000000 + int64(os.Getpid()%100000),
		Mode:       "sandbox",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := svc.GrantSandbox(context.Background(), u.ID, 1000000); err != nil {
		t.Fatalf("grant: %v", err)
	}
	return u.ID
}

func testSnap() price.Snapshot {
	mid := decimal.NewFromInt(1600000) // Rp1.6jt (SOL ~100 USDT x 16000)
	one := decimal.NewFromInt(1)
	buy := decimal.NewFromFloat(0.01)
	return price.Snapshot{
		Mid: mid, Ask: mid.Mul(one.Add(buy)), Bid: mid.Mul(one.Sub(buy)),
		Rate: decimal.NewFromInt(16000), RateSource: "test",
		BinanceUSDT: decimal.NewFromInt(100),
	}
}

func TestBuySuccess(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 200000, testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if qr.Units.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("units nol")
	}
	tr, err := svc.ExecuteTrade(ctx, uid, qr.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if tr.Units.IsZero() {
		t.Fatalf("trade units nol")
	}
	bal, _ := svc.Balances(ctx, uid)
	if !bal["SOL"].Equal(tr.Units) {
		t.Fatalf("saldo SOL=%s, trade=%s", bal["SOL"], tr.Units)
	}
	wantIDR := decimal.NewFromInt(800000)
	if !bal["IDR"].Equal(wantIDR) {
		t.Fatalf("saldo IDR=%s, mau %s", bal["IDR"], wantIDR)
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func TestDoubleTapOneWins(t *testing.T) {
	// Quote sama dieksekusi 2x paralel → 1 sukses, 1 ErrQuoteUsed.
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 200000, testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	var wg sync.WaitGroup
	res := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			_, res[k] = svc.ExecuteTrade(ctx, uid, qr.ID)
		}(i)
	}
	wg.Wait()
	ok, used := 0, 0
	for _, e := range res {
		if e == nil {
			ok++
		} else if errors.Is(e, ErrQuoteUsed) {
			used++
		} else {
			t.Fatalf("error tak terduga: %v", e)
		}
	}
	if ok != 1 || used != 1 {
		t.Fatalf("mau 1 sukses + 1 used, dapat ok=%d used=%d (%v)", ok, used, res)
	}
	bal, _ := svc.Balances(ctx, uid)
	if !bal["IDR"].Equal(decimal.NewFromInt(800000)) {
		t.Fatalf("double-spend! IDR=%s", bal["IDR"])
	}
}

func TestQuoteExpired(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, q)
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 200000, testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	// Paksa expired langsung di DB.
	_, err = svc.pool.Exec(ctx, `UPDATE quotes SET expires_at = now() - interval '1 second' WHERE id = $1`, qr.ID)
	if err != nil {
		t.Fatalf("force expire: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); !errors.Is(err, ErrQuoteUsed) {
		t.Fatalf("mau ErrQuoteUsed, dapat %v", err)
	}
}

func TestBuyInsufficientBalance(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	// Saldo 1jt, coba beli 950rb x2 (quote kedua dieksekusi setelah saldo habis sebagian).
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 950000, testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
		t.Fatalf("execute 1: %v", err)
	}
	qr2, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 500000, testSnap())
	if err != nil {
		t.Fatalf("quote 2: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr2.ID); !errors.Is(err, ErrNoBalance) {
		t.Fatalf("mau ErrNoBalance, dapat %v", err)
	}
}

func TestSellFlow(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	// Beli dulu 400rb.
	qr, _ := svc.CreateQuote(ctx, uid, "buy", "SOL", 400000, testSnap())
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
		t.Fatalf("buy: %v", err)
	}
	// Jual balik: amount IDR dihitung dari units yang dipunya x bid.
	// Catatan dust: buy units dibulatkan 18 desimal, jadi jual-ALL di level bot
	// menghitung amount dari balance. Di test ini toleransi dust kecil.
	snap := testSnap()
	balBefore, _ := svc.Balances(ctx, uid)
	sellAmount := balBefore["SOL"].Mul(snap.Bid).Round(0).IntPart()
	qr2, err := svc.CreateQuote(ctx, uid, "sell", "SOL", sellAmount, snap)
	if err != nil {
		t.Fatalf("quote sell: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr2.ID); err != nil {
		t.Fatalf("sell: %v", err)
	}
	bal, _ := svc.Balances(ctx, uid)
	// Dust < 1e-6 SOL wajar dari pembulatan 18 desimal dua arah (bukan bug ledger).
	dustLimit := decimal.NewFromFloat(0.000001)
	if bal["SOL"].Abs().GreaterThan(dustLimit) {
		t.Fatalf("SOL sisa %s, melebihi dust wajar %s", bal["SOL"], dustLimit)
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func TestBelowMinimum(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	if _, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 1000, testSnap()); !errors.Is(err, ErrBelowMinimum) {
		t.Fatalf("mau ErrBelowMinimum, dapat %v", err)
	}
}
