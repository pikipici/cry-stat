// Package bot: keyboard permanen + inline builder untuk T1-button.
// ReplyKeyboard = menu bawah permanen (user awam, tanpa slash).
// Inline = pilih coin + Confirm/Batal quote (dipakai ulang oleh slash flow).
package bot

import (
	"fmt"

	"github.com/go-telegram/bot/models"
	"github.com/pikipici/crypto-station/internal/ownerui"
)

// Teks tombol menu (tanpa slash — pesan masuk apa adanya).
const (
	BtnHarga    = "Harga"
	BtnSaldo    = "Saldo"
	BtnBeli     = "Beli"
	BtnJual     = "Jual"
	BtnRiwayat  = "Riwayat"
	BtnBatal    = "Batal"
	BtnAlamat   = "Alamat"
	BtnStatus   = "Status"
	BtnMenu     = "Menu"
	BtnKembali  = "Kembali"
	BtnAdmin    = "Admin"
	BtnResearch = "Research"
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
			{Text: "💼 Wallet & Aset", CallbackData: "a:wallet"},
			{Text: "🧾 Transaksi", CallbackData: "a:activity"},
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

// AdminStockPicker: pilih aksi stok. Callback a:stock:<aksi>.
// Aksi: lihat | float | fee | hot | topup.
func AdminStockPicker() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "📦 Lihat stok", CallbackData: "a:stock:lihat"},
		},
		{
			{Text: "📉 Float min", CallbackData: "a:stock:float"},
			{Text: "⛽ Fee", CallbackData: "a:stock:fee"},
		},
		{
			{Text: "🏦 Alamat hot", CallbackData: "a:stock:hot"},
			{Text: "➕ Isi float", CallbackData: "a:stock:topup"},
		},
		{
			{Text: "⬅️ Panel", CallbackData: "a:panel"},
		},
	}}
}

// AdminStockCoinPicker: pilih coin untuk aksi stok. Callback a:stock<aksi>:<COIN>.
func AdminStockCoinPicker(aksi string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "BNB", CallbackData: fmt.Sprintf("a:stock%s:BNB", aksi)},
			{Text: "ETH", CallbackData: fmt.Sprintf("a:stock%s:ETH", aksi)},
			{Text: "SOL", CallbackData: fmt.Sprintf("a:stock%s:SOL", aksi)},
		},
		{
			{Text: "⬅️ Wallet", CallbackData: "a:wallet"},
		},
	}}
}

// AdminWalletMenu adalah submenu operasi stok dan ringkasan jurnal sandbox.
func AdminWalletMenu() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "📊 Ringkasan Aset", CallbackData: "a:wallet:assets"}, {Text: "➕ Deposit Stok", CallbackData: "a:wallet:deposit"}},
		{{Text: "📉 Float Minimum", CallbackData: "a:stock:float"}, {Text: "⛽ Fee", CallbackData: "a:stock:fee"}},
		{{Text: "🏦 Alamat Hot", CallbackData: "a:stock:hot"}},
		{{Text: "⬅️ Panel", CallbackData: "a:panel"}},
	}}
}

func AdminWalletDepositPicker() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "BNB (BSC)", CallbackData: "a:wdep:BNB"}, {Text: "ETH (ERC20)", CallbackData: "a:wdep:ETH"}, {Text: "SOL", CallbackData: "a:wdep:SOL"}},
		{{Text: "⬅️ Wallet", CallbackData: "a:wallet"}},
	}}
}

func WalletDepositConfirm(depositID int64) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "✅ Catat Deposit", CallbackData: fmt.Sprintf("a:wdepok:%d", depositID)}, {Text: "❌ Batal", CallbackData: fmt.Sprintf("a:wdepno:%d", depositID)}},
	}}
}

func AdminActivityNav(limit int) *models.InlineKeyboardMarkup {
	next := 25
	label := "Tampilkan 25"
	if limit >= 25 {
		next, label = 10, "Tampilkan 10"
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: label, CallbackData: fmt.Sprintf("a:activity:%d", next)}},
		{{Text: "⬅️ Panel", CallbackData: "a:panel"}},
	}}
}

func ActivityDetailBack() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "⬅️ Transaksi", CallbackData: "a:activity"}}}}
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

// MainMenuFor: user mendapat Beli/Jual/Harga/Menu; admin hanya mendapat Admin.
// Submenu Akun (Saldo Riwayat Status Alamat) dibuka via tombol Menu untuk user.
func MainMenuFor(isAdmin bool) *models.ReplyKeyboardMarkup {
	kb := [][]models.KeyboardButton{
		{{Text: BtnBeli}, {Text: BtnJual}},
		{{Text: BtnHarga}, {Text: BtnMenu}},
	}
	if isAdmin {
		kb = [][]models.KeyboardButton{{{Text: BtnAdmin}}}
	}
	return &models.ReplyKeyboardMarkup{
		Keyboard:       kb,
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}

// MainMenuForMode keeps legacy_sandbox as the safe default. The pivot mode
// exposes only a non-trading Research entry to the allowlisted owner.
func MainMenuForMode(isOwner bool, productMode string) *models.ReplyKeyboardMarkup {
	if NormalizeProductMode(productMode) != ProductModeMemeResearch {
		return MainMenuFor(isOwner)
	}
	if isOwner {
		return ownerui.Keyboard()
	}
	return &models.ReplyKeyboardMarkup{Keyboard: [][]models.KeyboardButton{}, ResizeKeyboard: true, IsPersistent: true}
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

// Phase 6 (A10): SellAmountPicker (Jual SEMUA) DIHAPUS — sell-bandar mati total.

// BroadcastConfirmKeyboard: preview broadcast admin (Phase 8 A6).
// Kirim = a:broadcast:send, Batal = a:broadcast:no. Tanpa callback ini
// tidak ada pengiriman (preview nol kiriman).
func BroadcastConfirmKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "✅ Kirim broadcast", CallbackData: "a:broadcast:send"},
			{Text: "❌ Batal", CallbackData: "a:broadcast:no"},
		},
	}}
}

// QuoteKeyboard: Konfirmasi/Batal untuk quote id. Dipakai slash + button flow.
// Phase 7 (A25): label Indonesia; callback data tetap q:<id>:ok/no.
func QuoteKeyboard(quoteID int64) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "✅ Konfirmasi", CallbackData: fmt.Sprintf("q:%d:ok", quoteID)},
			{Text: "❌ Batal", CallbackData: fmt.Sprintf("q:%d:no", quoteID)},
		},
	}}
}

// DepositSimPicker: simulasi kirim masuk + nominal pilih.
// Phase 7 (A22): label jujur "senilai" — kredit neto = nominal − fee,
// bukan nominal penuh. Callback t:depsim:SOL:50000 (units IDR mock).
func DepositSimPicker(coin string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Simulasi kirim senilai Rp50rb", CallbackData: fmt.Sprintf("t:depsim:%s:50000", coin)},
			{Text: "Simulasi kirim senilai Rp100rb", CallbackData: fmt.Sprintf("t:depsim:%s:100000", coin)},
		},
		{
			{Text: "❌ Batal", CallbackData: "t:cancel"},
		},
	}}
}
