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
func GuardCallback(ctx context.Context, d *Deps, cb *models.CallbackQuery) (*CallbackContext, string, bool) {
	if cb == nil || cb.Message.Message == nil {
		return nil, "Tombol ini sudah kedaluwarsa. Mulai ulang proses.", false
	}
	chat := cb.Message.Message.Chat
	if chat.Type != "private" {
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
		ChatID:    chat.ID,
		MsgID:     cb.Message.Message.ID,
		IsPrivate: true,
	}, "", true
}
