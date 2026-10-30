// Package admin: allowlist + aksi admin + audit before/after (bab 9 + 11.1 plan).
// Allowlist: ADMIN_TELEGRAM_IDS env (koma) + settings admin_allowlist (koma). Gabungan keduanya.
// Perintah admin dari ID luar daftar = tolak + catat ke admin_audit (percobaan).
package admin

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type Service struct {
	q   *db.Queries
	set *settings.Service
}

func New(q *db.Queries, set *settings.Service) *Service {
	return &Service{q: q, set: set}
}

// IsAdmin: cek env + settings. Settings reload tiap panggil? Tidak — pakai cache,
// admin_allowlist di-reload eksplisit setelah /setlimit ubah allowlist (lihat SetLimit).
func (s *Service) IsAdmin(telegramID int64) bool {
	for _, id := range parseIDs(os.Getenv("ADMIN_TELEGRAM_IDS")) {
		if id == telegramID {
			return true
		}
	}
	if v, ok := s.set.Get("admin_allowlist"); ok {
		for _, id := range parseIDs(v) {
			if id == telegramID {
				return true
			}
		}
	}
	return false
}

func parseIDs(s string) []int64 {
	var out []int64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := strconv.ParseInt(p, 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// Deny: catat percobaan akses ilegal (target_id = telegram pelaku).
func (s *Service) Deny(ctx context.Context, telegramID int64, action string) error {
	_, err := s.q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
		AdminTelegramID: telegramID,
		Action:          "deny:" + action,
		TargetID:        pgtype.Int8{Int64: telegramID, Valid: true},
		Before:          "",
		After:           "rejected",
	})
	return err
}

// Audit: catat aksi sukses dengan before/after.
func (s *Service) Audit(ctx context.Context, telegramID int64, action string, target int64, hasTarget bool, before, after string) error {
	var tid pgtype.Int8
	if hasTarget {
		tid = pgtype.Int8{Int64: target, Valid: true}
	}
	_, err := s.q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
		AdminTelegramID: telegramID,
		Action:          action,
		TargetID:        tid,
		Before:          before,
		After:           after,
	})
	return err
}

// SetSpread: ubah spread 1 coin 1 side. Return before/after untuk audit.
func (s *Service) SetSpread(ctx context.Context, adminID int64, coin, side, val string) (before, after string, err error) {
	coin = strings.ToLower(coin)
	side = strings.ToLower(side)
	if coin != "bnb" && coin != "eth" && coin != "sol" {
		return "", "", fmt.Errorf("coin harus bnb/eth/sol")
	}
	if side != "buy" && side != "sell" {
		return "", "", fmt.Errorf("side harus buy/sell")
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if err != nil || f < 0 || f > 0.1 {
		return "", "", fmt.Errorf("spread harus angka 0–0.1 (misal 0.01 = 1%%)")
	}
	// FIX debug #1: normalisasi via decimal (float 0.008 aman, tapi 0.1+epsilon float
	// bisa lolos/tewas di border). Format konsisten tanpa notasi ilmiah.
	after = decimal.NewFromFloat(f).Round(6).String()
	key := "spread_" + side + "_" + coin
	before, _ = s.set.Get(key)
	row, err := s.q.UpsertSetting(ctx, db.UpsertSettingParams{Key: key, Value: after, UpdatedBy: pgtype.Int8{Int64: adminID, Valid: true}})
	if err != nil {
		return "", "", err
	}
	if err := s.set.Reload(ctx); err != nil {
		return "", "", err
	}
	_ = s.Audit(ctx, adminID, "setspread", 0, false, key+"="+before, key+"="+row.Value)
	return before, row.Value, nil
}

// LimitKeys yang boleh diubah via /setlimit (whitelist — bukan semua settings).
var limitKeys = map[string]bool{
	"daily_topup_limit_idr":         true,
	"daily_withdraw_limit_idr":      true,
	"withdraw_approval_threshold_idr": true,
	"min_topup_idr":                 true,
	"min_buy_idr":                   true,
	"min_withdraw_idr":              true,
	"quote_lock_seconds":            true,
	"staleness_coin_ms":             true,
	"staleness_fx_ms":               true,
	"rate_limit_per_min":            true,
	"exposure_limit_bnb":            true,
	"exposure_limit_eth":            true,
	"exposure_limit_sol":            true,
	"admin_allowlist":               true,
}

// SetLimit: ubah angka kunci tanpa restart. Return before/after.
func (s *Service) SetLimit(ctx context.Context, adminID int64, key, val string) (before, after string, err error) {
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	if !limitKeys[key] {
		return "", "", fmt.Errorf("key tidak boleh diubah. Pilihan: daily_topup_limit_idr, daily_withdraw_limit_idr, withdraw_approval_threshold_idr, min_topup_idr, min_buy_idr, min_withdraw_idr, quote_lock_seconds, staleness_coin_ms, staleness_fx_ms, rate_limit_per_min, exposure_limit_bnb/eth/sol, admin_allowlist")
	}
	if key != "admin_allowlist" {
		if _, err := strconv.ParseInt(val, 10, 64); err != nil {
			// exposure_limit_* boleh desimal (units coin).
			if strings.HasPrefix(key, "exposure_limit_") {
				if _, err2 := strconv.ParseFloat(val, 64); err2 != nil {
					return "", "", fmt.Errorf("nilai harus angka")
				}
			} else {
				return "", "", fmt.Errorf("nilai harus angka bulat")
			}
		}
	}
	before, _ = s.set.Get(key)
	row, err := s.q.UpsertSetting(ctx, db.UpsertSettingParams{Key: key, Value: val, UpdatedBy: pgtype.Int8{Int64: adminID, Valid: true}})
	if err != nil {
		return "", "", err
	}
	if err := s.set.Reload(ctx); err != nil {
		return "", "", err
	}
	_ = s.Audit(ctx, adminID, "setlimit", 0, false, key+"="+before, key+"="+row.Value)
	return before, row.Value, nil
}

// Pause kinds: "" (jalan) | "exposure" (beli saja stop) | "spike_stale" (semua stop).
// FIX debug #12: adminID 0 = system (job reconcile) — audit action prefix "system:" agar
// jejak jelas, bukan admin fiktif.
func (s *Service) Pause(ctx context.Context, adminID int64, coin, kind string) (before, after string, err error) {
	coin = strings.ToLower(coin)
	if coin != "bnb" && coin != "eth" && coin != "sol" {
		return "", "", fmt.Errorf("coin harus bnb/eth/sol")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "" && kind != "off" && kind != "exposure" && kind != "spike_stale" {
		return "", "", fmt.Errorf("jenis harus: off (jalan) / exposure (stop beli) / spike_stale (stop semua)")
	}
	if kind == "off" {
		kind = ""
	}
	key := "pause_" + coin
	before, _ = s.set.Get(key)
	row, err := s.q.UpsertSetting(ctx, db.UpsertSettingParams{Key: key, Value: kind, UpdatedBy: pgtype.Int8{Int64: adminID, Valid: true}})
	if err != nil {
		return "", "", err
	}
	if err := s.set.Reload(ctx); err != nil {
		return "", "", err
	}
	action := "pause"
	if adminID == 0 {
		action = "system:pause"
	}
	_ = s.Audit(ctx, adminID, action, 0, false, key+"="+before, key+"="+row.Value)
	return before, row.Value, nil
}
