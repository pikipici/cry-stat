package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pikipici/crypto-station/internal/adminwallet"
	"github.com/pikipici/crypto-station/internal/chain"
	"github.com/pikipici/crypto-station/internal/db"
	"github.com/pikipici/crypto-station/internal/ledger"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
)

// handleWalletCallback hanya mengorkestrasi service; seluruh mutasi saldo tetap di ledger.
// Allowlist utama dicek di handleAdminCallback sebelum delegasi ke sini.
// Decision check di bawah adalah defense-in-depth (diuji di wallet_sender_test.go).
func handleWalletCallback(ctx context.Context, b *bot.Bot, cb *models.CallbackQuery, d *Deps, parts []string) {
	dec := decideWalletCallback(cb.Data, d.Admin.IsAdmin(cb.From.ID), d.Wallet != nil && d.Q != nil && d.Ledger != nil)
	if dec.policy == "deny" {
		_ = d.Admin.Deny(ctx, cb.From.ID, dec.denyReason)
		liveWalletSender{b: b}.answerDenied(ctx, cb.ID)
		return
	}
	if dec.policy == "silent" {
		return
	}
	chatID, msgID := cb.Message.Message.Chat.ID, cb.Message.Message.ID
	edit := func(text string, markup *models.InlineKeyboardMarkup) {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: text, ReplyMarkup: markup})
	}
	clear := func() {
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
	}
	if d.Wallet == nil || d.Q == nil || d.Ledger == nil {
		edit("Wallet belum siap.", AdminPanel())
		return
	}

	switch parts[1] {
	case "wallet":
		if len(parts) == 2 {
			clear()
			edit("Wallet & Aset (SIMULASI)", AdminWalletMenu())
			return
		}
		switch parts[2] {
		case "assets":
			items, err := d.Wallet.AssetSummaries(ctx)
			if err != nil {
				edit("Gagal membaca aset: "+err.Error(), AdminWalletMenu())
				return
			}
			edit(formatAssetSummary(items), AdminWalletMenu())
		case "deposit":
			clear()
			edit("Deposit stok (SIMULASI)\nPilih aset yang dikirim cold → alamat hot.", AdminWalletDepositPicker())
		}
	case "wdep":
		if len(parts) != 3 || !validWalletAsset(parts[2]) {
			return
		}
		asset := strings.ToUpper(parts[2])
		hot, _ := d.Set.Get("hot_address_" + strings.ToLower(asset))
		hot = strings.TrimSpace(hot)
		if err := validateWalletDepositStart(asset, hot); err != nil {
			clear()
			edit("Alamat hot "+asset+" belum diatur. Atur dulu, lalu ulangi Deposit Stok.", AdminStockCoinPicker("hot"))
			return
		}
		if d.Session != nil {
			d.Session.Set(cb.From.ID, Session{Step: StepAdminStockDepositUnits, Coin: asset, HotAddr: hot})
		}
		edit(fmt.Sprintf("Deposit %s (SIMULASI)\nNetwork: %s\nAlamat hot: %s\nSaldo jurnal belum berubah.\n\nKetik units positif yang akan dicatat:", asset, adminwallet.NetworkFor(asset), shortAddress(hot)), nil)
	case "wdepok":
		if len(parts) != 3 {
			return
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || id <= 0 {
			return
		}
		handleWalletConfirm(ctx, b, chatID, cb.From.ID, id, d)
	case "wdepno":
		if len(parts) != 3 {
			return
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || id <= 0 {
			return
		}
		if err := d.Ledger.CancelAdminStockDeposit(ctx, id, cb.From.ID); err != nil {
			switch {
			case errors.Is(err, ledger.ErrStockDepositUnauthorized):
				edit("Deposit tidak ditemukan.", AdminWalletMenu())
			case errors.Is(err, ledger.ErrStockDepositCancelled):
				edit("Reference ini sebelumnya sudah dibatalkan dan tidak dapat digunakan ulang. Buat reference baru untuk pencatatan baru.", AdminWalletMenu())
			case strings.Contains(err.Error(), "audit"):
				edit("Deposit belum dicatat karena sistem gagal menyimpan jejak audit. Saldo tidak berubah. Coba lagi.", AdminWalletMenu())
			default:
				edit("Gagal membatalkan deposit.", AdminWalletMenu())
			}
			return
		}
		clear()
		edit("Deposit stok dibatalkan. Tidak ada jurnal dibuat.", AdminWalletMenu())
	case "activity":
		limit := 10
		if len(parts) == 3 {
			if n, err := strconv.Atoi(parts[2]); err == nil && (n == 10 || n == 25) {
				limit = n
			} else {
				return
			}
		}
		items, err := d.Wallet.ListRecent(ctx, limit)
		if err != nil {
			edit("Gagal membaca transaksi: "+err.Error(), AdminPanel())
			return
		}
		markup := activityKeyboard(items, limit)
		edit(formatActivityList(items, limit), markup)
	case "act":
		if len(parts) != 4 || !validActivityKind(parts[2]) {
			return
		}
		id, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil || id <= 0 {
			return
		}
		detail, err := d.Wallet.GetDetail(ctx, parts[2], id)
		if err != nil {
			edit("Detail tidak ditemukan.", ActivityDetailBack())
			return
		}
		edit(formatActivityDetail(detail), ActivityDetailBack())
	}
}

func handleWalletInput(ctx context.Context, b *bot.Bot, chatID, telegramID int64, text string, st Session, d *Deps) bool {
	if d.Wallet == nil || d.Q == nil || d.Ledger == nil {
		reply(ctx, b, chatID, "Wallet belum siap.")
		return true
	}
	switch st.Step {
	case StepAdminStockDepositUnits:
		units, err := parseWalletUnits(text)
		if err != nil {
			reply(ctx, b, chatID, "Units harus angka positif. Coba lagi.")
			return true
		}
		if d.Session != nil {
			d.Session.Set(telegramID, Session{Step: StepAdminStockDepositRef, Coin: st.Coin, Units: units.String(), HotAddr: st.HotAddr})
		}
		reply(ctx, b, chatID, "Ketik reference mock (MOCK-...) atau reference dengan format network yang sesuai:")
		return true
	case StepAdminStockDepositRef:
		ref := strings.TrimSpace(text)
		if err := validateWalletTxRef(st.Coin, ref); err != nil {
			reply(ctx, b, chatID, "Reference tidak valid. Gunakan MOCK-... atau format network yang sesuai. Coba lagi.")
			return true
		}
		units, err := parseWalletUnits(st.Units)
		if err != nil || st.HotAddr == "" {
			reply(ctx, b, chatID, "Sesi deposit tidak valid. Ulangi dari Wallet.")
			return true
		}
		row, err := d.Q.CreatePendingAdminStockDeposit(ctx, db.CreatePendingAdminStockDepositParams{AdminTelegramID: telegramID, Asset: st.Coin, Network: adminwallet.NetworkFor(st.Coin), Units: units, ToAddress: st.HotAddr, TxRef: ref})
		if err != nil {
			existing, lookupErr := d.Q.GetAdminStockDepositByRef(ctx, ref)
			if lookupErr == nil && existing.AdminTelegramID == telegramID {
				switch existing.Status {
				case "recorded_mock":
					balance, _, balanceErr := d.Ledger.FloatStatus(ctx, existing.Asset)
					if balanceErr == nil && existing.JournalID.Valid {
						reply(ctx, b, chatID, fmt.Sprintf("Deposit stok ini sudah pernah dicatat.\nJurnal: #%d\nSaldo saat ini: %s", existing.JournalID.Int64, balance.String()))
						return true
					}
					reply(ctx, b, chatID, "Deposit stok ini sudah pernah dicatat.")
					return true
				case "cancelled":
					reply(ctx, b, chatID, "Reference ini sebelumnya sudah dibatalkan dan tidak dapat digunakan ulang. Buat reference baru untuk pencatatan baru.")
					return true
				}
			}
			reply(ctx, b, chatID, "Reference sudah dipakai atau gagal membuat preview. Gunakan reference lain.")
			return true
		}
		before, _, err := d.Ledger.FloatStatus(ctx, st.Coin)
		if err != nil {
			_, _ = d.Q.MarkAdminStockDepositCancelled(ctx, row.ID)
			reply(ctx, b, chatID, "Preview gagal membaca saldo: "+err.Error())
			return true
		}
		if d.Session != nil {
			d.Session.Set(telegramID, Session{Coin: st.Coin, Units: units.String(), HotAddr: st.HotAddr, TxRef: ref, DepositID: row.ID})
		}
		replyInline(ctx, b, chatID, formatDepositPreview(st.Coin, adminwallet.NetworkFor(st.Coin), units, st.HotAddr, ref, before, before.Add(units)), WalletDepositConfirm(row.ID))
		return true
	}
	return false
}

func handleWalletConfirm(ctx context.Context, b *bot.Bot, chatID, telegramID, id int64, d *Deps) {
	row, err := d.Q.GetAdminStockDepositByID(ctx, id)
	if err != nil || row.AdminTelegramID != telegramID {
		reply(ctx, b, chatID, "Deposit tidak ditemukan.")
		return
	}
	if row.Status == "recorded_mock" {
		replyInline(ctx, b, chatID, fmt.Sprintf("Deposit #%d sudah diproses (SIMULASI).", id), activityDetailButton(id))
		return
	}
	if row.Status == "cancelled" {
		reply(ctx, b, chatID, "Reference ini sebelumnya sudah dibatalkan dan tidak dapat digunakan ulang. Buat reference baru untuk pencatatan baru.")
		return
	}
	if row.Status != "pending_confirm" {
		reply(ctx, b, chatID, "Deposit tidak dapat diproses pada status ini.")
		return
	}
	if !validWalletAsset(row.Asset) || row.Units.LessThanOrEqual(decimal.Zero) || strings.TrimSpace(row.ToAddress) == "" || validateWalletTxRef(row.Asset, row.TxRef) != nil {
		reply(ctx, b, chatID, "Data deposit tidak valid.")
		return
	}
	res, err := d.Ledger.RecordAdminStockDeposit(ctx, ledger.RecordStockDepositInput{AdminTelegramID: telegramID, Asset: row.Asset, Units: row.Units, ToAddress: row.ToAddress, TxRef: row.TxRef})
	if err != nil {
		reply(ctx, b, chatID, walletConfirmCopy(err))
		return
	}
	if d.Session != nil {
		d.Session.Clear(telegramID)
	}
	if res.AlreadyDone {
		replyInline(ctx, b, chatID, fmt.Sprintf("Deposit #%d sudah diproses (SIMULASI).", res.DepositID), activityDetailButton(res.DepositID))
		return
	}
	replyInline(ctx, b, chatID, fmt.Sprintf("Deposit stok tercatat (SIMULASI)\nDeposit ID: %d\nJournal ID: %d\nSaldo jurnal %s: %s", res.DepositID, res.JournalID, res.Asset, res.Balance.String()), activityDetailButton(res.DepositID))
}

func validWalletAsset(asset string) bool {
	asset = strings.ToUpper(asset)
	return asset == "BNB" || asset == "ETH" || asset == "SOL"
}
func validateWalletDepositStart(asset, hot string) error {
	if !validWalletAsset(asset) {
		return fmt.Errorf("aset tidak didukung")
	}
	if strings.TrimSpace(hot) == "" {
		return fmt.Errorf("alamat hot belum diatur")
	}
	return nil
}
func validActivityKind(kind string) bool {
	return kind == adminwallet.KindStockIn || kind == adminwallet.KindPayoutOut || kind == adminwallet.KindUserDepositIn
}
func parseWalletUnits(raw string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil || value.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("units harus positif")
	}
	return value, nil
}
func validateWalletTxRef(asset, ref string) error {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "MOCK-") && len(strings.TrimPrefix(ref, "MOCK-")) > 0 {
		return nil
	}
	return chain.ValidateAddress(asset, ref)
}

