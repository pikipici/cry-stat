// Package bot: keyboard permanen + inline builder untuk T1-button.
// ReplyKeyboard = menu bawah permanen (user awam, tanpa slash).
// Inline = pilih coin + Confirm/Batal quote (dipakai ulang oleh slash flow).
package bot

import (
	"fmt"

	"github.com/go-telegram/bot/models"
)

// Teks tombol menu (tanpa slash — pesan masuk apa adanya).
const (
	BtnHarga   = "Harga"
	BtnSaldo   = "Saldo"
	BtnBeli    = "Beli"
	BtnJual    = "Jual"
	BtnRiwayat = "Riwayat"
	BtnBatal   = "Batal"
	BtnAlamat  = "Alamat"
	BtnStatus  = "Status"
	BtnMenu    = "Menu"
	BtnKembali = "Kembali"
	BtnAdmin   = "Admin"
)

// AdminPanel: inline panel admin (dibuka via /admin atau tombol Admin).
// Semua callback prefix a: — handler wajib cek allowlist dulu.
func AdminPanel() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "📊 Spread", CallbackData: "a:spread"},
			{Text: "🔢 Limit", CallbackData: "a:limit"},
		},
		{
			{Text: "⏸️ Pause", CallbackData: "a:pause"},
			{Text: "📢 Broadcast", CallbackData: "a:broadcast"},
		},
		{
			{Text: "🧪 Trial", CallbackData: "a:trial"},
		},
		{
			{Text: "❌ Tutup", CallbackData: "a:close"},
		},
	}}
}

// AdminCoinPicker: pilih coin untuk spread/pause. use = "spread" | "pause".
func AdminCoinPicker(use string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "BNB", CallbackData: fmt.Sprintf("a:%s:BNB", use)},
			{Text: "ETH", CallbackData: fmt.Sprintf("a:%s:ETH", use)},
			{Text: "SOL", CallbackData: fmt.Sprintf("a:%s:SOL", use)},
		},
		{
			{Text: "❌ Batal", CallbackData: "a:close"},
		},
	}}
}

// AdminSidePicker: pilih sisi spread untuk coin. Callback a:spreadside:SOL:buy.
func AdminSidePicker(coin string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Buy", CallbackData: fmt.Sprintf("a:spreadside:%s:buy", coin)},
			{Text: "Sell", CallbackData: fmt.Sprintf("a:spreadside:%s:sell", coin)},
		},
		{
			{Text: "❌ Batal", CallbackData: "a:close"},
		},
	}}
}

// AdminPausePicker: pilih jenis pause untuk coin. Callback a:pausekind:SOL:exposure.
func AdminPausePicker(coin string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "▶️ Off (jalan)", CallbackData: fmt.Sprintf("a:pausekind:%s:off", coin)},
		},
		{
			{Text: "⏸️ Exposure (stop beli)", CallbackData: fmt.Sprintf("a:pausekind:%s:exposure", coin)},
		},
		{
			{Text: "⛔ Spike/Stale (stop semua)", CallbackData: fmt.Sprintf("a:pausekind:%s:spike_stale", coin)},
		},
		{
			{Text: "❌ Batal", CallbackData: "a:close"},
		},
	}}
}

// AdminLimitPicker: pilih key limit (whitelist sama kayak SetLimit).
// Callback a:limitkey:<key>.
var adminLimitKeys = []string{
	"min_buy_idr",
	"min_topup_idr",
	"min_withdraw_idr",
	"daily_topup_limit_idr",
	"daily_withdraw_limit_idr",
	"withdraw_approval_threshold_idr",
	"quote_lock_seconds",
	"staleness_coin_ms",
	"staleness_fx_ms",
	"rate_limit_per_min",
	"exposure_limit_bnb",
	"exposure_limit_eth",
	"exposure_limit_sol",
}

