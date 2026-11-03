// Cancel helpers Phase 2 (revisi V.1 A1/A8/A23): cancel quote eksplisit
// owner-scoped dengan count, + copy /batal 4-way yang tidak berbohong.
package bot

import (
	"context"
	"strings"

	"github.com/pikipici/crypto-station/internal/db"
)

// CancelActiveQuotes: batalkan semua quote active milik user.
// Return jumlah quote yang dibatalkan (0 = tidak ada yang aktif).
func CancelActiveQuotes(ctx context.Context, q *db.Queries, uid int64) (int64, error) {
	return q.CancelActiveQuotesByUser(ctx, uid)
}

// CancelQuote: batalkan satu quote spesifik milik user.
// Return true bila 1 baris dibatalkan; false bila sudah used/expired/cancelled
// atau bukan milik user (tanpa oracle — caller pakai copy generik).
func CancelQuote(ctx context.Context, q *db.Queries, uid, qid int64) (bool, error) {
	_, err := q.CancelQuoteByID(ctx, db.CancelQuoteByIDParams{ID: qid, UserID: uid})
	if err != nil {
		return false, err
	}
	return true, nil
}

// BatalCopy: copy /batal 4-way berdasarkan state aktual.
// Tidak pernah klaim sukses dengan zero rows.
func BatalCopy(sessionCleared bool, quotesCancelled int64) string {
	switch {
	case sessionCleared && quotesCancelled > 0:
		return "Proses dan quote dibatalkan."
	case sessionCleared && quotesCancelled == 0:
		return "Proses dibatalkan."
	case !sessionCleared && quotesCancelled > 0:
		return "Quote dibatalkan."
	default:
		return "Tidak ada proses aktif."
	}
}

// CancelAliases: teks yang dianggap intent batal (case-insensitive).
// Meliputi "batal", "/batal", dan varian tombol ❌ Batal.
func CancelAliases(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	switch t {
	case "batal", "/batal", "❌ batal":
		return true
	}
	return false
}
