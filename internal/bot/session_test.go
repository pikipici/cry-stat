package bot

import (
	"testing"
	"time"
)

func TestParseNominal(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"50000", 50000, true},
		{"50.000", 50000, true},
		{"Rp50.000", 50000, true},
		{"rp 50,000", 50000, true},
		{"  100000  ", 100000, true},
		{"halo", 0, false},
		{"50rb", 0, false},
		{"-100", 0, false},
		{"0", 0, false},
		{"", 0, false},
		{"ALL", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseNominal(c.in)
		if ok != c.ok || got != c.want {
			t.Fatalf("ParseNominal(%q) = (%d,%v), mau (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMatchButton(t *testing.T) {
	cases := map[string]string{
		"Harga": BtnHarga, "/harga": BtnHarga, "HARGA": BtnHarga,
		"Saldo": BtnSaldo, "/saldo": BtnSaldo,
		"Beli": BtnBeli, "/beli": BtnBeli,
		"Jual": BtnJual, "/jual": BtnJual,
		"Riwayat": BtnRiwayat, "/riwayat": BtnRiwayat,
		"Batal": BtnBatal, "/batal": BtnBatal,
		"Alamat": BtnAlamat, "/alamat": BtnAlamat,
		"Status": BtnStatus, "/status": BtnStatus,
		"Menu": BtnMenu, "/menu": BtnMenu,
		"Kembali": BtnKembali, "/kembali": BtnKembali,
		"Admin": BtnAdmin, "/admin": BtnAdmin,
		"halo": "", "50000": "", "/setspread": "",
	}
	for in, want := range cases {
		if got := MatchButton(in); got != want {
			t.Fatalf("MatchButton(%q) = %q, mau %q", in, got, want)
		}
	}
}

func TestSessionsTimeout(t *testing.T) {
	s := NewSessions()
	s.Set(1, Session{Step: StepAwaitBuy, Coin: "SOL"})
	if got := s.Get(1); got.Step != StepAwaitBuy || got.Coin != "SOL" {
		t.Fatalf("state fresh hilang: %+v", got)
	}
	// Suntik state basi langsung ke map.
	s.mu.Lock()
	s.m[2] = Session{Step: StepAwaitSell, Coin: "ETH", UpdatedAt: time.Now().Add(-5 * time.Minute)}
	s.mu.Unlock()
	if got := s.Get(2); got.Step != StepIdle {
		t.Fatalf("state basi tidak hangus: %+v", got)
	}
	s.Clear(1)
	if got := s.Get(1); got.Step != StepIdle {
		t.Fatalf("clear gagal: %+v", got)
	}
}

func TestKeyboardsNotNil(t *testing.T) {
	if MainMenu() == nil || CoinPicker("buy") == nil || SellAmountPicker("SOL") == nil || QuoteKeyboard(1) == nil {
		t.Fatal("keyboard builder mengembalikan nil")
	}
	if MainMenuFor(false) == nil || MainMenuFor(true) == nil {
		t.Fatal("MainMenuFor nil")
	}
	if len(MainMenuFor(false).Keyboard) != 2 {
		t.Fatalf("menu utama user harus 2 baris, dapat %d", len(MainMenuFor(false).Keyboard))
	}
	if len(MainMenuFor(true).Keyboard) != 3 {
		t.Fatalf("menu utama admin harus 3 baris, dapat %d", len(MainMenuFor(true).Keyboard))
	}
	if MainMenuFor(true).Keyboard[2][0].Text != BtnAdmin {
		t.Fatalf("baris admin salah: %q", MainMenuFor(true).Keyboard[2][0].Text)
	}
	if MainMenuFor(false).Keyboard[0][0].Text != BtnBeli || MainMenuFor(false).Keyboard[1][1].Text != BtnMenu {
		t.Fatalf("menu utama harus Beli/Jual/Harga/Menu: %+v", MainMenuFor(false).Keyboard)
	}
	if AccountMenuFor(false) == nil || AccountMenuFor(true) == nil {
		t.Fatal("AccountMenuFor nil")
	}
	if len(AccountMenuFor(false).Keyboard) != 3 {
		t.Fatalf("submenu akun harus 3 baris, dapat %d", len(AccountMenuFor(false).Keyboard))
	}
	if AccountMenuFor(false).Keyboard[2][0].Text != BtnKembali {
		t.Fatalf("submenu harus ada Kembali: %+v", AccountMenuFor(false).Keyboard)
	}
	if AddrCoinPicker() == nil {
		t.Fatal("AddrCoinPicker nil")
	}
	if got := AddrCoinPicker().InlineKeyboard[0][2].CallbackData; got != "t:addr:SOL" {
		t.Fatalf("callback addr salah: %q", got)
	}
	if AdminPanel() == nil || AdminCoinPicker("spread") == nil || AdminSidePicker("SOL") == nil || AdminPausePicker("SOL") == nil || AdminLimitPicker() == nil {
		t.Fatal("keyboard admin nil")
	}
	if got := AdminCoinPicker("pause").InlineKeyboard[0][2].CallbackData; got != "a:pause:SOL" {
		t.Fatalf("callback admin coin salah: %q", got)
	}
	if got := AdminSidePicker("SOL").InlineKeyboard[0][0].CallbackData; got != "a:spreadside:SOL:buy" {
		t.Fatalf("callback spreadside salah: %q", got)
	}
	if got := AdminPausePicker("ETH").InlineKeyboard[1][0].CallbackData; got != "a:pausekind:ETH:exposure" {
		t.Fatalf("callback pausekind salah: %q", got)
	}
	if got := AdminLimitPicker().InlineKeyboard[0][0].CallbackData; got != "a:limitkey:min_buy_idr" {
		t.Fatalf("callback limitkey salah: %q", got)
	}
	if got := CoinPicker("sell").InlineKeyboard[0][2].CallbackData; got != "t:sell:SOL" {
		t.Fatalf("callback coin salah: %q", got)
	}
	if got := SellAmountPicker("ETH").InlineKeyboard[0][0].CallbackData; got != "t:sellall:ETH" {
		t.Fatalf("callback sellall salah: %q", got)
	}
	if got := QuoteKeyboard(42).InlineKeyboard[0][0].CallbackData; got != "q:42:ok" {
		t.Fatalf("callback quote salah: %q", got)
	}
}
