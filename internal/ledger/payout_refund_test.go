// Task 1 CORRECTNESS_FIX_PLAN (CF-01/CF-02): RED tests untuk atomic refund exactly-once.
// Butuh DATABASE_URL (jalan di cbt-vps). RED: minimal concurrency + crash-window FAIL
// lawan kode HEAD (RefundPayoutTx + SetPayoutRefund dua call terpisah, RefID=tradeID).
package ledger

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/shopspring/decimal"
)

const refundTestAddr = "4wBqpZM9xaSheZzJSMawUKKwhdpChKbZ5eu5ky4Vigw"

// refundBuyFailed: quote+execute buy SOL 100rb, tandai payout failed via SQL.
// Return (uid, payoutID, amountIDR, tradeID).
func refundBuyFailed(t *testing.T, svc *Service) (int64, int64, int64, int64) {
	t.Helper()
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	qr, err := svc.CreateQuoteBuy(ctx, uid, "SOL", 100000, decimal.NewFromFloat(0.00001), refundTestAddr, testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	tr, err := svc.ExecuteTrade(ctx, uid, qr.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if tr.PayoutID == 0 {
		t.Fatal("ExecuteTrade harus kembalikan PayoutID")
	}
	if _, err := svc.pool.Exec(ctx, `UPDATE payouts SET status='failed', last_error='test fail' WHERE id=$1`, tr.PayoutID); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	return uid, tr.PayoutID, tr.AmountIDR, tr.TradeID
}

// refundLink: tiru bot.go — linkage terpisah sesudah jurnal commit (crash window CF-01).
func refundLink(t *testing.T, svc *Service, payoutID, jid int64) {
	t.Helper()
	if _, err := svc.pool.Exec(context.Background(),
		`UPDATE payouts SET refund_journal_id=$2 WHERE id=$1 AND refund_journal_id IS NULL`, payoutID, jid); err != nil {
		t.Fatalf("link: %v", err)
	}
}

// TestPayoutRefundConcurrentOnce: dua goroutine refund payout yang sama → tepat satu
// effective refund. RED lawan kode HEAD: RefundPayoutTx tanpa lock payout + linkage
// terpisah → dua jurnal bisa commit sebelum guard IS NULL menang.
func TestPayoutRefundConcurrentOnce(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, amountIDR, tradeID := refundBuyFailed(t, svc)

	balBefore, _ := svc.Balances(ctx, uid)

	var wg sync.WaitGroup
	var okCount atomic.Int32
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jid, rerr := svc.RefundPayoutTx(ctx, uid, amountIDR, tradeID)
			if rerr != nil {
				errs[idx] = rerr
				return
			}
			// Linkage terpisah ala bot.go.
			if _, lerr := svc.pool.Exec(ctx,
				`UPDATE payouts SET refund_journal_id=$2 WHERE id=$1 AND refund_journal_id IS NULL`, payoutID, jid); lerr != nil {
				errs[idx] = lerr
				return
			}
			okCount.Add(1)
		}(i)
	}
	wg.Wait()

	balAfter, _ := svc.Balances(ctx, uid)
	// Tepat satu refund efektif: after == before + AmountIDR satu kali.
	want := balBefore["IDR"].Add(decimal.NewFromInt(amountIDR))
	if !balAfter["IDR"].Equal(want) {
		t.Fatalf("RED CF-01: concurrent refund tidak exactly-once: before=%s after=%s want=%s okCount=%d errs=%v",
			balBefore["IDR"], balAfter["IDR"], want, okCount.Load(), errs)
	}
}

// TestPayoutRefundRetryIdempotent: retry setelah refund selesai → tidak membuat jurnal kedua.
func TestPayoutRefundRetryIdempotent(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, amountIDR, tradeID := refundBuyFailed(t, svc)

	jid1, err := svc.RefundPayoutTx(ctx, uid, amountIDR, tradeID)
	if err != nil {
		t.Fatalf("refund1: %v", err)
	}
	refundLink(t, svc, payoutID, jid1)
	balMid, _ := svc.Balances(ctx, uid)

	// Retry: kode HEAD tidak tahu refund sudah ada (tanpa lock payout) → jurnal kedua.
	jid2, err := svc.RefundPayoutTx(ctx, uid, amountIDR, tradeID)
	if err == nil && jid2 != jid1 {
		balAfter, _ := svc.Balances(ctx, uid)
		if !balAfter["IDR"].Equal(balMid["IDR"]) {
			t.Fatalf("RED CF-01: retry bikin jurnal kedua (jid1=%d jid2=%d saldo %s→%s)",
				jid1, jid2, balMid["IDR"], balAfter["IDR"])
		}
		t.Fatalf("RED CF-01: retry bikin jurnal kedua tanpa saldo berubah (jid1=%d jid2=%d)", jid1, jid2)
	}
}

