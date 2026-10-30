// Package bot: handler per command + middleware. Handler TIPIS (bab 11.5):
// parse input → panggil service → format output. Dilarang SQL langsung, hitung uang, pegang harga mentah.
package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/risk"
	"github.com/pikipici/crypto-station/internal/settings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
)

type Deps struct {
	Q      *db.Queries
	Set    *settings.Service
	Price  *price.Service
	Ledger *ledger.Service
	Risk   *risk.Service
	Mode   string // sandbox / real (Fase 1-2: selalu sandbox)
}

func Route() []string {
	return []string{"/start", "/saldo", "/harga", "/beli", "/jual", "/riwayat", "/batal"}
}

// Handle: dispatcher tunggal (dipanggil dari default handler main).
func Handle(ctx context.Context, b *bot.Bot, upd *models.Update, d *Deps) {
	// Callback inline (confirm/cancel quote).
	if upd.CallbackQuery != nil {
		handleCallback(ctx, b, upd.CallbackQuery, d)
		return
	}
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
	rest := []string{}
	if len(fields) > 1 {
		rest = fields[1:]
	}
	switch cmd {
	case "/start":
		reply(ctx, b, upd.Message.Chat.ID, start(ctx, upd.Message.From.ID, d))
	case "/saldo":
		reply(ctx, b, upd.Message.Chat.ID, saldo(ctx, upd.Message.From.ID, d))
	case "/harga":
		arg := ""
		if len(rest) > 0 {
			arg = strings.ToUpper(rest[0])
		}
		reply(ctx, b, upd.Message.Chat.ID, harga(ctx, arg, d))
	case "/beli":
		beli(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/jual":
		jual(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/riwayat":
		reply(ctx, b, upd.Message.Chat.ID, riwayat(ctx, upd.Message.From.ID, d))
	case "/batal":
		reply(ctx, b, upd.Message.Chat.ID, batal(ctx, upd.Message.From.ID, d))
	default:
		reply(ctx, b, upd.Message.Chat.ID, "Perintah tidak dikenal. Coba /start, /saldo, /harga, /beli, /jual, /riwayat.")
	}
}

func reply(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
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
			"Pakai /harga untuk cek harga, /saldo untuk cek saldo.\n" +
			"Beli: /beli SOL 200000 — Jual: /jual SOL 100000 atau /jual SOL ALL"
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

// parseTradeArgs: /beli SOL 200000 atau /jual SOL ALL. ALL hanya untuk jual.
func parseTradeArgs(rest []string) (asset string, amountIDR int64, all bool, errText string) {
	if len(rest) < 2 {
		return "", 0, false, "Format: /beli SOL 200000 atau /jual SOL 100000 (jual bisa ALL). Contoh: /beli SOL 200000"
	}
	asset = strings.ToUpper(strings.TrimSpace(rest[0]))
	if !price.ValidAsset(asset) {
		return "", 0, false, "Coin tidak dikenal. Pilihan: BNB, ETH, SOL."
	}
	raw := strings.ToUpper(strings.TrimSpace(rest[1]))
	raw = strings.ReplaceAll(raw, ".", "")
	raw = strings.ReplaceAll(raw, ",", "")
	if raw == "ALL" || raw == "MAX" {
		return asset, 0, true, ""
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return "", 0, false, "Nominal harus angka rupiah positif. Contoh: /beli SOL 200000"
	}
	return asset, n, false, ""
}

func needUser(ctx context.Context, telegramID int64, d *Deps) (int64, string) {
	uid, blocked := userID(ctx, d, telegramID)
	if blocked == "blocked" {
		return 0, "Akun diblokir. Hubungi admin."
	}
	if uid == 0 {
		return 0, "Belum terdaftar. Kirim /start dulu."
	}
	return uid, ""
}

func beli(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	asset, amount, all, errText := parseTradeArgs(rest)
	if errText != "" {
		reply(ctx, b, chatID, errText)
		return
	}
	if all {
		reply(ctx, b, chatID, "ALL hanya untuk jual. Beli pakai nominal: /beli SOL 200000")
		return
	}
	quoteFlow(ctx, b, chatID, telegramID, uid, "buy", asset, amount, d)
}

func jual(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	asset, amount, all, errText := parseTradeArgs(rest)
	if errText != "" {
		reply(ctx, b, chatID, errText)
		return
	}
	if all {
		// ALL = seluruh valuasi coin ke IDR pakai bid saat ini.
		bal, err := d.Ledger.Balances(ctx, uid)
		if err != nil {
			reply(ctx, b, chatID, "Gagal baca saldo, coba lagi.")
			return
		}
		units := bal[asset]
		if units.IsZero() || units.IsNegative() {
			reply(ctx, b, chatID, "Tidak ada "+asset+" untuk dijual.")
			return
		}
		snap, err := d.Price.Snapshot(asset)
		if err != nil {
			reply(ctx, b, chatID, "Harga basi ("+errMsg(err)+"), coba lagi.")
			return
		}
		amount = units.Mul(snap.Bid).Round(0).IntPart()
		if amount <= 0 {
			reply(ctx, b, chatID, "Nilai terlalu kecil untuk dijual.")
			return
		}
	}
	quoteFlow(ctx, b, chatID, telegramID, uid, "sell", asset, amount, d)
}

// quoteFlow: risk-check → snapshot fail-closed → simpan quote → inline confirm/cancel.
func quoteFlow(ctx context.Context, b *bot.Bot, chatID, telegramID, uid int64, side, asset string, amount int64, d *Deps) {
	if err := d.Risk.CheckBeforeQuote(ctx, asset, side); err != nil {
		_ = d.Risk.RecordEvent(ctx, uid, true, telegramID, "quote_rejected", side+" "+asset+": "+err.Error())
		reply(ctx, b, chatID, "Tidak bisa kasih quote: "+riskMsg(err))
		return
	}
	snap, err := d.Price.Snapshot(asset)
	if err != nil {
		reply(ctx, b, chatID, "Harga basi ("+errMsg(err)+"), coba lagi.")
		return
	}
	qr, err := d.Ledger.CreateQuote(ctx, uid, side, asset, amount, snap)
	if err != nil {
		reply(ctx, b, chatID, "Gagal bikin quote: "+err.Error())
		return
	}
	verb := "Beli"
	if side == "sell" {
		verb = "Jual"
	}
	lockSecs := d.Set.Int64("quote_lock_seconds")
	text := fmt.Sprintf("%s %s Rp%s\nDapat: %s %s\nHarga lock %d detik. Confirm?", verb, asset, formatIDR(decimal.NewFromInt(amount)), qr.Units.String(), asset, lockSecs)
	kb := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Confirm", CallbackData: fmt.Sprintf("q:%d:ok", qr.ID)},
			{Text: "Batal", CallbackData: fmt.Sprintf("q:%d:no", qr.ID)},
		},
	}}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: kb})
}

