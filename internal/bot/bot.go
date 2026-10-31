// Package bot: handler per command + middleware. Handler TIPIS (bab 11.5):
// parse input → panggil service → format output. Dilarang SQL langsung, hitung uang, pegang harga mentah.
package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pikipici/crypto-station/internal/admin"
	"github.com/pikipici/crypto-station/internal/chain"
	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/risk"
	"github.com/pikipici/crypto-station/internal/settings"
	"github.com/pikipici/crypto-station/internal/trial"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
)

type Deps struct {
	Q       *db.Queries
	Set     *settings.Service
	Price   *price.Service
	Ledger  *ledger.Service
	Risk    *risk.Service
	Admin   *admin.Service
	Trial   *trial.Service
	Chain   *chain.Service // pintu on-chain Fase 8 (mock di sandbox)
	Limiter *RateLimiter
	Session *Sessions // state multi-step T1-button (nil = dimatikan)
	Mode    string    // sandbox / real (Fase 1-4: selalu sandbox)
}

func Route() []string {
	return []string{"/start", "/saldo", "/harga", "/beli", "/jual", "/alamat", "/cekdeposit", "/status", "/riwayat", "/batal",
		"/admin", "/setspread", "/setlimit", "/pause", "/broadcast", "/trial", "/approve"}
}

