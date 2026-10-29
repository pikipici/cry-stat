// Package bot: handler per command + middleware. Handler TIPIS (bab 11.5):
// parse input → panggil service → format output. Dilarang SQL langsung, hitung uang, pegang harga mentah.
package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
)

type Deps struct {
	Q       *db.Queries
	Set     *settings.Service
	Price   *price.Service
	Ledger  *ledger.Service
	Mode    string // sandbox / real (Fase 1: selalu sandbox)
}

func Route() []string { return []string{"/start", "/saldo", "/harga"} }

// Handle: dispatcher tunggal (dipanggil dari default handler main).
func Handle(ctx context.Context, b *bot.Bot, upd *models.Update, d *Deps) {
	if upd.Message == nil {
		return
	}
	// Private-chat only (bab 11.5). Group/channel diabaikan diam-diam.
	if upd.Message.Chat.Type != "private" {
		return
	}
	text := strings.TrimSpace(upd.Message.Text)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return
	}
	cmd := strings.ToLower(fields[0])
	args := ""
	if len(fields) > 1 {
		args = strings.ToUpper(strings.TrimSpace(fields[1]))
	}
	var reply string
	switch cmd {
	case "/start":
		reply = start(ctx, upd.Message.From.ID, d)
	case "/saldo":
		reply = saldo(ctx, upd.Message.From.ID, d)
	case "/harga":
		reply = harga(ctx, args, d)
	default:
		reply = "Perintah tidak dikenal. Coba /start, /saldo, /harga."
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: upd.Message.Chat.ID, Text: reply})
}

func userID(ctx context.Context, d *Deps, telegramID int64) (int64, string) {
	u, err := d.Q.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return 0, ""
	}
	if u.Status == "blocked" {
		return 0, "blocked"
	}
	return u.ID, ""
}

func start(ctx context.Context, telegramID int64, d *Deps) string {
	u, err := d.Q.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		// User baru → buat + grant modal sandbox (idempotent di GrantSandbox).
		nu, err := d.Q.CreateUser(ctx, db.CreateUserParams{TelegramID: telegramID, Mode: "sandbox"})
		if err != nil {
			return "Gagal bikin akun, coba /start lagi."
		}
		if err := d.Ledger.GrantSandbox(ctx, nu.ID, d.Set.Int64("sandbox_starting_balance_idr")); err != nil {
			return "Akun jadi, tapi grant saldo gagal. Coba /saldo."
		}
		return "Selamat datang di Crypto Station (SIMULASI).\n" +
			"Modal sandbox Rp1.000.000 sudah masuk.\n" +
			"Pakai /harga untuk cek harga, /saldo untuk cek saldo."
	}
	if st := u.Status; st == "blocked" {
		return "Akun diblokir. Hubungi admin."
	}
	return "Kamu sudah terdaftar (SIMULASI).\nPakai /harga untuk cek harga, /saldo untuk cek saldo."
}

func saldo(ctx context.Context, telegramID int64, d *Deps) string {
	uid, blocked := userID(ctx, d, telegramID)
	if blocked == "blocked" {
		return "Akun diblokir. Hubungi admin."
	}
	if uid == 0 {
		return "Belum terdaftar. Kirim /start dulu."
	}
	bal, err := d.Ledger.Balances(ctx, uid)
	if err != nil {
		return "Gagal baca saldo, coba lagi."
	}
	var sb strings.Builder
	sb.WriteString("Saldo sandbox (SIMULASI):\n")
	sb.WriteString("IDR: Rp" + formatIDR(bal["IDR"]) + "\n")
	for _, a := range price.Assets {
		sb.WriteString(a + ": " + bal[a].String() + "\n")
	}
	// Valuasi pakai bid terakhir yang tidak basi (bab 7.1). Basi = tampil "harga basi".
	total := bal["IDR"]
	for _, a := range price.Assets {
		if bal[a].IsZero() {
			continue
		}
		snap, err := d.Price.Snapshot(a)
		if err != nil {
			fmt.Fprintf(&sb, "(%s harga basi — tidak ikut valuasi)\n", a)
			continue
		}
		total = total.Add(bal[a].Mul(snap.Bid))
	}
	sb.WriteString("Total valuasi: Rp" + formatIDR(total))
	return sb.String()
}

func harga(ctx context.Context, arg string, d *Deps) string {
	list := price.Assets
	if arg != "" {
		if !price.ValidAsset(arg) {
			return "Coin tidak dikenal. Pilihan: BNB, ETH, SOL. Contoh: /harga sol"
		}
		list = []string{arg}
	}
	var sb strings.Builder
	sb.WriteString("Harga (SIMULASI, ask=beli / bid=jual):\n")
	for _, a := range list {
		snap, err := d.Price.Snapshot(a)
		if err != nil {
			fmt.Fprintf(&sb, "%s: harga basi (%s)\n", a, errMsg(err))
			continue
		}
		fmt.Fprintf(&sb, "%s: beli Rp%s / jual Rp%s\n", a, formatIDR(snap.Ask), formatIDR(snap.Bid))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func errMsg(err error) string {
	switch err {
	case price.ErrStaleCoin:
		return "data coin basi"
	case price.ErrStaleFX:
		return "kurs basi"
	case price.ErrSourceDown:
		return "sumber mati"
	case price.ErrAssetPaused:
		return "di-pause"
	default:
		return "tidak tersedia"
	}
}

func formatIDR(v decimal.Decimal) string {
	// IDR selalu bulat. Format ribuan sederhana (cukup untuk Fase 1).
	i := v.Round(0).String()
	n := len(i)
	var out strings.Builder
	for k, c := range i {
		if k > 0 && (n-k)%3 == 0 {
			out.WriteString(".")
		}
		out.WriteRune(c)
	}
	return out.String()
}