func AdminLimitPicker() *models.InlineKeyboardMarkup {
	rows := [][]models.InlineKeyboardButton{}
	for i := 0; i < len(adminLimitKeys); i += 2 {
		row := []models.InlineKeyboardButton{
			{Text: adminLimitKeys[i], CallbackData: "a:limitkey:" + adminLimitKeys[i]},
		}
		if i+1 < len(adminLimitKeys) {
			row = append(row, models.InlineKeyboardButton{Text: adminLimitKeys[i+1], CallbackData: "a:limitkey:" + adminLimitKeys[i+1]})
		}
		rows = append(rows, row)
	}
	rows = append(rows, []models.InlineKeyboardButton{{Text: "❌ Batal", CallbackData: "a:close"}})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// MainMenu: keyboard permanen bawah. Resize + persistent.
// Baris Admin dirender terpisah via MainMenuFor (admin-only).
func MainMenu() *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{{Text: BtnHarga}, {Text: BtnSaldo}},
			{{Text: BtnBeli}, {Text: BtnJual}},
			{{Text: BtnRiwayat}, {Text: BtnBatal}},
		},
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}

// MainMenuFor: menu UTAMA ramping (v0.12-ux) — Beli Jual Harga Menu (+ Admin).
// Submenu Akun (Saldo Riwayat Status Alamat) dibuka via tombol Menu.
func MainMenuFor(isAdmin bool) *models.ReplyKeyboardMarkup {
	kb := [][]models.KeyboardButton{
		{{Text: BtnBeli}, {Text: BtnJual}},
		{{Text: BtnHarga}, {Text: BtnMenu}},
	}
	if isAdmin {
		kb = append(kb, []models.KeyboardButton{{Text: BtnAdmin}})
	}
	return &models.ReplyKeyboardMarkup{
		Keyboard:       kb,
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}

// AccountMenuFor: submenu Akun — Saldo Riwayat Status Alamat + Kembali (+ Admin).
func AccountMenuFor(isAdmin bool) *models.ReplyKeyboardMarkup {
	kb := [][]models.KeyboardButton{
		{{Text: BtnSaldo}, {Text: BtnRiwayat}},
		{{Text: BtnStatus}, {Text: BtnAlamat}},
		{{Text: BtnKembali}},
	}
	if isAdmin {
		kb = append(kb, []models.KeyboardButton{{Text: BtnAdmin}})
	}
	return &models.ReplyKeyboardMarkup{
		Keyboard:       kb,
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}

// AddrCoinPicker: pilih coin buat daftarin alamat. Callback t:addr:SOL.
func AddrCoinPicker() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "BNB (BSC)", CallbackData: "t:addr:BNB"},
			{Text: "ETH (ERC20)", CallbackData: "t:addr:ETH"},
			{Text: "SOL", CallbackData: "t:addr:SOL"},
		},
		{
			{Text: "❌ Batal", CallbackData: "t:cancel"},
		},
	}}
}

// CoinPicker: inline pilih coin untuk side buy/sell. Callback: t:buy:SOL / t:sell:ETH.
func CoinPicker(side string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "BNB", CallbackData: fmt.Sprintf("t:%s:BNB", side)},
			{Text: "ETH", CallbackData: fmt.Sprintf("t:%s:ETH", side)},
			{Text: "SOL", CallbackData: fmt.Sprintf("t:%s:SOL", side)},
		},
		{
			{Text: "❌ Batal", CallbackData: "t:cancel"},
		},
	}}
}

// SellAmountPicker: jual SEMUA (units langsung) atau ketik nominal.
// Callback t:sellall:SOL membawa coin; nominal diketik sebagai pesan biasa.
func SellAmountPicker(coin string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "💰 Jual SEMUA", CallbackData: fmt.Sprintf("t:sellall:%s", coin)},
		},
		{
			{Text: "❌ Batal", CallbackData: "t:cancel"},
		},
	}}
}

// QuoteKeyboard: Confirm/Batal untuk quote id. Dipakai slash + button flow.
func QuoteKeyboard(quoteID int64) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "✅ Confirm", CallbackData: fmt.Sprintf("q:%d:ok", quoteID)},
			{Text: "❌ Batal", CallbackData: fmt.Sprintf("q:%d:no", quoteID)},
		},
	}}
}

// DepositSimPicker: Fase 9 — simulasi kirim masuk + nominal pilih.
// Callback t:depsim:SOL:50000 (units IDR mock).
func DepositSimPicker(coin string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Rp50rb masuk", CallbackData: fmt.Sprintf("t:depsim:%s:50000", coin)},
			{Text: "Rp100rb masuk", CallbackData: fmt.Sprintf("t:depsim:%s:100000", coin)},
		},
		{
			{Text: "❌ Batal", CallbackData: "t:cancel"},
		},
	}}
}