// Handle: dispatcher tunggal (dipanggil dari default handler main).
func Handle(ctx context.Context, b *bot.Bot, upd *models.Update, d *Deps) {
	// Callback inline (confirm/cancel quote).
	if upd.CallbackQuery != nil {
		// Rate limit juga untuk callback (anti double-tap spam).
		if !d.Limiter.Allow(upd.CallbackQuery.From.ID) {
			_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: upd.CallbackQuery.ID, Text: "Kebanyakan, tunggu sebentar."})
			return
		}
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
	// Rate limit per akun (angka di settings rate_limit_per_min).
	if !d.Limiter.Allow(upd.Message.From.ID) {
		_ = d.Risk.RecordEvent(ctx, 0, false, upd.Message.From.ID, "rate_limited", upd.Message.Text)
		reply(ctx, b, upd.Message.Chat.ID, "Kebanyakan perintah, tunggu sebentar.")
		return
	}
	text := strings.TrimSpace(upd.Message.Text)
	// T2-button: session admin aktif = tangani dulu (allowlist di dalam).
	// T1-button: kalau lagi nunggu nominal DAN teks bukan tombol menu,
	// perlakukan sebagai input nominal (sebelum parsing slash).
	if d.Session != nil {
		if st := d.Session.Get(upd.Message.From.ID); st.Step != StepIdle && MatchButton(text) == "" && !strings.HasPrefix(strings.ToLower(text), "/") {
			switch st.Step {
			case StepAdminSpread, StepAdminLimit, StepAdminBroadcast:
				if handleAdminInput(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, text, st, d) {
					return
				}
			default:
				handleAmountInput(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, text, st, d)
				return
			}
		}
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return
	}
	cmd := strings.ToLower(fields[0])
	// T1-button: teks menu tanpa slash dipetakan ke perintah yang sama.
	if btn := MatchButton(cmd); btn != "" && !strings.HasPrefix(cmd, "/") {
		switch btn {
		case BtnHarga:
			cmd = "/harga"
		case BtnSaldo:
			cmd = "/saldo"
		case BtnBeli:
			cmd = "/beli"
		case BtnJual:
			cmd = "/jual"
		case BtnRiwayat:
			cmd = "/riwayat"
		case BtnBatal:
			cmd = "/batal"
		case BtnAlamat:
			cmd = "/alamat"
		case BtnStatus:
			cmd = "/status"
		case BtnAdmin:
			cmd = "/admin"
		}
		// Input nyasar saat await nominal tapi user tap menu = reset state + layani menu.
		if d.Session != nil {
			d.Session.Clear(upd.Message.From.ID)
		}
	}
	rest := []string{}
	if len(fields) > 1 {
		rest = fields[1:]
	}
	switch cmd {
	case "/start":
		startText := start(ctx, upd.Message.From.ID, d)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: upd.Message.Chat.ID, Text: startText, ReplyMarkup: MainMenuFor(d.Admin.IsAdmin(upd.Message.From.ID))})
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
	case "/setspread":
		adminSpread(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/setlimit":
		adminLimit(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/pause":
		adminPause(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/broadcast":
		adminBroadcast(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/trial":
		adminTrial(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, d)
	case "/admin":
		adminPanel(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, d)
	case "/alamat":
		alamatCmd(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/status":
		statusCmd(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, d)
	case "/cekdeposit":
		// Fase 9: gabung ke Status (1 pantau semua). Slash tetap jalan buat kompat.
		statusCmd(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, d)
	case "/approve":
		reply(ctx, b, upd.Message.Chat.ID, "Belum ada withdraw di sandbox (Stage 2).")
	default:
		reply(ctx, b, upd.Message.Chat.ID, "Perintah tidak dikenal. Tap tombol di bawah atau coba /start, /saldo, /harga, /beli, /jual, /riwayat.")
	}
}

// --- Admin commands (allowlist saja, selain itu tolak + catat) ---

func needAdmin(ctx context.Context, telegramID int64, action string, d *Deps) bool {
	if d.Admin.IsAdmin(telegramID) {
		return true
	}
	_ = d.Admin.Deny(ctx, telegramID, action)
	_ = d.Risk.RecordEvent(ctx, 0, false, telegramID, "admin_deny", action)
	return false
}

// adminPanel: buka panel inline admin (allowlist saja).
// User biasa tap tombol Admin (kalau tahu teksnya) = "Bukan admin" + deny-audit.
func adminPanel(ctx context.Context, b *bot.Bot, chatID, telegramID int64, d *Deps) {
	if !needAdmin(ctx, telegramID, "panel", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Panel admin (SIMULASI):", ReplyMarkup: AdminPanel()})
}

func adminSpread(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "setspread", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) < 3 {
		reply(ctx, b, chatID, "Format: /setspread SOL buy 0.008")
		return
	}
	before, after, err := d.Admin.SetSpread(ctx, telegramID, rest[0], rest[1], rest[2])
	if err != nil {
		reply(ctx, b, chatID, "Gagal: "+err.Error())
		return
	}
	reply(ctx, b, chatID, "Spread "+rest[0]+" "+rest[1]+": "+before+" → "+after)
}

func adminLimit(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "setlimit", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) < 2 {
		reply(ctx, b, chatID, "Format: /setlimit min_buy_idr 25000")
		return
	}
	before, after, err := d.Admin.SetLimit(ctx, telegramID, rest[0], rest[1])
	if err != nil {
		reply(ctx, b, chatID, "Gagal: "+err.Error())
		return
	}
	// FIX debug #2: sinkronkan limiter in-memory — /setlimit rate_limit_per_min
	// sebelumnya cuma ubah cache settings, limiter tetap pakai angka lama sampai restart.
	if rest[0] == "rate_limit_per_min" {
		if n, err := strconv.ParseInt(after, 10, 64); err == nil && n > 0 {
			d.Limiter.SetLimit(int(n))
		}
	}
	reply(ctx, b, chatID, rest[0]+": "+before+" → "+after)
}

func adminPause(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "pause", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) < 1 {
		reply(ctx, b, chatID, "Format: /pause SOL exposure | /pause SOL spike_stale | /pause SOL off")
		return
	}
	kind := "exposure"
	if len(rest) > 1 {
		kind = rest[1]
	}
	before, after, err := d.Admin.Pause(ctx, telegramID, rest[0], kind)
	if err != nil {
		reply(ctx, b, chatID, "Gagal: "+err.Error())
		return
	}
	reply(ctx, b, chatID, "Pause "+rest[0]+": "+before+" → "+after)
}

func adminTrial(ctx context.Context, b *bot.Bot, chatID, telegramID int64, d *Deps) {
	if !needAdmin(ctx, telegramID, "trial", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	reply(ctx, b, chatID, d.Trial.Report(ctx)+"\n\n"+d.Trial.SlippageReport(ctx, 10))
}

func adminBroadcast(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "broadcast", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) == 0 {
		reply(ctx, b, chatID, "Format: /broadcast pesan maintenance ...")
		return
	}
	reply(ctx, b, chatID, doBroadcast(ctx, b, telegramID, strings.Join(rest, " "), d))
}

// doBroadcast: kirim ke semua user aktif + audit. Dipakai slash + button.
func doBroadcast(ctx context.Context, b *bot.Bot, telegramID int64, text string, d *Deps) string {
	msg := "[Crypto Station] " + text
	ids, err := d.Q.ListActiveTelegramIDs(ctx)
	if err != nil {
		return "Gagal ambil daftar user: " + err.Error()
	}
	sent, failed := 0, 0
	// FIX debug #15: jeda 40ms per pesan (~25 msg/detik, di bawah flood limit ~30/detik).
	// Tanpa jeda, broadcast ke ratusan user kena 429 dan sisanya gagal.
	for _, id := range ids {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: id, Text: msg})
		if err != nil {
			failed++
		} else {
			sent++
		}
		time.Sleep(40 * time.Millisecond)
	}
	_ = d.Admin.Audit(ctx, telegramID, "broadcast", 0, false, fmt.Sprintf("target=%d", len(ids)), fmt.Sprintf("sent=%d failed=%d", sent, failed))
	return fmt.Sprintf("Broadcast: %d terkirim, %d gagal (dari %d user).", sent, failed, len(ids))
}