func formatDepositPreview(asset, network string, units decimal.Decimal, hot, ref string, before, after decimal.Decimal) string {
	return fmt.Sprintf("Preview Deposit Stok (SIMULASI)\n\nAset/network: %s / %s\nUnits: %s\nAlamat hot: %s\nReference mock: %s\nSaldo jurnal sebelum: %s\nSaldo jurnal sesudah: %s\n\nBelum ada jurnal sebelum tombol Catat Deposit.", asset, network, units.String(), hot, ref, before.String(), after.String())
}

func activityKeyboard(items []adminwallet.ActivityItem, limit int) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(items)+2)
	for _, item := range items {
		rows = append(rows, []models.InlineKeyboardButton{{Text: fmt.Sprintf("Detail %s #%d", item.Kind, item.ID), CallbackData: fmt.Sprintf("a:act:%s:%d", item.Kind, item.ID)}})
	}
	rows = append(rows, AdminActivityNav(limit).InlineKeyboard...)
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func activityDetailButton(id int64) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "Lihat Detail", CallbackData: fmt.Sprintf("a:act:%s:%d", adminwallet.KindStockIn, id)}}}}
}

func replyInline(ctx context.Context, b *bot.Bot, chatID int64, text string, markup *models.InlineKeyboardMarkup) {
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup})
}
