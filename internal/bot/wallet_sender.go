// Package bot — R5: wallet sender boundary + decision helpers yang testable.
// Sender memisahkan Telegram I/O dari keputusan bisnis sehingga handler
// dapat diuji tanpa Bot API asli.
package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pikipici/crypto-station/internal/ledger"

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
	// value is the validated callback payload used by the router.
	value string
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
		return walletCallbackDecision{policy: "ok", action: "confirm", value: data}
	case len(data) > len("a:wdepno:") && data[:len("a:wdepno:")] == "a:wdepno:":
		return walletCallbackDecision{policy: "ok", action: "cancel", value: data}
	case data == "a:activity" || data == "a:activity:10" || data == "a:activity:25":
		return walletCallbackDecision{policy: "ok", action: "activity"}
	case len(data) > len("a:act:") && data[:len("a:act:")] == "a:act:":
		return walletCallbackDecision{policy: "ok", action: "detail"}
	default:
		return walletCallbackDecision{policy: "silent"}
	}
}

type walletMutationService interface {
	ConfirmWalletDeposit(context.Context, int64, int64) (string, *models.InlineKeyboardMarkup)
	CancelWalletDeposit(context.Context, int64, int64) (string, *models.InlineKeyboardMarkup)
}

type depsWalletMutationService struct{ d *Deps }

func (s depsWalletMutationService) ConfirmWalletDeposit(ctx context.Context, adminID, id int64) (string, *models.InlineKeyboardMarkup) {
	row, err := s.d.Q.GetAdminStockDepositByID(ctx, id)
	if err != nil || row.AdminTelegramID != adminID {
		return "Deposit tidak ditemukan.", nil
	}
	res, err := s.d.Ledger.RecordAdminStockDeposit(ctx, ledger.RecordStockDepositInput{AdminTelegramID: adminID, Asset: row.Asset, Units: row.Units, ToAddress: row.ToAddress, TxRef: row.TxRef})
	if err != nil {
		return walletConfirmCopy(err), nil
	}
	return fmt.Sprintf("Deposit stok tercatat (SIMULASI)\nDeposit ID: %d\nJournal ID: %d", res.DepositID, res.JournalID), activityDetailButton(res.DepositID)
}
func (s depsWalletMutationService) CancelWalletDeposit(ctx context.Context, adminID, id int64) (string, *models.InlineKeyboardMarkup) {
	if err := s.d.Ledger.CancelAdminStockDeposit(ctx, id, adminID); err != nil {
		return walletConfirmCopy(err), AdminWalletMenu()
	}
	return "Deposit stok dibatalkan. Tidak ada jurnal dibuat.", AdminWalletMenu()
}

// routeWalletMutationCallback routes exclusively from the parsed decision.
func routeWalletMutationCallback(ctx context.Context, sender walletSender, dec walletCallbackDecision, callbackID string, chatID int64, msgID int, adminID int64, svc walletMutationService) bool {
	switch dec.policy {
	case "deny":
		sender.answerDenied(ctx, callbackID)
		return true
	case "silent":
		return false
	}
	if dec.action == "not-ready" {
		sender.editText(ctx, chatID, msgID, "Wallet belum siap.", AdminPanel())
		return true
	}
	if dec.action != "confirm" && dec.action != "cancel" {
		return false
	}
	parts := strings.Split(dec.value, ":")
	id, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil || id <= 0 {
		sender.editText(ctx, chatID, msgID, "Tombol ini sudah tidak berlaku. Buka kembali menu Wallet.", AdminWalletMenu())
		return true
	}
	var text string
	var markup *models.InlineKeyboardMarkup
	if dec.action == "confirm" {
		text, markup = svc.ConfirmWalletDeposit(ctx, adminID, id)
	} else {
		text, markup = svc.CancelWalletDeposit(ctx, adminID, id)
	}
	sender.editText(ctx, chatID, msgID, text, markup)
	return true
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