func reply(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
}

// replyMenu: sama kayak reply tapi sekalian pasang keyboard permanen T1-button.
func replyMenu(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: MainMenu()})
}

// sendCoinPicker: kirim inline pilih coin untuk side buy/sell.
func sendCoinPicker(ctx context.Context, b *bot.Bot, chatID int64, side string) {
	verb := "Beli"
	if side == "sell" {
		verb = "Jual"
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID, Text: verb + " coin apa?",
		ReplyMarkup: CoinPicker(side),
	})
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
		// FIX debug #16: race dua /start paralel → salah satu kena unique violation.
		// Ambil existing (dibuat request lain) lalu lanjut, jangan "gagal bikin akun".
		nu, err := d.Q.CreateUser(ctx, db.CreateUserParams{TelegramID: telegramID, Mode: "sandbox"})
		if err != nil {
			u2, err2 := d.Q.GetUserByTelegramID(ctx, telegramID)
			if err2 != nil {
				return "Gagal bikin akun, coba /start lagi."
			}
			u = u2
			_ = nu
		} else {
			if err := d.Ledger.GrantSandbox(ctx, nu.ID, d.Set.Int64("sandbox_starting_balance_idr")); err != nil {
				return "Akun jadi, tapi grant saldo gagal. Coba /saldo."
			}
			return "Selamat datang di Crypto Station (SIMULASI).\n" +
				"Modal sandbox Rp1.000.000 sudah masuk.\n" +
				"Tap tombol di bawah: Harga / Saldo / Beli / Jual / Riwayat.\n" +
				"Slash tetap bisa: /harga /saldo /beli SOL 200000 /jual SOL ALL"
		}
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
	// Fase 8 transit: coin tidak ngendap — tampilkan payout terakhir, bukan valuasi.
	prows, err := d.Q.RecentPayoutsByUser(ctx, db.RecentPayoutsByUserParams{UserID: uid, Limit: 3})
	if err == nil && len(prows) > 0 {
		sb.WriteString("Terkirim:\n")
		for _, r := range prows {
			tx := ""
			if r.TxHash.Valid {
				tx = " tx " + r.TxHash.String
			}
			fmt.Fprintf(&sb, "%s %s → %s (%s)%s\n", r.Asset, r.SentUnits.String(), shortAddr(r.ToAddress), r.Status, tx)
		}
	}
	sb.WriteString("Beli = otomatis dikirim ke wallet lu (/alamat buat daftar).")
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
	// T1-button: tap Beli tanpa arg = kirim picker coin (bukan error format).
	if len(rest) == 0 {
		sendCoinPicker(ctx, b, chatID, "buy")
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
	// Fase 8 transit: buy wajib alamat + fee + auto-kirim (bukan masuk saldo).
	quoteFlowBuy(ctx, b, chatID, telegramID, uid, asset, amount, d)
}