func handleCallback(ctx context.Context, b *bot.Bot, cb *models.CallbackQuery, d *Deps) {
	// Jawab callback dulu biar spinner hilang.
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cb.ID})
	data := cb.Data
	var qid int64
	var ok bool
	if _, err := fmt.Sscanf(data, "q:%d:ok", &qid); err == nil {
		ok = true
	} else if _, err := fmt.Sscanf(data, "q:%d:no", &qid); err == nil {
		ok = false
	} else {
		return
	}
	uid, blocked := userID(ctx, d, cb.From.ID)
	if blocked == "blocked" || uid == 0 {
		return
	}
	chatID := cb.Message.Message.Chat.ID
	msgID := cb.Message.Message.ID
	if !ok {
		// FIX debug #6: Batal via tombol WAJIB expire quote itu (sebelumnya cuma edit pesan,
		// quote tetap active 30 detik — inkonsisten dengan /batal).
		_ = d.Q.ExpireUserQuotes(ctx, uid)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Quote dibatalkan."})
		return
	}
	// Confirm → cek risk ulang (30 detik bisa berubah) → eksekusi atomik.
	qrow, err := d.Q.GetQuote(ctx, qid)
	if err != nil {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Quote tidak ketemu. Minta requote: ulangi /beli atau /jual."})
		return
	}
	if err := d.Risk.CheckBeforeExecute(ctx, qrow.Asset, qrow.Side); err != nil {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Gagal: " + riskMsg(err)})
		return
	}
	tr, err := d.Ledger.ExecuteTrade(ctx, uid, qid)
	if err != nil {
		msg := tradeErrMsg(err)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: msg})
		return
	}
	verb := "Beli"
	if tr.Side == "sell" {
		verb = "Jual"
	}
	text := fmt.Sprintf("%s sukses (SIMULASI): %s %s = Rp%s.\nCek /saldo, riwayat di /riwayat.", verb, tr.Units.String(), tr.Asset, formatIDR(decimal.NewFromInt(tr.AmountIDR)))
	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: text})
}

