// Package adminwallet menyediakan data operasional wallet untuk layar admin.
package adminwallet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/shopspring/decimal"
)

// AssetSummary adalah ringkasan saldo jurnal dan estimasi nilai satu aset.
type AssetSummary struct {
	Asset          string
	Network        string
	Balance        decimal.Decimal
	Minimum        decimal.Decimal
	HotAddress     string
	Status         string
	EstimatedIDR   int64
	EstimateOK     bool
	EstimateReason string
	PriceAgeMs     int64
}

// ActivityItem adalah item ringkas pada feed aktivitas wallet admin.
type ActivityItem struct {
	Kind      string
	ID        int64
	Asset     string
	Network   string
	Status    string
	TxRef     string
	Units     decimal.Decimal
	UserID    *int64
	CreatedAt time.Time
}

// Service menyatukan pembacaan wallet admin tanpa menyimpan state sendiri.
type Service struct {
	q      *db.Queries
	set    *settings.Service
	ledger *ledger.Service
	price  *price.Service
}

// New membuat service wallet admin.
func New(q *db.Queries, set *settings.Service, ledgerSvc *ledger.Service, priceSvc *price.Service) *Service {
	return &Service{q: q, set: set, ledger: ledgerSvc, price: priceSvc}
}

// NetworkFor mengembalikan nama network baku untuk aset yang didukung.
func NetworkFor(asset string) string {
	switch strings.ToUpper(asset) {
	case "BNB":
		return "BSC"
	case "ETH":
		return "ERC20"
	case "SOL":
		return "SOL"
	default:
		return ""
	}
}

// EstimateIDR menghitung nilai IDR dengan pembulatan rupiah terdekat.
func EstimateIDR(balance, bid decimal.Decimal) int64 {
	return balance.Mul(bid).Round(0).IntPart()
}

// AssetSummaries selalu mengembalikan BNB, ETH, dan SOL dalam urutan tetap.
// Kegagalan pembacaan satu aset atau harga dicatat pada aset tersebut agar aset
// lain tetap dapat ditampilkan. Harga tidak pernah memakai fallback lama.
func (s *Service) AssetSummaries(ctx context.Context) ([]AssetSummary, error) {
	assets := []string{"BNB", "ETH", "SOL"}
	out := make([]AssetSummary, 0, len(assets))
	for _, asset := range assets {
		out = append(out, s.assetSummary(ctx, asset))
	}
	return out, nil
}

func (s *Service) assetSummary(ctx context.Context, asset string) AssetSummary {
	summary := AssetSummary{
		Asset:      asset,
		Network:    NetworkFor(asset),
		HotAddress: settingString(s.set, "hot_address_"+strings.ToLower(asset)),
	}

	bal, min, err := s.ledger.FloatStatus(ctx, asset)
	if err != nil {
		summary.EstimateReason = fmt.Sprintf("baca saldo float: %v", err)
		summary.Status = "TIPIS"
		return summary
	}
	summary.Balance, summary.Minimum = bal, min
	if summary.Balance.GreaterThanOrEqual(summary.Minimum) {
		summary.Status = "AMAN"
	} else {
		summary.Status = "TIPIS"
	}

	snap, err := s.price.Snapshot(asset)
	if err != nil {
		summary.EstimateReason = appendReason(summary.EstimateReason, err.Error())
		return summary
	}
	summary.EstimatedIDR = EstimateIDR(summary.Balance, snap.Bid)
	summary.EstimateOK = true
	summary.PriceAgeMs = snap.PriceAgeMs
	return summary
}

func settingString(set *settings.Service, key string) string {
	if value, ok := set.Get(key); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func appendReason(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}
