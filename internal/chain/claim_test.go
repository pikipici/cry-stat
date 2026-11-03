// Task 4 CORRECTNESS_FIX_PLAN (CF-03): durable worker claim tests.
// Butuh DATABASE_URL (jalan di cbt-vps).
package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/shopspring/decimal"
)

// claimTestBuy: quote+execute buy, return payoutID.
func claimTestBuy(t *testing.T, ls *ledger.Service, uid int64, amount int64) int64 {
	t.Helper()
	ctx := context.Background()
	qr, err := ls.CreateQuoteBuy(ctx, uid, "SOL", amount, decimal.NewFromFloat(0.00001), "4wBqpZM9xaSheZzJSMawUKKwhdpChKbZ5eu5ky4Vigw", payoutSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	tr, err := ls.ExecuteTrade(ctx, uid, qr.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if tr.PayoutID == 0 {
		t.Fatal("ExecuteTrade harus kembalikan PayoutID")
	}
	return tr.PayoutID
}

// TestWorkerClaimNoOverlap: dua worker claim payout yang sama paralel → tepat satu menang.
func TestWorkerClaimNoOverlap(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	var wg sync.WaitGroup
	var wins atomic.Int32
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := cs.ClaimPayout(ctx, pid, fmt.Sprintf("w%d-%d", idx, time.Now().UnixNano()))
			if err == nil {
				wins.Add(1)
			} else {
				errs[idx] = err
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("claim overlap: wins=%d (mau 1) errs=%v", wins.Load(), errs)
	}
	if !errors.Is(errs[0], ErrPayoutClaimLost) && !errors.Is(errs[1], ErrPayoutClaimLost) {
		t.Fatalf("pecundang harus ErrPayoutClaimLost, dapat %v", errs)
	}
}

// TestWorkerClaimBatchNoOverlap: 4 payout, 2 worker claim daftar SAMA paralel →
// tiap ID tepat satu pemenang (isolasi via ClaimIDs, bukan scan global DB shared).
func TestWorkerClaimBatchNoOverlap(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	var pids []int64
	for i := 0; i < 4; i++ {
		uid := payoutUser(t, q)
		_ = ls.GrantSandbox(ctx, uid, 1000000)
		pids = append(pids, claimTestBuy(t, ls, uid, 50000))
	}
	stale := time.Now().Add(-StaleBroadcastingAge - time.Minute)
	var wg sync.WaitGroup
	results := make([][]int64, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			won, err := cs.ClaimIDs(ctx, pids, fmt.Sprintf("wb%d-%d", idx, time.Now().UnixNano()), stale)
			if err != nil {
				t.Errorf("batch %d: %v", idx, err)
				return
			}
			for _, r := range won {
				results[idx] = append(results[idx], r.ID)
			}
		}(i)
	}
	wg.Wait()
	seen := map[int64]int{}
	for _, ids := range results {
		for _, id := range ids {
			seen[id]++
		}
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("payout %d diklaim %d kali (mau 1): %v", id, n, results)
		}
	}
	if len(seen) != len(pids) {
		t.Fatalf("menang %d dari %d payout: %v", len(seen), len(pids), results)
	}
}

// TestStaleBroadcastingReclaim: fresh broadcasting TIDAK masuk recovery scan.
func TestStaleBroadcastingReclaim(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	// Worker 1 menang.
	if _, err := cs.ClaimPayout(ctx, pid, "w1"); err != nil {
		t.Fatalf("claim1: %v", err)
	}
	// RecoveryScan TIDAK boleh ambil fresh broadcasting.
	// limit besar: DB workspace shared numpuk payout test lama.
	rec, err := cs.RecoveryScan(ctx, 5000)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, id := range rec {
		if id == pid {
			t.Fatalf("fresh broadcasting %d masuk recovery scan", pid)
		}
	}
	// Stale-kan claimed_at manual → harus masuk scan.
	if _, err := cs.pool.Exec(ctx, `UPDATE payouts SET claimed_at=now()-interval '1 hour' WHERE id=$1`, pid); err != nil {
		t.Fatalf("stale-kan: %v", err)
	}
	rec2, err := cs.RecoveryScan(ctx, 5000)
	if err != nil {
		t.Fatalf("scan2: %v", err)
	}
	foundStale := false
	for _, id := range rec2 {
		if id == pid {
			foundStale = true
		}
	}
	if !foundStale {
		t.Fatalf("stale broadcasting %d tidak masuk recovery scan", pid)
	}
}

// TestRestartRecoveryQueued: queued payout selalu masuk recovery scan.
func TestRestartRecoveryQueued(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	rec, err := cs.RecoveryScan(ctx, 5000)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	found := false
	for _, id := range rec {
		if id == pid {
			found = true
		}
	}
	if !found {
		t.Fatalf("queued payout %d tidak masuk recovery scan", pid)
	}
}
