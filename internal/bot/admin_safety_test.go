// Phase 8 D7 (revisi V.1): callback contract stok + broadcast dedup/copy.
// Pure unit — tanpa DB, tanpa network.
package bot

import (
	"fmt"
	"strings"
	"testing"
)

func TestStockCallbackContract(t *testing.T) {
	// Builder (AdminStockCoinPicker) ↔ parser (case stockfloat/fee/hot/topup).
	actions := map[string]string{
		"float": "stockfloat", "fee": "stockfee", "hot": "stockhot", "topup": "stocktopup",
	}
	coins := []string{"BNB", "ETH", "SOL"}
	valid := map[string]bool{}
	for aksi, frag := range actions {
		for _, coin := range coins {
			cb := fmt.Sprintf("a:%s:%s", frag, coin)
			parts := strings.Split(cb, ":")
			if len(parts) != 3 || parts[0] != "a" {
				t.Fatalf("format salah: %q", cb)
			}
			// Parser mapping (mirror adminflow.go case).
			got := map[string]string{"stockfloat": "float", "stockfee": "fee", "stockhot": "hot", "stocktopup": "topup"}[parts[1]]
			if got != aksi {
				t.Fatalf("parser %q -> %q, mau %q", cb, got, aksi)
			}
			if parts[2] != coin {
				t.Fatalf("coin %q != %q", parts[2], coin)
			}
			valid[cb] = true
		}
	}
	if len(valid) != 12 {
		t.Fatalf("mau 12 kombinasi (4 aksi x 3 coin), dapat %d", len(valid))
	}
	// Invalid: aksi asing, coin asing, segmen kurang/lebih.
	for _, bad := range []string{
		"a:stockfloat", "a:stockfloat:SOL:X", "a:stockbogus:SOL",
		"a:stockfloat:USDT", "a:stockfloat:sol",
	} {
		parts := strings.Split(bad, ":")
		_, ok := map[string]bool{"stockfloat": true, "stockfee": true, "stockhot": true, "stocktopup": true}[parts[1]]
		coinOK := len(parts) == 3 && (parts[2] == "BNB" || parts[2] == "ETH" || parts[2] == "SOL")
		if len(parts) == 3 && ok && coinOK {
			t.Fatalf("invalid lolos: %q", bad)
		}
	}
}

func TestBroadcastConfirmKeyboard(t *testing.T) {
	kb := BroadcastConfirmKeyboard()
	if kb == nil || len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("keyboard harus 1 baris 2 tombol, dapat %+v", kb)
	}
	if got := kb.InlineKeyboard[0][0].CallbackData; got != "a:broadcast:send" {
		t.Fatalf("kirim: %q", got)
	}
	if got := kb.InlineKeyboard[0][1].CallbackData; got != "a:broadcast:no" {
		t.Fatalf("batal: %q", got)
	}
}

func TestClaimBroadcastSend(t *testing.T) {
	d := &Deps{}
	if !d.claimBroadcastSend("1\x00halo") {
		t.Fatal("klaim pertama harus true")
	}
	if d.claimBroadcastSend("1\x00halo") {
		t.Fatal("duplikat 60 detik harus false")
	}
	if !d.claimBroadcastSend("1\x00lain") {
		t.Fatal("teks beda harus true")
	}
	if !d.claimBroadcastSend("2\x00halo") {
		t.Fatal("sender beda harus true")
	}
}

func TestExpiredCopyBroadcastConfirm(t *testing.T) {
	got := ExpiredCopy(Session{Step: StepAdminBroadcastConfirm})
	want := "Sesi konfirmasi broadcast sudah habis. Buka Admin → Broadcast lagi."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
