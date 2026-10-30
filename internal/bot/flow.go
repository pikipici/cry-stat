// Flow T1-button: pilih coin → input nominal → quote → Confirm/Batal.
// Dipanggil dari dispatcher (pesan) dan handleCallback (inline t:*).
package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
)

// handleFlowCallback: inline t:buy:SOL / t:sell:ETH / t:sellall:SOL / t:cancel.
func handleFlowCallback(ctx context.Context, b *bot.Bot, cb *models.CallbackQuery, d *Deps) {
	uid, blocked := userID(ctx, d, cb.From.ID)
	if blocked == "blocked" || uid == 0 {
		return
	}
	chatID := cb.Message.Message.Chat.ID
	msgID := cb.Message.Message.ID
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		return
	}
	switch parts[1] {
	case "cancel":
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
		_ = d.Q.ExpireUserQuotes(ctx, uid)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Dibatalkan."})
		return
	case "buy", "sell":
		if len(parts) != 3 {
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			return
		}
		if d.Session != nil {
			step := StepAwaitBuy
			if parts[1] == "sell" {
				step = StepAwaitSell
			}
			d.Session.Set(cb.From.ID, Session{Step: step, Coin: coin})
		}
		if parts[1] == "sell" {
			// Jual: kasih opsi SEMUA + ketik nominal.
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID: chatID, MessageID: msgID,
				Text:        fmt.Sprintf("Jual %s: tap tombol atau ketik nominal IDR (min %s).", coin, d.Set.MustString("min_buy_idr")),
				ReplyMarkup: SellAmountPicker(coin),
			})
			return
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID: chatID, MessageID: msgID,
			Text: fmt.Sprintf("Mau beli %s berapa? Ketik angka IDR (min %s). Contoh: 50000", coin, d.Set.MustString("min_buy_idr")),
		})
		return
	case "sellall":
		if len(parts) != 3 {
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			return
		}
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
		// Edit pesan picker biar tidak bisa ditap 2x, lalu kirim quote baru.
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Siapin quote jual SEMUA " + coin + "..."})
		sellAllFlow(ctx, b, chatID, cb.From.ID, uid, coin, d)
		return
	}
}

// handleAmountInput: user lagi await nominal dan mengetik angka.
// st = session fresh (sudah dicek timeout oleh caller).
func handleAmountInput(ctx context.Context, b *bot.Bot, chatID, telegramID int64, text string, st Session, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	amount, ok := ParseNominal(text)
	if !ok {
		reply(ctx, b, chatID, "Nominal nggak kebaca. Ketik angka IDR aja, contoh: 50000")
		return
	}
	if amount < d.Set.Int64("min_buy_idr") {
		reply(ctx, b, chatID, fmt.Sprintf("Minimum Rp%s. Coba lagi atau tap Batal.", d.Set.MustString("min_buy_idr")))
		return
	}
	side := "buy"
	if st.Step == StepAwaitSell {
		side = "sell"
	}
	if d.Session != nil {
		d.Session.Clear(telegramID)
	}
	quoteFlow(ctx, b, chatID, telegramID, uid, side, st.Coin, amount, d)
}

// successHint: teks sukses trade pakai hint button (slash tetap disebut sekali).
func successHint(verb, units, asset string, amountIDR int64) string {
	return fmt.Sprintf("%s sukses (SIMULASI): %s %s = Rp%s.\nCek Saldo, riwayat di Riwayat.", verb, units, asset, formatIDR(decimal.NewFromInt(amountIDR)))
}
