// Package bot — R5: wallet sender boundary + decision helpers yang testable.
// Sender memisahkan Telegram I/O dari keputusan bisnis sehingga handler
// dapat diuji tanpa Bot API asli.
package bot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// walletSender adalah batas I/O Telegram untuk flow wallet.
type walletSender interface {
	editText(ctx context.Context, chatID int64, msgID int, text string, markup *models.InlineKeyboardMarkup)
	sendText(ctx context.Context, chatID int64, text string)
	sendInline(ctx context.Context, chatID int64, text string, markup *models.InlineKeyboardMarkup)
	answerDenied(ctx context.Context, callbackID string)
}

// liveWalletSender meneruskan ke Bot API asli.
type liveWalletSender struct {
	b *bot.Bot
}

func (s liveWalletSender) editText(ctx context.Context, chatID int64, msgID int, text string, markup *models.InlineKeyboardMarkup) {
	_, _ = s.b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: text, ReplyMarkup: markup})
}

func (s liveWalletSender) sendText(ctx context.Context, chatID int64, text string) {
	_, _ = s.b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
}

func (s liveWalletSender) sendInline(ctx context.Context, chatID int64, text string, markup *models.InlineKeyboardMarkup) {
	_, _ = s.b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup})
}

func (s liveWalletSender) answerDenied(ctx context.Context, callbackID string) {
	_, _ = s.b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: callbackID, Text: "Bukan admin."})
}

// walletCallbackDecision adalah keputusan murni untuk callback wallet.
// Dipisahkan dari I/O agar dapat diuji tabel tanpa Telegram API.
type walletCallbackDecision struct {
	// allowополитика: "ok" lanjut, "deny" tolak + audit, "silent" abaikan.
	policy string
	// action bisnis: "menu", "assets", "deposit-picker", "none", ...
	action string
	// denyReason diisi saat policy == "deny".
	denyReason string
}

// decideWalletCallback memetakan data callback + status admin ke keputusan.
// Tidak menyentuh DB/session/jurnal — hanya routing + allowlist.
func decideWalletCallback(data string, isAdmin bool, walletReady bool) walletCallbackDecision {
	if data == "" {
		return walletCallbackDecision{policy: "silent"}
	}
	// Callback wallet selalu admin-only.
	if !isAdmin {
		return walletCallbackDecision{policy: "deny", denyReason: "panel:" + data}
	}
	if !walletReady {
		return walletCallbackDecision{policy: "ok", action: "not-ready"}
	}
	switch {
	case data == "a:wallet":
		return walletCallbackDecision{policy: "ok", action: "menu"}
	case data == "a:wallet:assets":
		return walletCallbackDecision{policy: "ok", action: "assets"}
	case data == "a:wallet:deposit":
		return walletCallbackDecision{policy: "ok", action: "deposit-picker"}
	case len(data) > len("a:wdep:") && data[:len("a:wdep:")] == "a:wdep:":
		coin := data[len("a:wdep:"):]
		if !validWalletAsset(coin) {
			return walletCallbackDecision{policy: "silent"}
		}
		return walletCallbackDecision{policy: "ok", action: "deposit-coin"}
	case len(data) > len("a:wdepok:") && data[:len("a:wdepok:")] == "a:wdepok:":
		return walletCallbackDecision{policy: "ok", action: "confirm"}
	case len(data) > len("a:wdepno:") && data[:len("a:wdepno:")] == "a:wdepno:":
		return walletCallbackDecision{policy: "ok", action: "cancel"}
	case data == "a:activity" || data == "a:activity:10" || data == "a:activity:25":
		return walletCallbackDecision{policy: "ok", action: "activity"}
	case len(data) > len("a:act:") && data[:len("a:act:")] == "a:act:":
		return walletCallbackDecision{policy: "ok", action: "detail"}
	default:
		return walletCallbackDecision{policy: "silent"}
	}
}

// walletConfirmCopy memetakan error ledger ke copy user (§8 revisi).
// Return copy yang aman (tanpa payload lama) + flag apakah saldo dijamin tetap.
func walletConfirmCopy(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case isStockConflict(err):
		return "Reference ini sudah pernah digunakan untuk data lain. Gunakan reference transaksi yang benar atau hubungi operator."
	case isStockCancelled(err):
		return "Reference ini sebelumnya sudah dibatalkan dan tidak dapat digunakan ulang. Buat reference baru untuk pencatatan baru."
	case isStockUnauthorized(err):
		return "Deposit tidak ditemukan."
	case containsAuditFailure(msg):
		return "Deposit belum dicatat karena sistem gagal menyimpan jejak audit. Saldo tidak berubah. Coba lagi."
	default:
		return fmt.Sprintf("Gagal mencatat deposit: %s", msg)
	}
}
