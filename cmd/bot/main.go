package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/pikipici/crypto-station/internal/bot"
	"github.com/pikipici/crypto-station/internal/admin"
	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/risk"
	"github.com/pikipici/crypto-station/internal/settings"
	"github.com/pikipici/crypto-station/internal/trial"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	databaseURL := mustEnv("DATABASE_URL")
	botToken := os.Getenv("BOT_TOKEN") // kosong = mode cek DB saja

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		slog.Error("buka db gagal", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("ping db gagal", "err", err)
		os.Exit(1)
	}

	q := db.New(pool)
	set := settings.New(q)
	if err := set.Reload(ctx); err != nil {
		slog.Error("load settings gagal", "err", err)
		os.Exit(1)
	}

	priceSvc := price.New(set, q)
	ledgerSvc := ledger.New(pool, set)
	riskSvc := risk.New(q, set)
	adminSvc := admin.New(q, set)
	trialSvc := trial.New(q, set, ledgerSvc)
	limiter := bot.NewRateLimiter(int(set.Int64("rate_limit_per_min")))
	deps := &bot.Deps{Q: q, Set: set, Price: priceSvc, Ledger: ledgerSvc, Risk: riskSvc, Admin: adminSvc, Trial: trialSvc, Limiter: limiter, Mode: "sandbox"}

	slog.Info("crypto-station fase 4 start", "mode", "sandbox")

	if botToken == "" {
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from settings`).Scan(&n); err != nil {
			slog.Error("baca settings gagal", "err", err)
			os.Exit(1)
		}
		slog.Info("BOT_TOKEN kosong — mode cek DB saja", "settings_rows", n)
		return
	}

	// WS Binance background (gagal = harga basi, fail-closed).
	go func() {
		for {
			err := priceSvc.RunWS(ctx, price.CombinedStreamURL())
			slog.Warn("ws binance putus (harga jadi basi)", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()

	// FIX debug #14 (blocker trial live): kurs USDT/IDR harus di-refresh berkala.
	// Sebelumnya SetFX cuma sekali di start → setelah 60 detik SEMUA quote ditolak
	// ErrStaleFX selamanya. Sekarang refresh tiap 30 detik dari sumber yang sama.
	// Nama sumber fix di Stage 0 (S0.2); fungsi fetch diganti tanpa ubah pemanggil.
	fxFetch := func(ctx context.Context) (decimal.Decimal, string, error) {
		return fetchManualFX(ctx)
	}
	refreshFX := func() {
		rate, src, err := fxFetch(ctx)
		if err != nil {
			slog.Warn("refresh kurs gagal (kurs lama dipakai sampai basi)", "err", err)
			return
		}
		priceSvc.SetFX(rate, src)
	}
	refreshFX()
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				refreshFX()
			}
		}
	}()

	// Job rekonsiliasi harian (Fase 3): tiap 1 jam cek, selisih → pause semua coin.
	// Interval pendek di sandbox biar gampang diobservasi; prod nanti via cron/systemd timer.
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := ledgerSvc.Reconcile(ctx); err != nil {
					slog.Error("rekonsiliasi GAGAL — pause semua coin", "err", err)
					for _, c := range []string{"bnb", "eth", "sol"} {
						// FIX debug #12: adminID 0 = system (bukan admin fiktif).
						// Audit before/after eksplisit agar jejak jelas di admin_audit.
						_, _, _ = adminSvc.Pause(ctx, 0, c, "spike_stale")
					}
				}
			}
		}
	}()

	// Job exposure snapshots (Fase 4): tiap 10 menit tulis posisi house.
	go func() {
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := trialSvc.SnapshotExposure(ctx); err != nil {
					slog.Warn("exposure snapshot gagal", "err", err)
				}
			}
		}
	}()

	b, err := tgbot.New(botToken, tgbot.WithDefaultHandler(
		func(ctx context.Context, b *tgbot.Bot, upd *models.Update) {
			bot.Handle(ctx, b, upd, deps)
		}),
	)
	if err != nil {
		slog.Error("init bot gagal", "err", err)
		os.Exit(1)
	}
	// Long-poll dev. Worker default 1, biarkan 1 (konkurensi uang di Postgres).
	b.Start(ctx)
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		slog.Error("env wajib kosong", "key", k)
		os.Exit(1)
	}
	return v
}
