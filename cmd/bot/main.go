package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	databaseURL := mustEnv("DATABASE_URL")
	botToken := os.Getenv("BOT_TOKEN") // boleh kosong di Fase 0 (mode no-bot, cek DB doang)

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
	// Fase 0: pastikan settings ke-seed (migrasi goose jalan terpisah)
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from settings`).Scan(&n); err != nil {
		slog.Error("baca settings gagal (migrasi belum jalan?)", "err", err)
		os.Exit(1)
	}
	slog.Info("crypto-station fase 0 start", "settings_rows", n)

	if botToken == "" {
		slog.Info("BOT_TOKEN kosong — mode cek DB saja, exit 0")
		return
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, upd *models.Update) {
			if upd.Message == nil {
				return
			}
			// Fase 0: private-chat only (bab 11.5). Group/channel diabaikan.
			if upd.Message.Chat.Type != "private" {
				return
			}
			switch upd.Message.Text {
			case "/start":
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: upd.Message.Chat.ID,
					Text:   "Selamat datang di Crypto Station (SIMULASI).\nPakai /saldo untuk cek saldo sandbox kamu.",
				})
			case "/saldo":
				var balance int64 = 1000000 // Fase 0: placeholder, ledger asli Fase 1
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: upd.Message.Chat.ID,
					Text:   "Saldo sandbox (SIMULASI): Rp1.000.000\n_ ledger asli masuk Fase 1 _",
				})
				_ = balance
			}
		}),
	}
	// Long-poll buat dev (bab 11: webhook prod nanti). Worker default 1, biarkan 1.
	b, err := bot.New(botToken, opts...)
	if err != nil {
		slog.Error("init bot gagal", "err", err)
		os.Exit(1)
	}
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
