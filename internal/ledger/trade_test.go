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
	// Saldo 1jt, beli 950rb sukses (sisa 50rb).
	qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 950000, testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
		t.Fatalf("execute 1: %v", err)
	}
	// FIX debug #7: quote kedua (500rb > sisa 50rb) DITOLAK di awal, bukan pas execute.
	if _, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 500000, testSnap()); !errors.Is(err, ErrNoBalance) {
		t.Fatalf("mau ErrNoBalance di awal, dapat %v", err)
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

// FIX debug #3 (+ expire-otomatis): quote baru meng-expire quote lama.
// A dibuat lalu B dibuat → A expired, eksekusi A = ErrQuoteUsed, B sukses.
// Ini lapisan pertama anti-overspend (satu quote hidup per user).
// Lapisan kedua = FOR UPDATE di ExecuteTrade (antisipasi dua quote hidup via race DB).
func TestParallelDifferentQuotesNoOverspend(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	uid := mkTradeUser(t, svc, q, 1000000)
	a, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 600000, testSnap())
	if err != nil {
		t.Fatalf("quote A: %v", err)
	}
	b, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 600000, testSnap())
	if err != nil {
		t.Fatalf("quote B: %v", err)
	}
	// A sudah expired oleh B → eksekusi A gagal, B sukses.
	if _, err := svc.ExecuteTrade(ctx, uid, a.ID); !errors.Is(err, ErrQuoteUsed) {
		t.Fatalf("quote lama mau ErrQuoteUsed, dapat %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, b.ID); err != nil {
		t.Fatalf("quote baru: %v", err)
	}
	bal, _ := svc.Balances(ctx, uid)
	if !bal["IDR"].Equal(decimal.NewFromInt(400000)) {
		t.Fatalf("IDR=%s, mau 400000 (cuma 1 trade jalan)", bal["IDR"])
	}
}

// FIX debug #3 (lapisan lock): dua quote ACTIVE sekaligus (insert langsung, bypass
// expire-otomatis) dieksekusi paralel → 1 sukses + 1 ErrNoBalance, saldo tidak minus.
// Tanpa FOR UPDATE, dua-duanya baca cache cukup dan dua-duanya lolos (overspend).
func TestParallelActiveQuotesLock(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	uid := mkTradeUser(t, svc, q, 1000000)
	snap := testSnap()
	mk := func(amount int64) int64 {
		t.Helper()
		var id int64
		err := svc.pool.QueryRow(ctx, `INSERT INTO quotes
		  (user_id, side, asset, amount_idr, quoted_units, bid, ask, mid, spread_used,
		   rate_usdt_idr, rate_source, expires_at)
		  VALUES ($1,'buy','SOL',$2,$3,$4,$5,$6,0.01,16000,'test', now() + interval '30 seconds')
		  RETURNING id`,
			uid, amount,
			decimal.NewFromInt(amount).Div(snap.Ask).Round(18),
			snap.Bid, snap.Ask, snap.Mid).Scan(&id)
		if err != nil {
			t.Fatalf("insert quote: %v", err)
		}
		return id
	}
	a, b := mk(600000), mk(600000)
	var wg sync.WaitGroup
	res := make([]error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, res[0] = svc.ExecuteTrade(ctx, uid, a) }()
	go func() { defer wg.Done(); _, res[1] = svc.ExecuteTrade(ctx, uid, b) }()
	wg.Wait()
	ok, fail := 0, 0
	for _, e := range res {
		if e == nil {
			ok++
		} else if errors.Is(e, ErrNoBalance) || errors.Is(e, ErrQuoteUsed) {
			fail++
		} else {
			t.Fatalf("error tak terduga: %v", e)
		}
	}
	if ok != 1 || fail != 1 {
		t.Fatalf("mau 1 sukses + 1 gagal, dapat ok=%d gagal=%d (%v)", ok, fail, res)
	}
	bal, _ := svc.Balances(ctx, uid)
	if bal["IDR"].IsNegative() {
		t.Fatalf("saldo minus! IDR=%s", bal["IDR"])
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// FIX debug #4: grant tidak dobel walau balance 0 (user ALL-IN lalu /start lagi).
func TestGrantNoDoubleAfterZero(t *testing.T) {
	svc, q, _ := tradeDeps(t)
	ctx := context.Background()
	uid := mkTradeUser(t, svc, q, 100000)
	// Habiskan: beli 50000 x2 = 100rb → IDR 0.
	for i := 0; i < 2; i++ {
		qr, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 50000, testSnap())
		if err != nil {
			t.Fatalf("quote %d: %v", i, err)
		}
		if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
			t.Fatalf("execute %d: %v", i, err)
		}
	}
	bal, _ := svc.Balances(ctx, uid)
	if !bal["IDR"].IsZero() {
		t.Fatalf("IDR=%s, mau 0", bal["IDR"])
	}
	// /start lagi → grant harus DITOLAK (sudah pernah grant).
	if err := svc.GrantSandbox(ctx, uid, 1000000); err != nil {
		t.Fatalf("grant ulang error: %v", err)
	}
	bal2, _ := svc.Balances(ctx, uid)
	if !bal2["IDR"].IsZero() {
		t.Fatalf("grant dobel! IDR=%s setelah grant ulang", bal2["IDR"])
	}
}

// FIX debug #7: quote beli melebihi saldo ditolak di awal (bukan pas confirm).
func TestQuoteInsufficientUpfront(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool)) // 1jt
	if _, err := svc.CreateQuote(ctx, uid, "buy", "SOL", 9000000, testSnap()); !errors.Is(err, ErrNoBalance) {
		t.Fatalf("mau ErrNoBalance di awal, dapat %v", err)
	}
}

func mkTradeUser(t *testing.T, svc *Service, q *db.Queries, grant int64) int64 {
	t.Helper()
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		TelegramID: time.Now().UnixNano()%10000000000 + int64(os.Getpid()%100000),
		Mode:       "sandbox",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := svc.GrantSandbox(context.Background(), u.ID, grant); err != nil {
		t.Fatalf("grant: %v", err)
	}
	return u.ID
}
