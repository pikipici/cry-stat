package adminwallet

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func walletDeps(t *testing.T) (*Service, *ledger.Service) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL kosong — test DB hanya di VPS")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("buka db: %v", err)
	}
	t.Cleanup(pool.Close)
	q := db.New(pool)
	set := settings.New(q)
	if err := set.Reload(context.Background()); err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	ledgerSvc := ledger.New(pool, set)
	return New(q, set, ledgerSvc, price.New(set, q)), ledgerSvc
}

func TestAssetSummariesAssetsOrdered(t *testing.T) {
	svc, _ := walletDeps(t)
	items, err := svc.AssetSummaries(context.Background())
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if len(items) != 3 || items[0].Asset != "BNB" || items[1].Asset != "ETH" || items[2].Asset != "SOL" {
		t.Fatalf("urutan aset: %+v", items)
	}
}

func TestAssetSummaryStockMakesBalanceSafe(t *testing.T) {
	svc, ledgerSvc := walletDeps(t)
	ctx := context.Background()
	if _, err := ledgerSvc.RecordAdminStockDeposit(ctx, ledger.RecordStockDepositInput{
		AdminTelegramID: time.Now().UnixNano(), Asset: "BNB", Units: decimal.NewFromInt(1000000),
		ToAddress: "HOT-TEST", TxRef: fmt.Sprintf("SUMMARY-SAFE-%d", time.Now().UnixNano()),
	}); err != nil {
		t.Fatalf("record stock: %v", err)
	}
	items, err := svc.AssetSummaries(ctx)
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if items[0].Status != "AMAN" {
		t.Fatalf("status BNB=%q, mau AMAN", items[0].Status)
	}
}

func TestEstimateIDR(t *testing.T) {
	got := EstimateIDR(decimal.RequireFromString("1.25"), decimal.NewFromInt(16001))
	if got != 20001 {
		t.Fatalf("estimasi=%d, mau 20001", got)
	}
}

func TestAssetSummaryWithoutPriceFailsClosed(t *testing.T) {
	svc, _ := walletDeps(t)
	items, err := svc.AssetSummaries(context.Background())
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	for _, item := range items {
		if item.EstimateOK || item.EstimatedIDR != 0 || item.EstimateReason == "" {
			t.Fatalf("estimasi harus fail-closed: %+v", item)
		}
	}
}

func TestNetworkFor(t *testing.T) {
	for asset, want := range map[string]string{"BNB": "BSC", "ETH": "ERC20", "SOL": "SOL"} {
		if got := NetworkFor(asset); got != want {
			t.Fatalf("network %s=%q, mau %q", asset, got, want)
		}
	}
}
