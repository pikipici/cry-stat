// Phase 9 D8 (revisi V.1 A26/A27): targeted agg equivalence + DB guards.
// Butuh DATABASE_URL (jalan di VPS).
package ledger

import (
	"context"
	"strings"
	"testing"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/shopspring/decimal"
)

// TestTargetedAggEquivalence: SumLedgerByOwner == filter SumLedgerByOwnerAsset.
func TestTargetedAggEquivalence(t *testing.T) {
	svc, q := p5Setup(t)
	ctx := context.Background()
	uid := p5User(t, svc, q)
	if err := svc.GrantSandbox(ctx, uid, 1000000); err != nil {
		t.Fatalf("grant: %v", err)
	}
	// Targeted.
	got, err := q.SumLedgerByOwner(ctx, userOwner(uid))
	if err != nil {
		t.Fatalf("targeted: %v", err)
	}
	// Full scan difilter.
	all, err := q.SumLedgerByOwnerAsset(ctx)
	if err != nil {
		t.Fatalf("full: %v", err)
	}
	want := map[string]decimal.Decimal{}
	for _, r := range all {
		if r.Owner == userOwner(uid) {
			want[strings.ToUpper(r.Asset)] = r.Total
		}
	}
	if len(got) != len(want) {
		t.Fatalf("jumlah asset beda: targeted=%d full=%d", len(got), len(want))
	}
	for _, r := range got {
		w, ok := want[strings.ToUpper(r.Asset)]
		if !ok || !w.Equal(r.Total) {
			t.Fatalf("asset %s: targeted=%s full=%s", r.Asset, r.Total, w)
		}
	}
}

// TestTradeJournalBalanceDB: jurnal trade baru balance per asset (DB-level).
// Verifikasi via ReconcileGlobal delta-nol + cache konsisten.
func TestTradeJournalBalanceDB(t *testing.T) {
	svc, q := p5Setup(t)
	ctx := context.Background()
	uid := p5User(t, svc, q)
	before, err := globalSumsChain(ctx, q)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	qr, err := svc.CreateQuoteBuy(ctx, uid, "SOL", 200000, decimal.RequireFromString("0.00001"), "4wBqpZM9xaSheZzJSMawUKKwhdpChKbZ5eu5ky4Vigw", testSnap())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if _, err := svc.ExecuteTrade(ctx, uid, qr.ID); err != nil {
		t.Fatalf("execute: %v", err)
	}
	after, err := globalSumsChain(ctx, q)
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	for a, b := range before {
		if after[a] != b {
			t.Fatalf("delta global asset %s: %s -> %s", a, b, after[a])
		}
	}
}

// globalSumsChain: duplikat lokal (hindari import cycle chain<->ledger test).
func globalSumsChain(ctx context.Context, q *db.Queries) (map[string]string, error) {
	rows, err := q.SumLedgerByOwnerAsset(ctx)
	if err != nil {
		return nil, err
	}
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

// TestEntryAssetMismatchRejected: trigger menolak asset != account asset.
func TestEntryAssetMismatchRejected(t *testing.T) {
	_, q := p5Setup(t)
	ctx := context.Background()
	acct, err := q.GetLedgerAccountForUpdate(ctx, db.GetLedgerAccountForUpdateParams{Owner: "house:cash", Asset: "IDR"})
	if err != nil {
		// Akun belum ada di DB bersih — buat.
		acct, err = q.CreateLedgerAccount(ctx, db.CreateLedgerAccountParams{Owner: "house:cash", Asset: "IDR"})
		if err != nil {
			t.Fatalf("account: %v", err)
		}
	}
	j, err := q.CreateJournal(ctx, db.CreateJournalParams{Type: "adjust"})
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		JournalID: j.ID, AccountID: acct.ID, Asset: "SOL", Amount: decimal.RequireFromString("1"),
	})
	if err == nil {
		t.Fatal("entry SOL di akun IDR harus DITOLAK trigger")
	}
}