func riwayat(ctx context.Context, telegramID int64, d *Deps) string {
	uid, blocked := userID(ctx, d, telegramID)
	if blocked == "blocked" {
		return "Akun diblokir. Hubungi admin."
	}
	if uid == 0 {
		return "Belum terdaftar. Kirim /start dulu."
	}
	rows, err := d.Q.RecentTradesByUser(ctx, db.RecentTradesByUserParams{UserID: uid, Limit: 10})
	if err != nil {
		return "Gagal baca riwayat, coba lagi."
	}
	if len(rows) == 0 {
		return "Belum ada transaksi. Mulai dengan /beli SOL 50000"
	}
	var sb strings.Builder
	sb.WriteString("10 transaksi terakhir (SIMULASI):\n")
	for _, r := range rows {
		verb := "Beli"
		if r.Side == "sell" {
			verb = "Jual"
		}
		fmt.Fprintf(&sb, "%s %s Rp%s (%s)\n", verb, r.Asset, formatIDR(decimal.NewFromInt(r.AmountIdr)), r.Units.String())
	}
	return strings.TrimRight(sb.String(), "\n")
}

func batal(ctx context.Context, telegramID int64, d *Deps) string {
	uid, blocked := userID(ctx, d, telegramID)
	if blocked == "blocked" {
		return "Akun diblokir. Hubungi admin."
	}
	if uid == 0 {
		return "Belum terdaftar. Kirim /start dulu."
	}
	if err := expireActive(ctx, d, uid); err != nil {
		return "Gagal membatalkan, coba lagi."
	}
	return "Quote aktif dibatalkan."
}

// expireActive: tandai semua quote active user jadi expired (best effort, 1 query).
func expireActive(ctx context.Context, d *Deps, uid int64) error {
	return d.Q.ExpireUserQuotes(ctx, uid)
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

func riskMsg(err error) string {
	switch err {
	case risk.ErrBuyPausedExposure:
		return "beli di-pause (eksposur lewat batas). Jual tetap boleh."
	case risk.ErrPausedSpikeStale:
		return "trading di-pause (harga tidak sehat). Coba lagi nanti."
	default:
		return err.Error()
	}
}

func tradeErrMsg(err error) string {
	switch err {
	case ledger.ErrQuoteUsed:
		return "Quote sudah dipakai/expired. Minta requote: ulangi /beli atau /jual."
	case ledger.ErrNoBalance:
		return "Saldo tidak cukup untuk eksekusi."
	case risk.ErrBuyPausedExposure:
		return "Gagal: beli di-pause (eksposur). Jual tetap boleh."
	case risk.ErrPausedSpikeStale:
		return "Gagal: trading di-pause (harga tidak sehat)."
	default:
		return "Gagal eksekusi: " + err.Error()
	}
}

func formatIDR(v decimal.Decimal) string {
	// IDR selalu bulat. Format ribuan sederhana (cukup untuk Fase 2).
	i := v.Round(0).String()
	neg := false
	if strings.HasPrefix(i, "-") {
		neg = true
		i = i[1:]
	}
	n := len(i)
	var out strings.Builder
	for k, c := range i {
		if k > 0 && (n-k)%3 == 0 {
			out.WriteString(".")
		}
		out.WriteRune(c)
	}
	if neg {
		return "-" + out.String()
	}
	return out.String()
}
