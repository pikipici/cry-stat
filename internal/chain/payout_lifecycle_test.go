// Phase 4 D3 (revisi V.1 A2): durable payout lifecycle tests.
// Butuh DATABASE_URL (jalan di cbt-vps). TDD: RED dulu vs kode lama.
package chain

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// payoutUser: user unik per run (telegramID nanotime — rerun aman).
func payoutUser(t *testing.T, q *db.Queries) int64 {
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

func payoutTestSetup(t *testing.T) (*Service, *ledger.Service, *db.Queries) {
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
	return New(pool, q, set), ledger.New(pool, set), q
}

// TestTradeCreatesQueuedPayout: tidak ada trade buy tanpa payout row (A2 gate).
func TestTradeCreatesQueuedPayout(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	qr, err := ls.CreateQuoteBuy(ctx, uid, "SOL", 50000, decimal.RequireFromString("0.00001"), "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA", payoutSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	tr, err := ls.ExecuteTrade(ctx, uid, qr.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if tr.PayoutID == 0 {
		t.Fatal("ExecuteTrade harus kembalikan PayoutID (queued atomik)")
	}
	p, err := q.GetPayout(ctx, tr.PayoutID)
	if err != nil {
		t.Fatalf("get payout: %v", err)
	}
	if p.Status != "queued" {
		t.Fatalf("status %s, mau queued langsung sesudah commit", p.Status)
	}
	_ = cs
}

// TestPayoutSuccessLifecycle: queued → confirmed + txhash, idempotent retry.
func TestPayoutSuccessLifecycle(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	qr, _ := ls.CreateQuoteBuy(ctx, uid, "SOL", 50000, decimal.RequireFromString("0.00001"), "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA", payoutSnap())
	tr, _ := ls.ExecuteTrade(ctx, uid, qr.ID)
	tx1, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutSuccess)
	if err != nil || tx1 == "" {
		t.Fatalf("success: %v %q", err, tx1)
	}
	// Retry confirmed = hasil lama, tanpa duplikat.
	tx2, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutSuccess)
	if err != nil || tx2 != tx1 {
		t.Fatalf("retry confirmed: %v %q vs %q", err, tx2, tx1)
	}
}

// TestPayoutFailedRefundOnce: failed → refund tepat sekali, retry aman.
func TestPayoutFailedRefundOnce(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	before, err := globalSums(ctx, q)
	if err != nil {
		t.Fatalf("baseline sums: %v", err)
	}
	balBefore, _ := ls.Balances(ctx, uid)
	qr, _ := ls.CreateQuoteBuy(ctx, uid, "SOL", 100000, decimal.RequireFromString("0.00001"), "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA", payoutSnap())
	tr, _ := ls.ExecuteTrade(ctx, uid, qr.ID)
	if _, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutFailed); err == nil {
		t.Fatal("failed harus return error sentinel")
	}
	// Orkestrasi refund seperti bot.go: refund via ledger + catat.
	jid, err := ls.RefundPayoutTx(ctx, uid, tr.AmountIDR, tr.TradeID)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if err := cs.SetRefundJournal(ctx, tr.PayoutID, jid); err != nil {
		t.Fatalf("set refund: %v", err)
	}
	balAfter, _ := ls.Balances(ctx, uid)
	// Saldo kembali = before - 0 (buy 100rb didebit, refund 100rb).
	if !balAfter["IDR"].Equal(balBefore["IDR"]) {
		t.Fatalf("refund once: before=%s after=%s", balBefore["IDR"], balAfter["IDR"])
	}
	// Retry failed+refund = sentinel, tanpa jurnal kedua (SetPayoutRefund guard).
	if _, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutFailed); err == nil {
		t.Fatal("retry failed harus tetap error")
	}
	p, _ := q.GetPayout(ctx, tr.PayoutID)
	if !p.RefundJournalID.Valid || p.RefundJournalID.Int64 != jid {
		t.Fatal("refund_journal_id harus tercatat tepat sekali")
	}
	// ReconcileGlobal absolut FAIL di workspace DB historis (SOL -54.69 warisan
	// A3/A4). Yang diassert: DELTA global sum dari flow ini = 0 (jurnal baru
	// buy+payout-failed+refund balance persis).
	if err := assertGlobalDeltaZero(ctx, q, before); err != nil {
		t.Fatalf("delta global: %v", err)
	}
}

