// Task 3 CORRECTNESS_FIX_PLAN (CF-01/CF-02): GREEN tests untuk atomic refund.
// API baru RefundPayout(userID, payoutID) — lock+jurnal+linkage 1 tx, typed result.
// Butuh DATABASE_URL (jalan di cbt-vps).
package ledger

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/shopspring/decimal"
)

const refundTestAddr = "4wBqpZM9xaSheZzJSMawUKKwhdpChKbZ5eu5ky4Vigw"

// refundBuyFailed: quote+execute buy SOL 100rb, tandai payout failed via SQL.
// Return (uid, payoutID, amountIDR).
func refundBuyFailed(t *testing.T, svc *Service) (int64, int64, int64) {
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
	return uid, tr.PayoutID, tr.AmountIDR
}

// TestPayoutRefundConcurrentOnce: dua goroutine refund payout yang sama → tepat satu
// Created, satu AlreadyDone; saldo nambah tepat satu kali.
func TestPayoutRefundConcurrentOnce(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, amountIDR := refundBuyFailed(t, svc)

	balBefore, _ := svc.Balances(ctx, uid)

	var wg sync.WaitGroup
	var created, alreadyDone atomic.Int32
	errs := make([]error, 2)
	jids := make([]int64, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jid, res, rerr := svc.RefundPayout(ctx, uid, payoutID)
			jids[idx] = jid
			if rerr != nil {
				errs[idx] = rerr
				return
			}
			if res == RefundCreated {
				created.Add(1)
			} else if res == RefundAlreadyDone {
				alreadyDone.Add(1)
			}
		}(i)
	}
	wg.Wait()

	// Tepat satu Created + satu AlreadyDone, tanpa error mentah (unique violation dll).
	if created.Load() != 1 || alreadyDone.Load() != 1 {
		t.Fatalf("concurrent: created=%d alreadyDone=%d (mau 1/1) errs=%v jids=%v",
			created.Load(), alreadyDone.Load(), errs, jids)
	}
	// Kedua caller dapat journal ID yang sama.
	if jids[0] != jids[1] || jids[0] == 0 {
		t.Fatalf("concurrent: journal ID berbeda/nol: %v", jids)
	}
	balAfter, _ := svc.Balances(ctx, uid)
	want := balBefore["IDR"].Add(decimal.NewFromInt(amountIDR))
	if !balAfter["IDR"].Equal(want) {
		t.Fatalf("concurrent: saldo before=%s after=%s want=%s (double-credit?)",
			balBefore["IDR"], balAfter["IDR"], want)
	}
}

// TestPayoutRefundRetryIdempotent: retry setelah refund selesai → AlreadyDone + jid sama.
func TestPayoutRefundRetryIdempotent(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, _ := refundBuyFailed(t, svc)

	jid1, res1, err := svc.RefundPayout(ctx, uid, payoutID)
	if err != nil || res1 != RefundCreated {
		t.Fatalf("refund1: jid=%d res=%d err=%v", jid1, res1, err)
	}
	jid2, res2, err := svc.RefundPayout(ctx, uid, payoutID)
	if err != nil {
		t.Fatalf("retry: harus AlreadyDone tanpa error, dapat err=%v", err)
	}
	if res2 != RefundAlreadyDone || jid2 != jid1 {
		t.Fatalf("retry: res=%d jid2=%d (mau AlreadyDone/%d)", res2, jid2, jid1)
	}
	// Jurnal refund untuk payout ini tepat satu.
	var cnt int
	if err := svc.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM journals WHERE type='refund' AND ref_table='payouts' AND ref_id=$1`, payoutID).Scan(&cnt); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("retry: jurnal refund count=%d (mau 1)", cnt)
	}
}

// TestPayoutRefundJournalIdentity: refund journal wajib RefTable=payouts + RefID=payoutID.
func TestPayoutRefundJournalIdentity(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, _ := refundBuyFailed(t, svc)

	jid, _, err := svc.RefundPayout(ctx, uid, payoutID)
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
		t.Fatalf("identity: ref_table=%q ref_id=%d (mau payouts/%d)", tbl, rid, payoutID)
	}
}

// TestPayoutRefundUnknownRejected: unknown payout ditolak + saldo tetap.
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
	if _, _, err := svc.RefundPayout(ctx, uid, tr.PayoutID); !errors.Is(err, ErrPayoutNotRefundable) {
		t.Fatalf("unknown: harus ErrPayoutNotRefundable, dapat %v", err)
	}
	balAfter, _ := svc.Balances(ctx, uid)
	if !balAfter["IDR"].Equal(balBefore["IDR"]) {
		t.Fatalf("unknown: saldo berubah %s→%s", balBefore["IDR"], balAfter["IDR"])
	}
}

// TestPayoutRefundCrashWindowNoPartialLink: jurnal + linkage atomik — tidak ada
// jurnal yatim (commit tanpa linkage). Panggil refund, lalu pastikan linkage ada.
func TestPayoutRefundCrashWindowNoPartialLink(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	uid, payoutID, _ := refundBuyFailed(t, svc)

	jid, res, err := svc.RefundPayout(ctx, uid, payoutID)
	if err != nil || res != RefundCreated {
		t.Fatalf("refund: jid=%d res=%d err=%v", jid, res, err)
	}
	var linked bool
	if err := svc.pool.QueryRow(ctx,
		`SELECT refund_journal_id IS NOT NULL FROM payouts WHERE id=$1`, payoutID).Scan(&linked); err != nil {
		t.Fatalf("cek linkage: %v", err)
	}
	if !linked {
		t.Fatalf("crash window: jurnal %d commit tapi refund_journal_id NULL", jid)
	}
	// Jurnal yatim untuk payout ini = 0 (semua refund journal ter-link).
	var orphans int
	if err := svc.pool.QueryRow(ctx, `SELECT COUNT(*) FROM journals j
		WHERE j.type='refund' AND j.ref_table='payouts' AND j.ref_id=$1
		AND NOT EXISTS (SELECT 1 FROM payouts p WHERE p.id=$1 AND p.refund_journal_id=j.id)`,
		payoutID).Scan(&orphans); err != nil {
		t.Fatalf("cek orphans: %v", err)
	}
	if orphans != 0 {
		t.Fatalf("crash window: %d jurnal yatim untuk payout %d", orphans, payoutID)
	}
}

// TestPayoutRefundWrongUser: user lain tidak bisa refund payout orang.
func TestPayoutRefundWrongUser(t *testing.T) {
	svc, _, _ := tradeDeps(t)
	ctx := context.Background()
	_, payoutID, _ := refundBuyFailed(t, svc)
	other := tradeUser(t, svc, db.New(svc.pool))
	if _, _, err := svc.RefundPayout(ctx, other, payoutID); !errors.Is(err, ErrPayoutNotRefundable) {
		t.Fatalf("wrong user: harus ErrPayoutNotRefundable, dapat %v", err)
	}
}
