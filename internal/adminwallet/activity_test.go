package adminwallet

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/ledger"

	"github.com/shopspring/decimal"
)

func TestActivityListAndStockDetail(t *testing.T) {
	svc, ledgerSvc := walletDeps(t)
	ctx := context.Background()
	ref := fmt.Sprintf("ACTIVITY-STOCK-%d", time.Now().UnixNano())
	res, err := ledgerSvc.RecordAdminStockDeposit(ctx, ledger.RecordStockDepositInput{
		AdminTelegramID: time.Now().UnixNano(), Asset: "SOL", Units: decimal.NewFromInt(3),
		ToAddress: "HOT-ACTIVITY", TxRef: ref,
	})
	if err != nil {
		t.Fatalf("record stock: %v", err)
	}
	items, err := svc.ListRecent(ctx, 25)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	var found *ActivityItem
	for i := range items {
		if items[i].Kind == KindStockIn && items[i].ID == res.DepositID {
			found = &items[i]
			break
		}
	}
	if found == nil || found.TxRef != ref {
		t.Fatalf("stock activity tidak ditemukan: %+v", items)
	}
	detail, err := svc.GetDetail(ctx, KindStockIn, res.DepositID)
	if err != nil {
		t.Fatalf("stock detail: %v", err)
	}
	if detail.JournalID == nil || *detail.JournalID != res.JournalID || detail.AdminTelegramID == nil || detail.ToAddress != "HOT-ACTIVITY" {
		t.Fatalf("detail stock tidak lengkap: %+v", detail)
	}
}

func TestActivityLimitAndOrder(t *testing.T) {
	svc, _ := walletDeps(t)
	items, err := svc.ListRecent(context.Background(), 1)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(items) > 1 {
		t.Fatalf("limit tidak diterapkan: %d", len(items))
	}
	all, err := svc.ListRecent(context.Background(), 25)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	for i := 1; i < len(all); i++ {
		if all[i].CreatedAt.After(all[i-1].CreatedAt) {
			t.Fatalf("feed tidak urut DESC: %+v lalu %+v", all[i-1], all[i])
		}
	}
}

func TestActivityInvalidDetailDoesNotLeak(t *testing.T) {
	svc, _ := walletDeps(t)
	if _, err := svc.GetDetail(context.Background(), "LAIN", 1); err == nil {
		t.Fatal("kind invalid harus gagal")
	}
	if _, err := svc.GetDetail(context.Background(), KindPayoutOut, 0); err == nil {
		t.Fatal("id invalid harus gagal")
	}
}
