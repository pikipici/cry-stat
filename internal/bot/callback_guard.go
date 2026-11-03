// Callback guard Phase 2 (revisi V.1 A12/A13/A15): satu pintu validasi
// sebelum callback manapun boleh mutasi: nil-safe, private-chat-only,
// user terdaftar, ownership dicek caller via GetQuoteForUser.
// Caller WAJIB AnswerCallbackQuery dulu (spinner), lalu GuardCallback,
// lalu: !ok -> tampilkan denyCopy (bila non-kosong) + return tanpa mutasi.
package bot

import (
	"context"

	"github.com/go-telegram/bot/models"
)

// CallbackContext: hasil guard yang lolos — aman dipakai handler.
type CallbackContext struct {
	UID       int64
	ChatID    int64
	MsgID     int
	IsPrivate bool
}

// GuardCallback: validasi callback. Return (cctx, denyCopy, ok).
// denyCopy kosong + !ok = tolak diam-diam tapi spinner sudah dijawab caller.
// Task 6 CF-06: nil-check SEMUA level sebelum dereference (cb, From, Message,
// nested Message, Chat). Tidak ada panic untuk input apapun.
func GuardCallback(ctx context.Context, d *Deps, cb *models.CallbackQuery) (*CallbackContext, string, bool) {
	if cb == nil {
		return nil, "", false
	}
	if cb.From.ID == 0 {
		return nil, "", false
	}
	if cb.Message.Message == nil {
		return nil, "Tombol ini sudah kedaluwarsa. Mulai ulang proses.", false
	}
	msg := cb.Message.Message
	// Chat zero-value aman dibaca (struct, bukan pointer) — tapi pastikan field ada.
	if msg.Chat.Type != "private" {
		return nil, "Proses hanya bisa dari chat pribadi.", false
	}
	uid, blocked := userID(ctx, d, cb.From.ID)
	if blocked == "blocked" {
		return nil, "Akun diblokir. Hubungi admin.", false
	}
	if uid == 0 {
		return nil, "Mulai dulu dengan /start.", false
	}
	return &CallbackContext{
		UID:       uid,
		ChatID:    msg.Chat.ID,
		MsgID:     msg.ID,
		IsPrivate: true,
	}, "", true
}

// ValidateCallbackStructure: cek struktur mentah sebelum AnswerCallbackQuery pun.
// Return false = jangan sentuh cb lebih jauh (panic-safe). Dipakai dispatcher
// paling awal (Task 6 CF-06): AnswerCallback butuh cb.ID yang valid.
func ValidateCallbackStructure(cb *models.CallbackQuery) bool {
	if cb == nil {
		return false
	}
	if cb.ID == "" {
		return false
	}
	if cb.From.ID == 0 {
		return false
	}
	if cb.Message.Message == nil {
		return false
	}
	return true
}
