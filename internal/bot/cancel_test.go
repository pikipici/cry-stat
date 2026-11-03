// Unit tests Phase 2 (revisi V.1 D1): BatalCopy 4-way, CancelAliases,
// GuardCallback nil-safety. Pure unit — tanpa DB pool, tanpa network.
// Branch guard yang butuh user lookup (Q) dicover integration di server.
package bot

import (
	"context"
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
	// Task 6 CF-06: cb nil / From nil = silent reject (deny boleh kosong,
	// spinner pun tak bisa dijawab); message nil = denyCopy ada.
	d := &Deps{}
	silent := []struct {
		name string
		cb   *models.CallbackQuery
	}{
		{"nil-cb", nil},
		{"nil-from", &models.CallbackQuery{ID: "x"}},
		{"empty-id", &models.CallbackQuery{ID: "", From: models.User{ID: 1}}},
	}
	for _, c := range silent {
		if ValidateCallbackStructure(c.cb) {
			t.Errorf("%s: ValidateCallbackStructure harus false", c.name)
		}
		_, _, ok := GuardCallback(context.Background(), d, c.cb)
		if ok {
			t.Errorf("%s: expected !ok", c.name)
		}
	}
	loud := []struct {
		name string
		cb   *models.CallbackQuery
	}{
		{"nil-message", &models.CallbackQuery{ID: "x", From: models.User{ID: 1}}},
		{"nil-inner", &models.CallbackQuery{ID: "x", From: models.User{ID: 1}, Message: models.MaybeInaccessibleMessage{}}},
	}
	for _, c := range loud {
		if ValidateCallbackStructure(c.cb) {
			t.Errorf("%s: ValidateCallbackStructure harus false", c.name)
		}
		_, copy, ok := GuardCallback(context.Background(), d, c.cb)
		if ok {
			t.Errorf("%s: expected !ok", c.name)
		}
		if copy == "" {
			t.Errorf("%s: expected non-empty denyCopy", c.name)
		}
	}
}

func TestGuardCallbackGroupRejected(t *testing.T) {
	// Group/channel callback ditolak sebelum user lookup (tanpa Deps DB).
	// Message non-nil tapi Chat.Type group → deny private.
	d := &Deps{}
	cb := &models.CallbackQuery{
		ID:   "x",
		From: models.User{ID: 1},
		Message: models.MaybeInaccessibleMessage{
			Message: &models.Message{
				Chat: models.Chat{ID: -100, Type: "group"},
			},
		},
	}
	if !ValidateCallbackStructure(cb) {
		t.Fatalf("structural harus true untuk group cb lengkap")
	}
	_, copy, ok := GuardCallback(context.Background(), d, cb)
	if ok {
		t.Fatalf("group harus ditolak")
	}
	if copy == "" {
		t.Fatalf("group harus ada denyCopy")
	}
}

func TestValidateCallbackStructureMatrix(t *testing.T) {
	full := &models.CallbackQuery{
		ID:   "abc",
		From: models.User{ID: 7},
		Message: models.MaybeInaccessibleMessage{
			Message: &models.Message{ID: 1, Chat: models.Chat{ID: 7, Type: "private"}},
		},
	}
	if !ValidateCallbackStructure(full) {
		t.Fatalf("callback lengkap harus valid")
	}
	// Tanpa From tetap structural-false walau field lain lengkap.
	noFrom := &models.CallbackQuery{
		ID: "abc",
		Message: models.MaybeInaccessibleMessage{
			Message: &models.Message{ID: 1, Chat: models.Chat{ID: 7, Type: "private"}},
		},
	}
	if ValidateCallbackStructure(noFrom) {
		t.Fatalf("tanpa From harus invalid")
	}
}
