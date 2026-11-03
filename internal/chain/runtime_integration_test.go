package chain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProcessClaimedPayoutRejectsWrongToken(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	if _, err := cs.ClaimPayout(ctx, pid, "worker-a"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := cs.ProcessClaimedPayout(ctx, pid, "worker-b", PayoutSuccess); !errors.Is(err, ErrPayoutClaimLost) {
		t.Fatalf("wrong token harus ErrPayoutClaimLost, dapat %v", err)
	}
	p, err := cs.q.GetPayout(ctx, pid)
	if err != nil {
		t.Fatalf("get payout: %v", err)
	}
	if p.Status != "broadcasting" || p.TxHash.Valid {
		t.Fatalf("wrong-token mengubah payout: status=%s tx=%v", p.Status, p.TxHash)
	}
}

func TestRecoverQueuedClaimsAndProcesses(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)

	results, err := cs.RecoverQueued(ctx, []int64{pid}, "recovery-test", time.Now().Add(-StaleBroadcastingAge), PayoutSuccess)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(results) != 1 || results[0].PayoutID != pid || results[0].Err != nil {
		t.Fatalf("hasil recovery salah: %+v", results)
	}
	p, err := cs.q.GetPayout(ctx, pid)
	if err != nil {
		t.Fatalf("get payout: %v", err)
	}
	if p.Status != "confirmed" || !p.TxHash.Valid {
		t.Fatalf("recovery tidak finalize: status=%s tx=%v", p.Status, p.TxHash)
	}
}

func TestConfirmedEventFailureRollsBackPayout(t *testing.T) {
	cs, ls, q := payoutTestSetup(t)
	ctx := context.Background()
	uid := payoutUser(t, q)
	_ = ls.GrantSandbox(ctx, uid, 1000000)
	pid := claimTestBuy(t, ls, uid, 50000)
	if _, err := cs.ClaimPayout(ctx, pid, "event-fail-worker"); err != nil {
		t.Fatalf("claim: %v", err)
	}

	cs.beforeConfirmedEvent = func() error { return errors.New("forced event failure") }
	if _, err := cs.ProcessClaimedPayout(ctx, pid, "event-fail-worker", PayoutSuccess); err == nil {
		t.Fatal("expected forced event failure")
	}
	p, err := cs.q.GetPayout(ctx, pid)
	if err != nil {
		t.Fatalf("get payout: %v", err)
	}
	if p.Status != "broadcasting" || p.TxHash.Valid {
		t.Fatalf("confirm tidak rollback: status=%s tx=%v", p.Status, p.TxHash)
	}
}