func jual(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	// T1-button: tap Jual tanpa arg = kirim picker coin (bukan error format).
	if len(rest) == 0 {
		sendCoinPicker(ctx, b, chatID, "sell")
		return
	}
	// Fase 9: jual slash langsung = arahin ke flow deposit (bandar mati).
	reply(ctx, b, chatID, "Jual sekarang via deposit: tap Jual → pilih coin → kirim ke alamat deposit → tap simulasi masuk.")
}

// sellAllFlow: jual SEMUA units dari saldo (tanpa muter IDR).
func sellAllFlow(ctx context.Context, b *bot.Bot, chatID, telegramID, uid int64, asset string, d *Deps) {
	if err := d.Risk.CheckBeforeQuote(ctx, asset, "sell"); err != nil {
		_ = d.Risk.RecordEvent(ctx, uid, true, telegramID, "quote_rejected", "sell "+asset+": "+err.Error())
		reply(ctx, b, chatID, "Tidak bisa kasih quote: "+riskMsg(err))
		return
	}
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
	qr, err := d.Ledger.CreateQuoteUnits(ctx, uid, asset, units, snap)
	if err != nil {
		reply(ctx, b, chatID, "Gagal bikin quote: "+err.Error())
		return
	}
	lockSecs := d.Set.Int64("quote_lock_seconds")
	text := fmt.Sprintf("Jual %s Rp%s\nDapat: %s %s\nHarga lock %d detik. Confirm?", asset, formatIDR(decimal.NewFromInt(qr.AmountIDR)), qr.Units.String(), asset, lockSecs)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: QuoteKeyboard(qr.ID)})
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
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: QuoteKeyboard(qr.ID)})
}

func handleCallback(ctx context.Context, b *bot.Bot, cb *models.CallbackQuery, d *Deps) {
	// Jawab callback dulu biar spinner hilang.
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cb.ID})
	data := cb.Data
	// T1-button: callback flow t:* ditangani terpisah (pilih coin / sellall / cancel).
	if strings.HasPrefix(data, "t:") {
		handleFlowCallback(ctx, b, cb, d)
		return
	}
	// T2-button: callback admin a:* (allowlist dicek di dalam).
	if strings.HasPrefix(data, "a:") {
		handleAdminCallback(ctx, b, cb, d)
		return
	}
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
	// Fase 8 transit: buy sukses → payout mock otomatis (txhash MOCK-xxx).
	if tr.Side == "buy" {
		tx, perr := d.Chain.Payout(ctx, uid, tr.TradeID, tr.Asset, tr.Units, tr.FeeUnits, tr.ToAddress)
		if perr != nil {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID,
				Text: fmt.Sprintf("Beli tercatat tapi kirim gagal: %s\nIDR tidak hilang — hubungi admin.", perr.Error())})
			return
		}
		net, _ := chain.NetworkFor(tr.Asset)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID,
			Text: fmt.Sprintf("Terkirim (SIMULASI): %s %s → %s (%s)\ntx %s\nCek /status, riwayat di /riwayat.",
				tr.SentUnits.String(), tr.Asset, shortAddr(tr.ToAddress), net, tx)})
		return
	}
	verb := "Beli"
	if tr.Side == "sell" {
		verb = "Jual"
	}
	text := successHint(verb, tr.Units.String(), tr.Asset, tr.AmountIDR)
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
		return "Belum ada transaksi. Tap Beli buat mulai (contoh: beli SOL 50000)"
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
	if d.Session != nil {
		d.Session.Clear(telegramID)
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
