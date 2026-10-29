package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/pikipici/crypto-station/internal/bot"
	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/settings"

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
	deps := &bot.Deps{Q: q, Set: set, Price: priceSvc, Ledger: ledgerSvc, Mode: "sandbox"}

	slog.Info("crypto-station fase 1 start", "mode", "sandbox")

	if botToken == "" {
		// Mode cek DB: pastikan settings kebaca, exit 0.
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from settings`).Scan(&n); err != nil {
			slog.Error("baca settings gagal", "err", err)
			os.Exit(1)
		}
		slog.Info("BOT_TOKEN kosong — mode cek DB saja", "settings_rows", n)
		return
	}

	// WS Binance jalan di background (best effort; gagal = harga basi, fail-closed).
	// Kurs awal placeholder manual — nama sumber fix di Stage 0 (S0.2).
	priceSvc.SetFX(decimal.NewFromInt(16000), "manual-fase1")
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
