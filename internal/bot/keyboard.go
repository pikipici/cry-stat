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
)

// MainMenu: keyboard permanen bawah. Resize + persistent.
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
