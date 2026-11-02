package adminwallet

import (
	"context"
	"fmt"
	"sort"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

const (
	KindStockIn       = "STOCK_IN"
	KindPayoutOut     = "PAYOUT_OUT"
	KindUserDepositIn = "USER_DEPOSIT_IN"
)

// ActivityDetail menyimpan data lengkap sebuah aktivitas untuk tampilan detail.
type ActivityDetail struct {
	Item            ActivityItem
	FeeUnits        *decimal.Decimal
	SentUnits       *decimal.Decimal
	CreditedIDR     *int64
	FromAddress     string
	ToAddress       string
	JournalID       *int64
	AdminTelegramID *int64
}

// ListRecent mengambil tiga sumber aktivitas sekaligus, lalu menggabungkannya
// di memori. Tidak ada query tambahan per item.
func (s *Service) ListRecent(ctx context.Context, limit int) ([]ActivityItem, error) {
	limit = activityLimit(limit)
	stocks, err := s.q.ListRecentAdminStockDeposits(ctx, int32(25))
	if err != nil {
		return nil, fmt.Errorf("list deposit stok: %w", err)
	}
	payouts, err := s.q.ListRecentPayoutsAdmin(ctx, int32(25))
	if err != nil {
		return nil, fmt.Errorf("list payout: %w", err)
	}
	deposits, err := s.q.ListRecentUserDepositsAdmin(ctx, int32(25))
	if err != nil {
		return nil, fmt.Errorf("list deposit user: %w", err)
	}

	items := make([]ActivityItem, 0, len(stocks)+len(payouts)+len(deposits))
	for _, row := range stocks {
		items = append(items, ActivityItem{Kind: KindStockIn, ID: row.ID, Asset: row.Asset,
			Network: row.Network, Status: row.Status, TxRef: row.TxRef, Units: row.Units, CreatedAt: row.CreatedAt})
	}
	for _, row := range payouts {
		items = append(items, ActivityItem{Kind: KindPayoutOut, ID: row.ID, Asset: row.Asset,
			Network: row.Network, Status: row.Status, TxRef: textValue(row.TxHash), Units: row.Units,
			UserID: int64Ptr(row.UserID), CreatedAt: row.CreatedAt})
	}
	for _, row := range deposits {
		items = append(items, ActivityItem{Kind: KindUserDepositIn, ID: row.ID, Asset: row.Asset,
			Network: NetworkFor(row.Asset), Status: row.Status, TxRef: row.TxHash, Units: row.Units,
			UserID: nullableInt64Ptr(row.UserID), CreatedAt: row.CreatedAt})
	}

	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		if activityRank(items[i].Kind) != activityRank(items[j].Kind) {
			return activityRank(items[i].Kind) < activityRank(items[j].Kind)
		}
		return items[i].ID > items[j].ID
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// GetDetail mengambil detail satu aktivitas berdasarkan kind dan ID.
func (s *Service) GetDetail(ctx context.Context, kind string, id int64) (ActivityDetail, error) {
	if id <= 0 {
		return ActivityDetail{}, fmt.Errorf("id aktivitas harus positif")
	}
	switch kind {
	case KindStockIn:
		row, err := s.q.GetAdminStockDepositByID(ctx, id)
		if err != nil {
			return ActivityDetail{}, fmt.Errorf("detail deposit stok: %w", err)
		}
		return ActivityDetail{Item: ActivityItem{Kind: kind, ID: row.ID, Asset: row.Asset, Network: row.Network,
			Status: row.Status, TxRef: row.TxRef, Units: row.Units, CreatedAt: row.CreatedAt},
			ToAddress: row.ToAddress, JournalID: nullableInt64Ptr(row.JournalID), AdminTelegramID: int64Ptr(row.AdminTelegramID)}, nil
	case KindPayoutOut:
		row, err := s.q.GetPayoutAdminByID(ctx, id)
		if err != nil {
			return ActivityDetail{}, fmt.Errorf("detail payout: %w", err)
		}
		fee, sent := row.FeeUnits, row.SentUnits
		return ActivityDetail{Item: ActivityItem{Kind: kind, ID: row.ID, Asset: row.Asset, Network: row.Network,
			Status: row.Status, TxRef: textValue(row.TxHash), Units: row.Units, UserID: int64Ptr(row.UserID), CreatedAt: row.CreatedAt},
			FeeUnits: &fee, SentUnits: &sent, ToAddress: row.ToAddress}, nil
	case KindUserDepositIn:
		row, err := s.q.GetUserDepositAdminByID(ctx, id)
		if err != nil {
			return ActivityDetail{}, fmt.Errorf("detail deposit user: %w", err)
		}
		credited := row.CreditedIdr
		return ActivityDetail{Item: ActivityItem{Kind: kind, ID: row.ID, Asset: row.Asset, Network: NetworkFor(row.Asset),
			Status: row.Status, TxRef: row.TxHash, Units: row.Units, UserID: nullableInt64Ptr(row.UserID), CreatedAt: row.CreatedAt},
			CreditedIDR: &credited, FromAddress: row.FromAddress}, nil
	default:
		return ActivityDetail{}, fmt.Errorf("kind aktivitas tidak dikenal: %q", kind)
	}
}

func activityLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 25 {
		return 25
	}
	return limit
}

func activityRank(kind string) int {
	switch kind {
	case KindStockIn:
		return 0
	case KindPayoutOut:
		return 1
	default:
		return 2
	}
}

func textValue(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func int64Ptr(value int64) *int64 { return &value }

func nullableInt64Ptr(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	return int64Ptr(value.Int64)
}