// TestPayoutRefundJournalIdentity: refund journal wajib RefTable=payouts + RefID=payoutID.
// RED CF-02: kode HEAD menulis RefID=tradeID.
func TestPayoutRefundJournalIdentity(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, amountIDR, _ := refundBuyFailed(t, svc)

	jid, err := svc.RefundPayoutTx(ctx, uid, amountIDR, 0)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	var tbl string
	var rid int64
	if err := svc.pool.QueryRow(ctx,
		`SELECT COALESCE(ref_table,''), COALESCE(ref_id,0) FROM journals WHERE id=$1`, jid).Scan(&tbl, &rid); err != nil {
		t.Fatalf("baca journal: %v", err)
	}
	if tbl != "payouts" || rid != payoutID {
		t.Fatalf("RED CF-02: refund journal identity salah: ref_table=%q ref_id=%d (mau payouts/%d)",
			tbl, rid, payoutID)
	}
}

// TestPayoutRefundUnknownRejected: unknown payout ditolak, tidak ada refund.
func TestPayoutRefundUnknownRejected(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid := tradeUser(t, svc, db.New(svc.pool))
	qr, err := svc.CreateQuoteBuy(ctx, uid, "SOL", 100000, decimal.NewFromFloat(0.00001), refundTestAddr, testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	tr, err := svc.ExecuteTrade(ctx, uid, qr.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := svc.pool.Exec(ctx, `UPDATE payouts SET status='unknown', last_error='test unknown' WHERE id=$1`, tr.PayoutID); err != nil {
		t.Fatalf("mark unknown: %v", err)
	}
	balBefore, _ := svc.Balances(ctx, uid)
	// Kode HEAD: RefundPayoutTx tidak cek status payout sama sekali → refund jalan walau unknown.
	// Setelah fix: harus ada typed error NotRefundable dan saldo tetap.
	if _, err := svc.RefundPayoutTx(ctx, uid, tr.AmountIDR, tr.TradeID); err == nil {
		balAfter, _ := svc.Balances(ctx, uid)
		if !balAfter["IDR"].Equal(balBefore["IDR"]) {
			t.Fatalf("RED CF-01: unknown payout ke-refund (saldo %s→%s)", balBefore["IDR"], balAfter["IDR"])
		}
		t.Fatalf("RED: unknown payout refund tidak ditolak (harus typed error)")
	}
}

// TestPayoutRefundCrashWindowNoPartialLink: failure injection sebelum linkage → tidak boleh
// ada jurnal yatim tanpa linkage. Kode HEAD: jurnal commit dulu, linkage kemudian —
// simulasi crash = jurnal ada tapi refund_journal_id NULL.
func TestPayoutRefundCrashWindowNoPartialLink(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, amountIDR, tradeID := refundBuyFailed(t, svc)

	// Simulasi crash: panggil RefundPayoutTx (jurnal commit) TANPA linkage.
	jid, err := svc.RefundPayoutTx(ctx, uid, amountIDR, tradeID)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	var linked bool
	if err := svc.pool.QueryRow(ctx,
		`SELECT refund_journal_id IS NOT NULL FROM payouts WHERE id=$1`, payoutID).Scan(&linked); err != nil {
		t.Fatalf("cek linkage: %v", err)
	}
	// Kode HEAD: linked==false padahal jurnal jid sudah commit → crash window TERBUKTI.
	// Test ini PASS bila atomic refund ada (jurnal+linkage 1 tx: tidak mungkin parsial),
	// FAIL (RED) selama dua-step masih dipakai.
	if !linked {
		t.Fatalf("RED CF-01 crash window: jurnal %d commit tapi refund_journal_id masih NULL (retry bisa double-credit)", jid)
	}
}
