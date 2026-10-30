// Package trial: metrics trial sandbox Fase 4 (plan v0.5 Fase 4 + exit criteria bab 3).
// - Exposure snapshots berkala (posisi house vs limit).
// - Slippage report: quote_age per trade + reject counts + exposure terakhir.
// - Exit criteria check: reconcile OK + tidak ada double-spend + pause teruji.
package trial

import (
	"context"
	"fmt"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/shopspring/decimal"
)

type Service struct {
	q      *db.Queries
	set    *settings.Service
	ledger *ledger.Service
}

func New(q *db.Queries, set *settings.Service, ledgerSvc *ledger.Service) *Service {
	return &Service{q: q, set: set, ledger: ledgerSvc}
}

// SnapshotExposure: tulis posisi house per coin + limit_used. Dipanggil job berkala.
func (s *Service) SnapshotExposure(ctx context.Context) error {
	rows, err := s.q.NetPositionByAsset(ctx)
	if err != nil {
		return err
	}
	net := map[string]decimal.Decimal{}
	for _, r := range rows {
		net[strings.ToUpper(r.Asset)] = r.Net
	}
	for _, a := range []string{"BNB", "ETH", "SOL"} {
		limit := s.set.Decimal("exposure_limit_" + strings.ToLower(a))
		n := net[a]
		// FIX debug #6: signed (bukan Abs) — negatif = house short (eksposur),
		// positif = house long (aman). Tampilan Report yang ubah jadi persen.
		used := decimal.Zero
		if !limit.IsZero() {
			used = n.Div(limit)
		}
		if _, err := s.q.InsertExposureSnapshot(ctx, db.InsertExposureSnapshotParams{
			Asset: a, NetPosition: n, LimitUsed: used,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Report: ringkasan trial untuk /trial (admin).
func (s *Service) Report(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("Trial sandbox (SIMULASI):\n")

	// 1. Reconcile.
	if err := s.ledger.Reconcile(ctx); err != nil {
		fmt.Fprintf(&sb, "Reconcile: GAGAL (%v)\n", err)
	} else {
		sb.WriteString("Reconcile: OK (selisih 0)\n")
	}

	// 2. Quotes by status (double-spend = tidak ada quote used 2x; dijamin unique quote_id di trades).
	qs, err := s.q.CountQuotesByStatus(ctx)
	if err == nil {
		for _, r := range qs {
			fmt.Fprintf(&sb, "Quote %s: %d\n", r.Status, r.N)
		}
	}

	// 3. Quote age (proxy slippage): umur quote saat dieksekusi vs lock 30s.
	ages, err := s.q.TradeQuoteAges(ctx, 50)
	if err == nil && len(ages) > 0 {
		var max, sum int64
		warn := s.set.Int64("trial_quote_age_warn_seconds")
		warns := 0
		for _, a := range ages {
			sum += a.QuoteAgeSeconds
			if a.QuoteAgeSeconds > max {
				max = a.QuoteAgeSeconds
			}
			if a.QuoteAgeSeconds > warn {
				warns++
			}
		}
		fmt.Fprintf(&sb, "Trades: %d, avg quote age %ds, max %ds, >%ds: %d\n",
			len(ages), sum/int64(len(ages)), max, warn, warns)
	} else {
		sb.WriteString("Trades: 0 (belum ada)\n")
	}

	// 4. Rejects (quote_rejected, rate_limited, admin_deny).
	rej, err := s.q.CountRiskEventsByKind(ctx)
	if err == nil {
		for _, r := range rej {
			fmt.Fprintf(&sb, "Event %s: %d\n", r.Kind, r.N)
		}
	}

	// 5. Exposure terakhir (signed: negatif = house short = eksposur).
	for _, a := range []string{"BNB", "ETH", "SOL"} {
		if e, err := s.q.LatestExposure(ctx, a); err == nil {
			fmt.Fprintf(&sb, "Exposure %s: %s (limit %s%%)\n", a, e.NetPosition.String(), e.LimitUsed.Mul(decimal.NewFromInt(100)).Round(1).String())
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// SlippageReport: (price_used - mid_used)/mid_used per trade terakhir.
// FIX debug #8: datanya sudah ada di trades (price_used, mid_used) tapi tidak pernah
// dihitung — report cuma tampil quote age (proxy). Sekarang slippage beneran.
func (s *Service) SlippageReport(ctx context.Context, limit int32) string {
	rows, err := s.q.TradeSlippage(ctx, limit)
	if err != nil {
		return "Slippage: gagal baca (" + err.Error() + ")"
	}
	if len(rows) == 0 {
		return "Slippage: belum ada trade"
	}
	var sb strings.Builder
	sb.WriteString("Slippage (price vs mid):\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "#%d %s %s: %s%%\n", r.ID, r.Side, r.Asset,
			r.SlipPct.Mul(decimal.NewFromInt(100)).Round(3).String())
	}
	return strings.TrimRight(sb.String(), "\n")
}
