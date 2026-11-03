// Unit tests Phase 2 (revisi V.1 D1): BatalCopy 4-way, CancelAliases,
// GuardCallback nil-safety. Pure unit — tanpa DB pool, tanpa network.
// Branch guard yang butuh user lookup (Q) dicover integration di server.
package bot

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestBatalCopy4Way(t *testing.T) {
	cases := []struct {
		name           string
		sessionCleared bool
		quotes         int64
		want           string
	}{
		{"both", true, 3, "Proses dan quote dibatalkan."},
		{"session-only", true, 0, "Proses dibatalkan."},
		{"quote-only", false, 2, "Quote dibatalkan."},
		{"noop", false, 0, "Tidak ada proses aktif."},
	}
	for _, c := range cases {
		if got := BatalCopy(c.sessionCleared, c.quotes); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestCancelAliases(t *testing.T) {
	yes := []string{"batal", "/batal", "Batal", "BATAL", "❌ Batal", "  batal  ", "❌ batal"}
	for _, s := range yes {
		if !CancelAliases(s) {
			t.Errorf("expected alias true for %q", s)
		}
	}
	no := []string{"", "harga", "broadcast halo", "batalkan semua", "bat al"}
	for _, s := range no {
		if CancelAliases(s) {
			t.Errorf("expected alias false for %q", s)
		}
	}
}

func TestGuardCallbackNil(t *testing.T) {
	// Nil-safety branches return SEBELUM Q access — aman tanpa Deps DB.
	d := &Deps{}
	cases := []struct {
		name string
		cb   *models.CallbackQuery
	}{
		{"nil-cb", nil},
		{"nil-message", &models.CallbackQuery{}},
		{"nil-inner", &models.CallbackQuery{Message: models.MaybeInaccessibleMessage{}}},
	}
	for _, c := range cases {
		_, copy, ok := GuardCallback(t.Context(), d, c.cb)
		if ok {
			t.Errorf("%s: expected !ok", c.name)
		}
		if copy == "" {
			t.Errorf("%s: expected non-empty denyCopy", c.name)
		}
	}
}
