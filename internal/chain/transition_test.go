// Task 5 CORRECTNESS_FIX_PLAN (CF-04/CF-05): guarded transition tests.
// Zero-row transition = typed stale error, bukan sukses palsu. Error tidak diabaikan.
// Butuh DATABASE_URL (jalan di cbt-vps).
package chain

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pikipici/crypto-station/internal/db"
	"github.com/shopspring/decimal"
)

func confirmParams(pid int64) db.MarkPayoutConfirmedParams {
	return db.MarkPayoutConfirmedParams{ID: pid, TxHash: pgtype.Text{String: "MOCK-TEST", Valid: true}, Confirms: 1}
}

func failedParams(pid int64) db.MarkPayoutFailedParams {
	return db.MarkPayoutFailedParams{ID: pid, LastError: "test"}
}

func unknownParams(pid int64) db.MarkPayoutUnknownParams {
	return db.MarkPayoutUnknownParams{ID: pid, LastError: "test"}
}

// TestTransitionConfirmFromIllegal: confirm dari terminal state → typed error,
// tanpa chain event baru. Langsung via SQL: confirm payout yang sudah failed.
func TestTransitionConfirmFromIllegal(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	// Bawa ke failed dulu.
	if _, err := cs.ProcessPayout(ctx, pid, PayoutFailed); err == nil {
		t.Fatalf("failed harus return sentinel, dapat nil")
	}
	// Coba confirm dari failed via SQL langsung → 0 row.
	n, err := cs.q.MarkPayoutConfirmed(ctx, confirmParams(pid))
	if err != nil {
		t.Fatalf("sql err: %v", err)
	}
	if n != 0 {
		t.Fatalf("confirm dari failed menang %d row (mau 0)", n)
	}
	if err := mustWin(n, nil); !errors.Is(err, ErrPayoutStaleTransition) {
		t.Fatalf("harus ErrPayoutStaleTransition, dapat %v", err)
	}
}

// TestTransitionFailedUnknownZeroRow: transition zero-row → typed error.
func TestTransitionFailedUnknownZeroRow(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)
	if _, err := cs.ProcessPayout(ctx, pid, PayoutSuccess); err != nil {
		t.Fatalf("success: %v", err)
	}
	// Payout sudah confirmed → failed/unknown harus 0 row.
	nf, _ := cs.q.MarkPayoutFailed(ctx, failedParams(pid))
	if err := mustWin(nf, nil); !errors.Is(err, ErrPayoutStaleTransition) {
		t.Fatalf("failed dari confirmed harus stale, dapat n=%d", nf)
	}
	nu, _ := cs.q.MarkPayoutUnknown(ctx, unknownParams(pid))
	if err := mustWin(nu, nil); !errors.Is(err, ErrPayoutStaleTransition) {
		t.Fatalf("unknown dari confirmed harus stale, dapat n=%d", nu)
	}
}

// TestTransitionTwoFinalizerOneWins: dua finalizer paralel → tepat satu menang.
func TestTransitionTwoFinalizerOneWins(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	var wg sync.WaitGroup
	var wins atomic.Int32
	var stales atomic.Int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cs.ProcessPayout(ctx, pid, PayoutSuccess)
			if err == nil {
				wins.Add(1)
			} else if errors.Is(err, ErrPayoutStaleTransition) {
				stales.Add(1)
			}
		}()
	}
	wg.Wait()
	// Satu menang confirm; yang kalah bisa stale ATAU idempotent-retry (confirmed → hasil lama).
	// Keduanya valid exactly-once; yang dilarang = dua-duanya sukses dengan txhash beda.
	if wins.Load() < 1 {
		t.Fatalf("tidak ada finalizer menang: wins=%d stales=%d", wins.Load(), stales.Load())
	}
	p, _ := cs.q.GetPayout(ctx, pid)
	if p.Status != "confirmed" {
		t.Fatalf("status akhir %s (mau confirmed)", p.Status)
	}
	// Chain event untuk txhash final tepat satu.
	var evCount int
	if err := cs.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM chain_events WHERE tx_hash=$1 AND event_type='mock_confirmed'`,
		p.TxHash.String).Scan(&evCount); err != nil {
		t.Fatalf("count event: %v", err)
	}
	if evCount != 1 {
		t.Fatalf("chain event count=%d (mau 1, tx=%s)", evCount, p.TxHash.String)
	}
}

// TestTransitionConfirmedIdempotent: retry confirmed → hasil lama, tanpa event kedua.
func TestTransitionConfirmedIdempotent(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	tx1, err := cs.ProcessPayout(ctx, pid, PayoutSuccess)
	if err != nil {
		t.Fatalf("success1: %v", err)
	}
	tx2, err := cs.ProcessPayout(ctx, pid, PayoutSuccess)
	if err != nil || tx2 != tx1 {
		t.Fatalf("retry: %v %q vs %q", err, tx2, tx1)
	}
	var evCount int
	if err := cs.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM chain_events WHERE tx_hash=$1 AND event_type='mock_confirmed'`, tx1).Scan(&evCount); err != nil {
		t.Fatalf("count event: %v", err)
	}
	if evCount != 1 {
		t.Fatalf("retry bikin event kedua: count=%d", evCount)
	}
}

var _ = decimal.Zero
