// Package chain: SATU-SATUNYA pintu on-chain (plan v0.12 bab 11).
// Fase 8 = mock: ValidateAddress (format), EstimateFee (fee_mock_*),
// Payout (txhash MOCK-xxx, selalu confirmed). Node beneran = Fase 10.
// Modul lain dilarang broadcast langsung.
package chain

import (
	"context"
	"fmt"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	set  *settings.Service
}

func New(pool *pgxpool.Pool, q *db.Queries, set *settings.Service) *Service {
	return &Service{pool: pool, q: q, set: set}
}

// NetworkFor: 1 coin = 1 network tetap (tanpa pilihan di MVP).
func NetworkFor(asset string) (string, bool) {
	switch strings.ToUpper(asset) {
	case "BNB":
		return "BSC", true
	case "ETH":
		return "ERC20", true
	case "SOL":
		return "SOL", true
	}
	return "", false
}

// ValidateAddress: format per network, tanpa node.
func ValidateAddress(asset, addr string) error {
	addr = strings.TrimSpace(addr)
	net, ok := NetworkFor(asset)
	if !ok {
		return fmt.Errorf("coin tidak dikenal")
	}
	switch net {
	case "BSC", "ERC20":
		if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
			return fmt.Errorf("alamat %s harus 0x + 40 hex", net)
		}
		for _, c := range addr[2:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return fmt.Errorf("alamat %s mengandung char bukan hex", net)
			}
		}
		return nil
	case "SOL":
		if len(addr) < 32 || len(addr) > 44 {
			return fmt.Errorf("alamat SOL harus 32–44 char base58")
		}
		for _, c := range addr {
			if !((c >= '1' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
				return fmt.Errorf("alamat SOL mengandung char bukan base58")
			}
		}
		return nil
	}
	return fmt.Errorf("network tidak dikenal")
}

// EstimateFee: Stage 1 = fee_mock_* dari Settings. Stage 2 = estimator node.
func (s *Service) EstimateFee(asset string) (decimal.Decimal, error) {
	asset = strings.ToUpper(asset)
	if _, ok := NetworkFor(asset); !ok {
		return decimal.Zero, fmt.Errorf("coin tidak dikenal")
	}
	return s.set.SafeDecimal("fee_mock_" + strings.ToLower(asset))
}

// FeeRejectPct: ambang tolak fee vs units (default 0.2 = 20%).
func (s *Service) FeeRejectPct() decimal.Decimal {
	if v, err := s.set.SafeDecimal("fee_reject_pct"); err == nil {
		return v
	}
	return decimal.NewFromFloat(0.2)
}

// Payout mock: catat Payouts confirmed + ChainEvents, txhash MOCK-<tradeID>.
// Idempotent by trade_id (panggil 2x = baris sama, bukan dobel kirim).
func (s *Service) Payout(ctx context.Context, userID, tradeID int64, asset string, units, feeUnits decimal.Decimal, toAddr string) (string, error) {
	asset = strings.ToUpper(asset)
	net, ok := NetworkFor(asset)
	if !ok {
		return "", fmt.Errorf("coin tidak dikenal")
	}
	sent := units.Sub(feeUnits)
	if sent.LessThanOrEqual(decimal.Zero) {
		return "", fmt.Errorf("fee menelan seluruh units")
	}
	tx := fmt.Sprintf("MOCK-%d", tradeID)
	// Idempotent: trade sudah punya payout = kembalikan tx lama.
	if p, err := s.q.GetPayoutByTrade(ctx, tradeID); err == nil {
		if p.TxHash.Valid {
			return p.TxHash.String, nil
		}
		return tx, nil
	}
	if _, err := s.q.CreatePayout(ctx, db.CreatePayoutParams{
		UserID: userID, TradeID: tradeID, Asset: asset,
		Units: units, FeeUnits: feeUnits, SentUnits: sent,
		ToAddress: toAddr, Network: net,
		TxHash: pgText(tx), Status: "confirmed", Confirms: 1,
	}); err != nil {
		// Race: orang lain bikin duluan → ambil yang ada.
		if p, err2 := s.q.GetPayoutByTrade(ctx, tradeID); err2 == nil && p.TxHash.Valid {
			return p.TxHash.String, nil
		}
		return "", err
	}
	_, _ = s.q.InsertChainEvent(ctx, db.InsertChainEventParams{
		Asset: asset, TxHash: tx, EventType: "mock_confirmed",
		RawPayload: "sandbox mock, bukan chain beneran", Confirms: 1,
	})
	return tx, nil
}
