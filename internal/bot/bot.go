// Package bot: handler per command + middleware. Handler TIPIS (bab 11.5):
// parse input → panggil service → format output. Dilarang SQL langsung, hitung uang, pegang harga mentah.
package bot

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pikipici/crypto-station/internal/admin"
	"github.com/pikipici/crypto-station/internal/adminwallet"
	"github.com/pikipici/crypto-station/internal/chain"
	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"
	"github.com/pikipici/crypto-station/internal/price"
	"github.com/pikipici/crypto-station/internal/risk"
	"github.com/pikipici/crypto-station/internal/settings"
	"github.com/pikipici/crypto-station/internal/trial"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgtype"
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
	Wallet  *adminwallet.Service
	Chain   *chain.Service // pintu on-chain Fase 8 (mock di sandbox)
	Limiter *RateLimiter
	Session *Sessions // state multi-step T1-button (nil = dimatikan)
	Mode    string    // sandbox / real (Fase 1-4: selalu sandbox)

	// Phase 8 (A6): dedup broadcast double-tap/double-slash.
	// Kunci: teks+sender sama dalam 60 detik = tolak kirim kedua.
	BroadcastMu   sync.Mutex
	BroadcastLast map[string]time.Time
}

// claimBroadcastSend: klaim sekali pakai per key dalam 60 detik.
// Pure map+mutex — unit-testable tanpa DB/bot.
func (d *Deps) claimBroadcastSend(key string) bool {
	d.BroadcastMu.Lock()
	defer d.BroadcastMu.Unlock()
	if d.BroadcastLast == nil {
		d.BroadcastLast = map[string]time.Time{}
	}
	if last, ok := d.BroadcastLast[key]; ok && time.Since(last) < 60*time.Second {
		return false
	}
	d.BroadcastLast[key] = time.Now()
	return true
}

func Route() []string {
	return []string{"/start", "/saldo", "/harga", "/beli", "/jual", "/alamat", "/cekdeposit", "/status", "/menu", "/kembali", "/riwayat", "/batal", "/float",
		"/admin", "/setspread", "/setlimit", "/setfloat", "/setfee", "/sethot", "/floattopup", "/pause", "/broadcast", "/trial", "/approve"}
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
			case StepAdminSpread, StepAdminLimit, StepAdminBroadcast, StepAdminStock, StepAdminStockDepositUnits, StepAdminStockDepositRef:
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
		case BtnMenu:
			cmd = "/menu"
		case BtnKembali:
			cmd = "/kembali"
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
	case "/setfloat":
		adminFloat(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/setfee":
		adminFee(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/sethot":
		adminHot(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/floattopup":
		adminFloatTopup(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, rest, d)
	case "/float":
		adminFloatStatus(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, d)
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
	case "/menu":
		// Submenu Akun (clear state nyasar, pasang keyboard Akun).
		if d.Session != nil {
			d.Session.Clear(upd.Message.From.ID)
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: upd.Message.Chat.ID, Text: "Menu Akun:", ReplyMarkup: AccountMenuFor(d.Admin.IsAdmin(upd.Message.From.ID))})
	case "/kembali":
		// Balik ke menu utama.
		if d.Session != nil {
			d.Session.Clear(upd.Message.From.ID)
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: upd.Message.Chat.ID, Text: "Menu utama:", ReplyMarkup: MainMenuFor(d.Admin.IsAdmin(upd.Message.From.ID))})
	case "/cekdeposit":
		// Fase 9: gabung ke Status (1 pantau semua). Slash tetap jalan buat kompat.
		statusCmd(ctx, b, upd.Message.Chat.ID, upd.Message.From.ID, d)
	case "/approve":
		reply(ctx, b, upd.Message.Chat.ID, "Belum ada withdraw di sandbox (Stage 2).")
	default:
		// Phase 7 (A21): session hangus = copy kontekstual, bukan unknown.
		if d.Session != nil {
			if st := d.Session.ExpiredStep(upd.Message.From.ID); st.Step != StepIdle {
				if copy := ExpiredCopy(st); copy != "" {
					reply(ctx, b, upd.Message.Chat.ID, copy)
					return
				}
			}
		}
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

// Sistem stok v0.13 (admin saja): /setfloat /setfee /sethot /floattopup /float.
func adminFloat(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "setfloat", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) < 2 {
		reply(ctx, b, chatID, "Format: /setfloat SOL 100")
		return
	}
	before, after, err := d.Admin.SetFloat(ctx, telegramID, rest[0], rest[1])
	if err != nil {
		reply(ctx, b, chatID, "Gagal: "+err.Error())
		return
	}
	reply(ctx, b, chatID, "Float min "+strings.ToUpper(rest[0])+": "+before+" → "+after)
}

func adminFee(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "setfee", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) < 2 {
		reply(ctx, b, chatID, "Format: /setfee SOL 0.00001")
		return
	}
	before, after, err := d.Admin.SetFee(ctx, telegramID, rest[0], rest[1])
	if err != nil {
		reply(ctx, b, chatID, "Gagal: "+err.Error())
		return
	}
	reply(ctx, b, chatID, "Fee "+strings.ToUpper(rest[0])+": "+before+" → "+after)
}

