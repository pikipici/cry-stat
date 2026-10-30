package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// fetchManualFX: kurs USDT/IDR sementara dari Indodax public ticker (tanpa key).
// FIX debug #14: sebelumnya kurs cuma SetFX sekali → basi permanen setelah 60 detik.
// Fungsi ini dipanggil tiap 30 detik oleh refresher di main.
// Nama sumber final fix di Stage 0 (S0.2) — ganti isi fungsi ini tanpa ubah pemanggil.
func fetchManualFX(ctx context.Context) (decimal.Decimal, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://indodax.com/api/usdt_idr/ticker", nil)
	if err != nil {
		return decimal.Zero, "", err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return decimal.Zero, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return decimal.Zero, "", err
	}
	var t struct {
		Ticker struct {
			Last string `json:"last"`
		} `json:"ticker"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return decimal.Zero, "", fmt.Errorf("parse ticker: %w", err)
	}
	rate, err := decimal.NewFromString(t.Ticker.Last)
	if err != nil || rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, "", fmt.Errorf("rate tidak valid: %q", t.Ticker.Last)
	}
	return rate, "indodax-usdt_idr", nil
}
