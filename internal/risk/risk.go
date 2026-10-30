// Package risk: cek eksposur + dua jenis pause (bab 11.2 plan, #16).
// Dipanggil 2x: sebelum quote (boleh kasih quote?) + sebelum eksekusi (masih boleh?).
// Pause exposure = tolak BELI saja. Pause spike_stale = tolak BELI dan JUAL.
// Risk tidak nulis jurnal (baca posisi + tulis RiskEvents saja).
package risk

import (
	"context"
	"errors"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

var (
	ErrBuyPausedExposure = errors.New("beli di-pause: eksposur lewat batas")
	ErrPausedSpikeStale  = errors.New("trading di-pause: harga tidak sehat")
)

type Service struct {
	q   *db.Queries
	set *settings.Service
}

func New(q *db.Queries, set *settings.Service) *Service {
	return &Service{q: q, set: set}
}

// PauseKind: "" | "exposure" | "spike_stale" (dari settings pause_<coin>).
func (s *Service) PauseKind(asset string) string {
	v, _ := s.set.Get("pause_" + strings.ToLower(asset))
	return v
}

// CheckBeforeQuote: boleh kasih quote untuk side ini?
func (s *Service) CheckBeforeQuote(ctx context.Context, asset, side string) error {
	kind := s.PauseKind(asset)
	if kind == "spike_stale" {
		return ErrPausedSpikeStale
	}
	if kind == "exposure" && side == "buy" {
		return ErrBuyPausedExposure
	}
	// Eksposur otomatis: net position vs limit (hanya untuk beli).
	if side == "buy" {
		over, err := s.overExposure(ctx, asset)
		if err != nil {
			return err
		}
		if over {
			return ErrBuyPausedExposure
		}
	}
	return nil
}

// CheckBeforeExecute: cek ulang sebelum eksekusi (30 detik bisa berubah).
func (s *Service) CheckBeforeExecute(ctx context.Context, asset, side string) error {
	return s.CheckBeforeQuote(ctx, asset, side)
}

// overExposure: net < -limit → true. Net = terjual − terbeli (units house).
// FIX debug #5: hanya net NEGATIF (house short, coin keluar ke user) yang dipause.
// Net positif (house long karena user jual) = aman, bahkan mengurangi eksposur.
func (s *Service) overExposure(ctx context.Context, asset string) (bool, error) {
	rows, err := s.q.NetPositionByAsset(ctx)
	if err != nil {
		return false, err
	}
	var net decimal.Decimal
	for _, r := range rows {
		if strings.ToUpper(r.Asset) == strings.ToUpper(asset) {
			net = r.Net
		}
	}
	limit := s.set.Decimal("exposure_limit_" + strings.ToLower(asset))
	return net.LessThan(limit.Neg()), nil
}

// RecordEvent: tulis RiskEvents (Stage 1: catat saja, belum hukum).
func (s *Service) RecordEvent(ctx context.Context, userID int64, hasUser bool, telegramID int64, kind, payload string) error {
	var uid pgtype.Int8
	if hasUser {
		uid = pgtype.Int8{Int64: userID, Valid: true}
	}
	_, err := s.q.InsertRiskEvent(ctx, db.InsertRiskEventParams{
		UserID:     uid,
		TelegramID: telegramID,
		Kind:       kind,
		Payload:    payload,
	})
	return err
}