func adminHot(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "sethot", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) < 2 {
		reply(ctx, b, chatID, "Format: /sethot SOL <alamat>")
		return
	}
	before, after, err := d.Admin.SetHotAddress(ctx, telegramID, rest[0], rest[1])
	if err != nil {
		reply(ctx, b, chatID, "Gagal: "+err.Error())
		return
	}
	reply(ctx, b, chatID, "Hot "+strings.ToUpper(rest[0])+": "+before+" → "+after)
}

func adminFloatTopup(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if !needAdmin(ctx, telegramID, "floattopup", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	if len(rest) < 2 {
		reply(ctx, b, chatID, "Format: /floattopup SOL 100 (catat transfer cold→hot)")
		return
	}
	amt, err := decimal.NewFromString(strings.TrimSpace(rest[1]))
	if err != nil || amt.LessThanOrEqual(decimal.Zero) {
		reply(ctx, b, chatID, "Gagal: units harus angka positif")
		return
	}
	res, err := d.Ledger.FloatTopup(ctx, rest[0], amt)
	if err != nil {
		reply(ctx, b, chatID, "Gagal: "+err.Error())
		return
	}
	_ = d.Admin.Audit(ctx, telegramID, "floattopup", 0, false, res.Asset+"=+0", res.Asset+"=+"+res.Units.String())
	bal, min, _ := d.Ledger.FloatStatus(ctx, res.Asset)
	reply(ctx, b, chatID, fmt.Sprintf("Float %s +%s (jurnal #%d). Saldo: %s / min %s",
		res.Asset, res.Units.String(), res.JournalID, bal.String(), min.String()))
}

func adminFloatStatus(ctx context.Context, b *bot.Bot, chatID, telegramID int64, d *Deps) {
	if !needAdmin(ctx, telegramID, "float", d) {
		reply(ctx, b, chatID, "Bukan admin.")
		return
	}
	reply(ctx, b, chatID, stockText(ctx, d))
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
// Phase 8 (A6): dedup 60 detik per (sender, teks) — double-tap/double-slash
// tidak broadcast dua kali. Preview sendiri nol kiriman (bukan di sini).
func doBroadcast(ctx context.Context, b *bot.Bot, telegramID int64, text string, d *Deps) string {
	key := fmt.Sprintf("%d\x00%s", telegramID, text)
	if !d.claimBroadcastSend(key) {
		_ = d.Admin.Audit(ctx, telegramID, "broadcast", 0, false, "target=dedup", "sent=0 duplicate")
		return "Broadcast sama baru saja dikirim — duplikat ditolak."
	}
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
			// Phase 7 (A16): grant dari setting aktual, pesan dinamis.
			grant := d.Set.Int64("sandbox_starting_balance_idr")
			if err := d.Ledger.GrantSandbox(ctx, nu.ID, grant); err != nil {
				return "Akun jadi, tapi grant saldo gagal. Coba /saldo."
			}
			return "Selamat datang di Crypto Station (SIMULASI).\n" +
				"Modal sandbox Rp" + formatIDR(decimal.NewFromInt(grant)) + " sudah masuk.\n" +
				"Beli dikirim ke wallet pribadi kamu — daftarin dulu via Menu → Alamat.\n" +
				"Jual dengan mengirim coin ke alamat deposit sistem."
		}
	}
	if st := u.Status; st == "blocked" {
		return "Akun diblokir. Hubungi admin."
	}
	// Phase 7 (A16): user existing tanpa grant diperbaiki; pesan pakai saldo aktual.
	if err := d.Ledger.GrantSandbox(ctx, u.ID, d.Set.Int64("sandbox_starting_balance_idr")); err != nil {
		return "Akun bermasalah, hubungi admin."
	}
	bal, err := d.Ledger.Balances(ctx, u.ID)
	if err != nil {
		return "Kamu sudah terdaftar (SIMULASI). Pakai /saldo untuk cek saldo."
	}
	return "Kamu sudah terdaftar (SIMULASI).\nSaldo IDR: Rp" + formatIDR(bal["IDR"]) + ".\nBeli dikirim ke wallet pribadi kamu. Jual via kirim coin ke alamat deposit."
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
	// Phase 7 (A24): heading lifecycle + status copy Indonesia.
	prows, err := d.Q.RecentPayoutsByUser(ctx, db.RecentPayoutsByUserParams{UserID: uid, Limit: 3})
	if err == nil && len(prows) > 0 {
		sb.WriteString("Pengiriman terbaru:\n")
		for _, r := range prows {
			tx := ""
			if r.TxHash.Valid {
				tx = " tx " + r.TxHash.String
			}
			fmt.Fprintf(&sb, "%s %s → %s (%s)%s\n", r.Asset, chain.NumericToDecimal(r.SentUnits).String(), shortAddr(r.ToAddress), PayoutStatusID(r.Status, r.RefundJournalID.Valid), tx)
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

// parseTradeArgs: /beli SOL 200000. ALL/MAX DIBATALKAN (Phase 6 A10) —
// coin tidak ngendap di bot, tidak ada yang bisa di-ALL-kan.
func parseTradeArgs(rest []string) (asset string, amountIDR int64, all bool, errText string) {
	if len(rest) < 2 {
		return "", 0, false, "Format: /beli SOL 200000. Contoh: /beli SOL 200000"
	}
	asset = strings.ToUpper(strings.TrimSpace(rest[0]))
	if !price.ValidAsset(asset) {
		return "", 0, false, "Coin tidak dikenal. Pilihan: BNB, ETH, SOL."
	}
	raw := strings.ToUpper(strings.TrimSpace(rest[1]))
	raw = strings.ReplaceAll(raw, ".", "")
	raw = strings.ReplaceAll(raw, ",", "")
	if raw == "ALL" || raw == "MAX" {
		return "", 0, false, "Jual ALL/MAX sudah tidak berlaku. Jual sekarang via deposit: /jual SOL"
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
	asset, amount, _, errText := parseTradeArgs(rest)
	if errText != "" {
		reply(ctx, b, chatID, errText)
		return
	}
	// Fase 8 transit: buy wajib alamat + fee + auto-kirim (bukan masuk saldo).
	quoteFlowBuy(ctx, b, chatID, telegramID, uid, asset, amount, d)
}

func jual(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	if _, need := needUser(ctx, telegramID, d); need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	// Phase 6 (A11): slash dan tombol satu flow deposit. Tanpa arg = picker.
	if len(rest) == 0 {
		sendCoinPicker(ctx, b, chatID, "sell")
		return
	}
	asset := strings.ToUpper(strings.TrimSpace(rest[0]))
	if !price.ValidAsset(asset) {
		reply(ctx, b, chatID, "Coin tidak dikenal. Pilihan: BNB, ETH, SOL. Contoh: /jual SOL")
		return
	}
	if len(rest) > 1 {
		raw := strings.ToUpper(strings.TrimSpace(rest[1]))
		if raw == "ALL" || raw == "MAX" {
			// Phase 6 (A10): ALL/MAX dibatalkan — tawarkan flow deposit.
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID,
				Text: "Jual ALL/MAX sudah tidak berlaku (coin tidak ngendap di bot).\n" + sellDepositText(asset, d), ReplyMarkup: DepositSimPicker(asset)})
			return
		}
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID,
		Text: sellDepositText(asset, d), ReplyMarkup: DepositSimPicker(asset)})
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
		// Phase 2 (A13): malformed q: callback — feedback, tanpa mutasi.
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: cb.Message.Message.Chat.ID, MessageID: cb.Message.Message.ID, Text: "Tombol ini sudah kedaluwarsa. Mulai ulang proses."})
		return
	}
	// Phase 2 (A12): guard q: branch - nil-safe + private + user. Spinner sudah dijawab.
	if cb.Message.Message == nil {
		return
	}
	uid, blocked := userID(ctx, d, cb.From.ID)
	if blocked == "blocked" || uid == 0 {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: cb.Message.Message.Chat.ID, MessageID: cb.Message.Message.ID, Text: "Tombol ini sudah kedaluwarsa. Mulai ulang proses."})
		return
	}
	if cb.Message.Message.Chat.Type != "private" {
		return
	}

	chatID := cb.Message.Message.Chat.ID
	msgID := cb.Message.Message.ID
	if !ok {
		// Phase 2 (A1): cancel SATU quote spesifik, owner-scoped.
		cancelled, cerr := CancelQuote(ctx, d.Q, uid, qid)
		if cerr != nil {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Gagal membatalkan, coba lagi."})
			return
		}
		if cancelled {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Quote dibatalkan."})
			return
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Quote sudah tidak aktif."})
		return
	}

	// Confirm → cek risk ulang (30 detik bisa berubah) → eksekusi atomik.
	// Phase 2 (A15): lookup owner-scoped dulu, risk sesudah — forged qid
	// user lain = generik "tidak ketemu", tanpa oracle.
	qrow, err := d.Q.GetQuoteForUser(ctx, db.GetQuoteForUserParams{ID: qid, UserID: uid})
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
	// Phase 4: buy sukses -> payout durable (queued atomik di ExecuteTrade,
	// diproses di sini). Refund HANYA diklaim setelah jurnal confirmed.
	if tr.Side == "buy" {
		tx, perr := d.Chain.Payout(ctx, uid, tr.TradeID, tr.Asset, tr.Units, tr.FeeUnits, tr.ToAddress)
		if perr != nil {
			// Failed -> refund via ledger, klaim hanya bila jurnal ada.
			if errors.Is(perr, chain.ErrPayoutFailed) && tr.PayoutID != 0 {
				if jid, rerr := d.Ledger.RefundPayoutTx(ctx, uid, tr.AmountIDR, tr.TradeID); rerr == nil {
					_ = d.Chain.SetRefundJournal(ctx, tr.PayoutID, jid)
					_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID,
						Text: "Payout gagal - dana dikembalikan (SIMULASI). Cek /saldo."})
					return
				}
				_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID,
					Text: "Payout gagal - catat dan hubungi admin. Jangan ulang transaksi."})
				return
			}
			if errors.Is(perr, chain.ErrPayoutUnknown) {
				_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID,
					Text: "Payout perlu pengecekan - jangan ulang transaksi. Cek /status."})
				return
			}
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID,
				Text: fmt.Sprintf("Beli tercatat tapi kirim gagal: %s. Hubungi admin.", perr.Error())})
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
	// Phase 6 (A9): timeline transit gabungan payout+deposit, merge-sort, limit 10.
	// Trade tanpa payout TIDAK ditampilkan sebagai terkirim.
	type item struct {
		ts   int64
		text string
	}
	var items []item
	perrOK := false
	if prows, perr := d.Q.RecentPayoutsByUser(ctx, db.RecentPayoutsByUserParams{UserID: uid, Limit: 10}); perr == nil {
		perrOK = true
		for _, r := range prows {
			ts := r.CreatedAt.Time.Unix()
			tx := "(tanpa tx)"
			if r.TxHash.Valid {
				tx = shortTx(r.TxHash.String)
			}
			items = append(items, item{ts: ts, text: fmt.Sprintf("%s Beli %s %s -> %s (%s) tx %s", formatWIB(r.CreatedAt.Time), r.Asset, chain.NumericToDecimal(r.SentUnits).String(), shortAddr(r.ToAddress), PayoutStatusID(r.Status, r.RefundJournalID.Valid), tx)})
		}
	}
	derrOK := false
	if drows, derr := d.Q.RecentDepositsByUser(ctx, db.RecentDepositsByUserParams{UserID: pgtype.Int8{Int64: uid, Valid: true}, Limit: 10}); derr == nil {
		derrOK = true
		for _, r := range drows {
			ts := r.CreatedAt.Time.Unix()
			items = append(items, item{ts: ts, text: fmt.Sprintf("%s Jual %s %s -> IDR +%s (%s) tx %s", formatWIB(r.CreatedAt.Time), r.Asset, r.Units.String(), formatIDR(decimal.NewFromInt(r.CreditedIdr)), DepositStatusID(r.Status), shortTx(r.TxHash))})
		}
	}
	if !perrOK && !derrOK {
		return "Status sedang tidak tersedia. Coba lagi."
	}
	if len(items) == 0 {
		return "Belum ada transaksi. Tap Beli buat mulai (contoh: beli SOL 50000)"
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ts > items[j].ts })
	if len(items) > 10 {
		items = items[:10]
	}
	var sb strings.Builder
	sb.WriteString("Riwayat (SIMULASI):" + "\n")
	if !perrOK || !derrOK {
		sb.WriteString("(sebagian data tidak tersedia)" + "\n")
	}
	for _, it := range items {
		sb.WriteString(it.text + "\n")
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
	// Phase 2 (A8/A23): pisah navigation-cancel vs quote-cancel, copy jujur.
	sessionCleared := false
	if d.Session != nil {
		if st := d.Session.Get(telegramID); st.Step != StepIdle {
			sessionCleared = true
		}
		d.Session.Clear(telegramID)
	}
	n, err := CancelActiveQuotes(ctx, d.Q, uid)
	if err != nil {
		return "Gagal membatalkan, coba lagi."
	}
	return BatalCopy(sessionCleared, n)
}

// expireActive: housekeeping quote yang waktunya lewat (best effort, 1 query).
// Phase 2: batal() tidak lagi memakai ini - cancel eksplisit via CancelActiveQuotes.
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
