// Admin flow T2-button: panel inline + input multi-step (admin-only).
// Semua entry WAJIB cek allowlist dulu (callback maupun pesan).
package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleAdminCallback: inline a:* — spread/limit/pause/broadcast/trial/close.
// Dipanggil dari handleCallback SETELAH cek prefix a:.
func handleAdminCallback(ctx context.Context, b *bot.Bot, cb *models.CallbackQuery, d *Deps) {
	uid, blocked := userID(ctx, d, cb.From.ID)
	if blocked == "blocked" || uid == 0 {
		return
	}
	// Allowlist di callback juga (bukan cuma pesan). Bukan admin = tolak + audit.
	if !d.Admin.IsAdmin(cb.From.ID) {
		_ = d.Admin.Deny(ctx, cb.From.ID, "panel:"+cb.Data)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cb.ID, Text: "Bukan admin."})
		return
	}
	chatID := cb.Message.Message.Chat.ID
	msgID := cb.Message.Message.ID
	edit := func(text string, markup *models.InlineKeyboardMarkup) {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: text, ReplyMarkup: markup})
	}
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		return
	}
	clearSess := func() {
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
	}
	switch parts[1] {
	case "close":
		clearSess()
		_ = d.Q.ExpireUserQuotes(ctx, uid)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Panel ditutup."})
		return
	case "spread":
		edit("Ubah spread coin apa?", AdminCoinPicker("spread"))
		return
	case "limit":
		edit("Ubah limit apa?", AdminLimitPicker())
		return
	case "pause":
		edit("Pause coin apa?", AdminCoinPicker("pause"))
		return
	case "broadcast":
		if d.Session != nil {
			d.Session.Set(cb.From.ID, Session{Step: StepAdminBroadcast})
		}
		edit("Ketik pesan broadcast (atau ❌ Batal):", nil)
		return
	case "trial":
		clearSess()
		text := d.Trial.Report(ctx) + "\n\n" + d.Trial.SlippageReport(ctx, 10)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
		return
	}
	// Sub-pick: a:spread:SOL / a:pause:SOL / a:limitkey:min_buy_idr
	if len(parts) == 3 {
		coin := strings.ToUpper(parts[2])
		switch parts[1] {
		case "spread":
			if coin != "BNB" && coin != "ETH" && coin != "SOL" {
				return
			}
			edit(fmt.Sprintf("Spread %s sisi apa?", coin), AdminSidePicker(coin))
			return
		case "pause":
			if coin != "BNB" && coin != "ETH" && coin != "SOL" {
				return
			}
			cur, _ := d.Set.Get("pause_" + strings.ToLower(coin))
			if cur == "" {
				cur = "off (jalan)"
			}
			edit(fmt.Sprintf("Pause %s sekarang: %s. Pilih:", coin, cur), AdminPausePicker(coin))
			return
		case "limitkey":
			key := parts[2]
			cur, _ := d.Set.Get(key)
			if d.Session != nil {
				d.Session.Set(cb.From.ID, Session{Step: StepAdminLimit, Extra: key})
			}
			edit(fmt.Sprintf("%s sekarang: %s.\nKetik nilai baru (atau ❌ Batal):", key, cur), nil)
			return
		}
	}
	// Sub-pick 4 segmen: a:spreadside:SOL:buy / a:pausekind:SOL:exposure
	if len(parts) == 4 {
		switch parts[1] {
		case "spreadside":
			coin := strings.ToUpper(parts[2])
			side := strings.ToLower(parts[3])
			if (coin != "BNB" && coin != "ETH" && coin != "SOL") || (side != "buy" && side != "sell") {
				return
			}
			cur, _ := d.Set.Get("spread_" + side + "_" + strings.ToLower(coin))
			if d.Session != nil {
				d.Session.Set(cb.From.ID, Session{Step: StepAdminSpread, Coin: coin, Extra: side})
			}
			edit(fmt.Sprintf("Spread %s %s sekarang: %s.\nKetik angka 0–0.1 (misal 0.008) atau ❌ Batal:", coin, side, cur), nil)
			return
		case "pausekind":
			coin := strings.ToUpper(parts[2])
			kind := strings.ToLower(parts[3])
			if coin != "BNB" && coin != "ETH" && coin != "SOL" {
				return
			}
			if kind != "off" && kind != "exposure" && kind != "spike_stale" {
				return
			}
			clearSess()
			before, after, err := d.Admin.Pause(ctx, cb.From.ID, coin, kind)
			if err != nil {
				edit("Gagal: "+err.Error(), AdminPanel())
				return
			}
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: fmt.Sprintf("Pause %s: %s → %s", coin, before, after)})
			edit("Panel admin (SIMULASI):", AdminPanel())
			return
		}
	}
}

// handleAdminInput: pesan teks saat session admin aktif (spread/limit/broadcast).
// Return true = ditangani (caller jangan lanjut ke flow user).
func handleAdminInput(ctx context.Context, b *bot.Bot, chatID, telegramID int64, text string, st Session, d *Deps) bool {
	// Allowlist di pesan juga. Bukan admin tapi punya session nyasar = reset + tolak.
	if !d.Admin.IsAdmin(telegramID) {
		if d.Session != nil {
			d.Session.Clear(telegramID)
		}
		_ = d.Admin.Deny(ctx, telegramID, "input:"+string(st.Step))
		reply(ctx, b, chatID, "Bukan admin.")
		return true
	}
	switch st.Step {
	case StepAdminSpread:
		val := strings.TrimSpace(text)
		f, err := strconv.ParseFloat(val, 64)
		if err != nil || f < 0 || f > 0.1 {
			reply(ctx, b, chatID, "Spread harus angka 0–0.1 (misal 0.008). Coba lagi atau tap ❌ Batal via /batal.")
			return true
		}
		if d.Session != nil {
			d.Session.Clear(telegramID)
		}
		before, after, err := d.Admin.SetSpread(ctx, telegramID, st.Coin, st.Extra, val)
		if err != nil {
			reply(ctx, b, chatID, "Gagal: "+err.Error())
			return true
		}
		reply(ctx, b, chatID, fmt.Sprintf("Spread %s %s: %s → %s", st.Coin, st.Extra, before, after))
		return true
	case StepAdminLimit:
		val := strings.TrimSpace(text)
		if d.Session != nil {
			d.Session.Clear(telegramID)
		}
		before, after, err := d.Admin.SetLimit(ctx, telegramID, st.Extra, val)
		if err != nil {
			reply(ctx, b, chatID, "Gagal: "+err.Error())
			return true
		}
		// Sinkron limiter in-memory (sama kayak slash — FIX debug #2).
		if st.Extra == "rate_limit_per_min" {
			if n, err := strconv.ParseInt(after, 10, 64); err == nil && n > 0 {
				d.Limiter.SetLimit(int(n))
			}
		}
		reply(ctx, b, chatID, st.Extra+": "+before+" → "+after)
		return true
	case StepAdminBroadcast:
		if d.Session != nil {
			d.Session.Clear(telegramID)
		}
		reply(ctx, b, chatID, doBroadcast(ctx, b, telegramID, text, d))
		return true
	}
	return false
}