// globalSums: snapshot sum per asset seluruh ledger (termasuk historis).
func globalSums(ctx context.Context, q *db.Queries) (map[string]string, error) {
	rows, err := q.SumLedgerByOwnerAsset(ctx)
	if err != nil {
		return nil, err
	}
	// Agregasi manual per asset dari (owner,asset,total).
	out := map[string]decimal.Decimal{}
	for _, r := range rows {
		a := strings.ToUpper(r.Asset)
		cur, ok := out[a]
		if !ok {
			cur = decimal.Zero
		}
		out[a] = cur.Add(r.Total)
	}
	m := map[string]string{}
	for a, v := range out {
		m[a] = v.String()
	}
	return m, nil
}

// assertGlobalDeltaZero: baseline vs kini harus IDENTIK — jurnal flow baru
// net-nol (balance), walau historis workspace imbalance.
func assertGlobalDeltaZero(ctx context.Context, q *db.Queries, before map[string]string) error {
	after, err := globalSums(ctx, q)
	if err != nil {
		return err
	}
	for a, b := range before {
		if after[a] != b {
			return fmt.Errorf("asset %s berubah: %s -> %s", a, b, after[a])
		}
	}
	for a, v := range after {
		if _, ok := before[a]; !ok {
			return fmt.Errorf("asset baru %s=%s", a, v)
		}
	}
	return nil
}

// TestPayoutUnknownNoRefund: unknown → hold, TIDAK ada refund.
func TestPayoutUnknownNoRefund(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	qr, _ := ls.CreateQuoteBuy(ctx, uid, "SOL", 100000, decimal.RequireFromString("0.00001"), "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA", payoutSnap())
	tr, _ := ls.ExecuteTrade(ctx, uid, qr.ID)
	if _, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutUnknown); err == nil {
		t.Fatal("unknown harus return error sentinel")
	}
	p, _ := q.GetPayout(ctx, tr.PayoutID)
	if p.Status != "unknown" {
		t.Fatalf("status %s, mau unknown", p.Status)
	}
	if p.RefundJournalID.Valid {
		t.Fatal("unknown TIDAK boleh punya refund journal")
	}
	// Unknown → success kemudian (recovery) = confirmed.
	tx, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutSuccess)
	if err != nil || tx == "" {
		t.Fatalf("unknown→success: %v %q", err, tx)
	}
}

// TestQueuedRecovery: delayed tetap queued → ClaimQueued dapat → proses OK.
func TestQueuedRecovery(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	qr, _ := ls.CreateQuoteBuy(ctx, uid, "SOL", 50000, decimal.RequireFromString("0.00001"), "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsHYdLVL9zreA", payoutSnap())
	tr, _ := ls.ExecuteTrade(ctx, uid, qr.ID)
	if _, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutDelayed); err == nil {
		t.Fatal("delayed harus return error penanda")
	}
	claimed, err := cs.ClaimQueued(ctx, 10)
	if err != nil || len(claimed) == 0 {
		t.Fatalf("claim queued: %v n=%d", err, len(claimed))
	}
	found := false
	for _, c := range claimed {
		if c.ID == tr.PayoutID {
			found = true
		}
	}
	if !found {
		t.Fatal("payout delayed harus bisa di-claim recovery")
	}
	if _, err := cs.ProcessPayout(ctx, tr.PayoutID, PayoutSuccess); err != nil {
		t.Fatalf("recovery success: %v", err)
	}
}

// payoutSnap: snapshot test (mirror ledger testSnap).
func payoutSnap() price.Snapshot {
	mid := decimal.NewFromInt(1600000)
	one := decimal.NewFromInt(1)
	buy := decimal.NewFromFloat(0.01)
	return price.Snapshot{
		Mid: mid, Ask: mid.Mul(one.Add(buy)), Bid: mid.Mul(one.Sub(buy)),
		Rate: decimal.NewFromInt(16000), RateSource: "test",
		BinanceUSDT: decimal.NewFromInt(100),
	}
}
