// Flow T1-button: pilih coin → input nominal → quote → Confirm/Batal.
// Dipanggil dari dispatcher (pesan) dan handleCallback (inline t:*).
package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pikipici/crypto-station/internal/chain"
	"github.com/pikipici/crypto-station/internal/db"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
)

// handleFlowCallback: inline t:buy:SOL / t:sell:ETH / t:sellall:SOL / t:cancel.
func handleFlowCallback(ctx context.Context, b *bot.Bot, cb *models.CallbackQuery, d *Deps) {
	uid, blocked := userID(ctx, d, cb.From.ID)
	if blocked == "blocked" || uid == 0 {
		return
	}
	chatID := cb.Message.Message.Chat.ID
	msgID := cb.Message.Message.ID
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		return
	}
	switch parts[1] {
	case "cancel":
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
		_ = d.Q.ExpireUserQuotes(ctx, uid)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Dibatalkan."})
		return
	case "addr":
		// Fase 8: daftar alamat — t:addr:SOL → session await addr → user ketik alamat.
		if len(parts) != 3 {
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			return
		}
		if d.Session != nil {
			d.Session.Set(cb.From.ID, Session{Step: StepAwaitAddr, Coin: coin})
		}
		net, _ := chain.NetworkFor(coin)
		hint := "0x + 40 hex"
		if net == "SOL" {
			hint = "base58 32–44 char"
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID: chatID, MessageID: msgID,
			Text: fmt.Sprintf("Ketik alamat %s lu (%s, %s):", coin, net, hint),
		})
		return
	case "depsim":
		// Fase 9: simulasi deposit masuk — t:depsim:SOL:50000.
		// Units mock = nominal IDR / bid (arah kebalik payout).
		if len(parts) != 4 {
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			return
		}
		amount, ok := ParseNominal(parts[3])
		if !ok || amount <= 0 {
			return
		}
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
		snap, err := d.Price.Snapshot(coin)
		if err != nil {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Harga basi (" + errMsg(err) + "), coba lagi."})
			return
		}
		// Units = amount / bid (mock kirim masuk senilai IDR itu).
		units := decimal.NewFromInt(amount).Div(snap.Bid).Round(18)
		txHash := fmt.Sprintf("MOCK-DEP-%d-%d", uid, time.Now().Unix())
		dep, err := d.Ledger.MockDeposit(ctx, uid, coin, units, txHash, snap)
		if err != nil {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Deposit gagal: " + err.Error()})
			return
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID,
			Text: fmt.Sprintf("Deposit masuk (SIMULASI): %s %s\ntx %s\nIDR +%s. Cek /saldo.", dep.Units.String(), dep.Asset, txHash, formatIDR(decimal.NewFromInt(dep.AmountIDR)))})
		return
	case "buy", "sell":
		if len(parts) != 3 {
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			return
		}
		if d.Session != nil {
			step := StepAwaitBuy
			if parts[1] == "sell" {
				step = StepAwaitSell
			}
			d.Session.Set(cb.From.ID, Session{Step: step, Coin: coin})
		}
		if parts[1] == "sell" {
			// Fase 9: jual = deposit on-chain (bukan nominal/ALL).
			// Tampilkan alamat deposit + tombol simulasi kirim masuk.
			depAddr, _ := d.Set.Get("deposit_address_" + strings.ToLower(coin))
			if depAddr == "" {
				depAddr = "MOCK-DEPOSIT-" + coin
			}
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID: chatID, MessageID: msgID,
				Text:        fmt.Sprintf("Jual %s:\n1. Kirim %s dari wallet lu ke:\n%s\n2. Tap tombol di bawah buat simulasi kirim masuk (sandbox).", coin, coin, depAddr),
				ReplyMarkup: DepositSimPicker(coin),
			})
			return
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID: chatID, MessageID: msgID,
			Text: fmt.Sprintf("Mau beli %s berapa? Ketik angka IDR (min %s). Contoh: 50000", coin, d.Set.MustString("min_buy_idr")),
		})
		return
	case "sellall":
		if len(parts) != 3 {
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			return
		}
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
		// Edit pesan picker biar tidak bisa ditap 2x, lalu kirim quote baru.
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Siapin quote jual SEMUA " + coin + "..."})
		sellAllFlow(ctx, b, chatID, cb.From.ID, uid, coin, d)
		return
	}
}

// handleAmountInput: user lagi await nominal/alamat dan mengetik.
// st = session fresh (sudah dicek timeout oleh caller).
func handleAmountInput(ctx context.Context, b *bot.Bot, chatID, telegramID int64, text string, st Session, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	// Fase 8: await alamat — validasi + simpan (bukan parse nominal).
	if st.Step == StepAwaitAddr {
		addr := strings.TrimSpace(text)
		if err := chain.ValidateAddress(st.Coin, addr); err != nil {
			reply(ctx, b, chatID, "Alamat tidak valid: "+err.Error()+"\nCoba lagi atau tap Batal.")
			return
		}
		if d.Session != nil {
			d.Session.Clear(telegramID)
		}
		net, _ := chain.NetworkFor(st.Coin)
		row, err := d.Q.UpsertWalletAddress(ctx, db.UpsertWalletAddressParams{UserID: uid, Asset: st.Coin, Address: addr, Network: net})
		if err != nil {
			reply(ctx, b, chatID, "Gagal simpan alamat: "+err.Error())
			return
		}
		_ = d.Admin.Audit(ctx, telegramID, "alamat", uid, true, st.Coin+"=(kosong)", st.Coin+"="+row.Address)
		reply(ctx, b, chatID, fmt.Sprintf("Alamat %s (%s) tersimpan.\nBeli sekarang otomatis dikirim ke sini.", st.Coin, net))
		return
	}
	amount, ok := ParseNominal(text)
	if !ok {
		reply(ctx, b, chatID, "Nominal nggak kebaca. Ketik angka IDR aja, contoh: 50000")
		return
	}
	if amount < d.Set.Int64("min_buy_idr") {
		reply(ctx, b, chatID, fmt.Sprintf("Minimum Rp%s. Coba lagi atau tap Batal.", d.Set.MustString("min_buy_idr")))
		return
	}
	side := "buy"
	if st.Step == StepAwaitSell {
		side = "sell"
	}
	if d.Session != nil {
		d.Session.Clear(telegramID)
	}
	// Fase 8 transit: buy via quoteFlowBuy (alamat + fee + kirim).
	if side == "buy" {
		quoteFlowBuy(ctx, b, chatID, telegramID, uid, st.Coin, amount, d)
		return
	}
	quoteFlow(ctx, b, chatID, telegramID, uid, side, st.Coin, amount, d)
}

// successHint: teks sukses trade pakai hint button (slash tetap disebut sekali).
func successHint(verb, units, asset string, amountIDR int64) string {
	return fmt.Sprintf("%s sukses (SIMULASI): %s %s = Rp%s.\nCek Saldo, riwayat di Riwayat.", verb, units, asset, formatIDR(decimal.NewFromInt(amountIDR)))
}

// alamatCmd: /alamat BNB 0x... — simpan 1 alamat per coin (validasi format).
func alamatCmd(ctx context.Context, b *bot.Bot, chatID, telegramID int64, rest []string, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	if len(rest) < 2 {
		// T1-button Fase 8: tap Alamat tanpa arg = picker coin (bukan list teks).
		// List teks tetap via /alamat list.
		if len(rest) == 1 && strings.ToLower(rest[0]) == "list" {
			rows, err := d.Q.ListWalletAddresses(ctx, uid)
			if err != nil {
				reply(ctx, b, chatID, "Gagal baca alamat, coba lagi.")
				return
			}
			if len(rows) == 0 {
				reply(ctx, b, chatID, "Belum ada alamat. Tap Alamat → pilih coin → ketik wallet lu.")
				return
			}
			var sb strings.Builder
			sb.WriteString("Alamat tersimpan:\n")
			for _, r := range rows {
				fmt.Fprintf(&sb, "%s (%s): %s\n", r.Asset, r.Network, r.Address)
			}
			reply(ctx, b, chatID, strings.TrimRight(sb.String(), "\n"))
			return
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Daftarin alamat wallet apa?", ReplyMarkup: AddrCoinPicker()})
		return
	}
	coin := strings.ToUpper(strings.TrimSpace(rest[0]))
	addr := strings.TrimSpace(rest[1])
	if err := chain.ValidateAddress(coin, addr); err != nil {
		reply(ctx, b, chatID, "Alamat tidak valid: "+err.Error())
		return
	}
	net, _ := chain.NetworkFor(coin)
	row, err := d.Q.UpsertWalletAddress(ctx, db.UpsertWalletAddressParams{UserID: uid, Asset: coin, Address: addr, Network: net})
	if err != nil {
		reply(ctx, b, chatID, "Gagal simpan alamat: "+err.Error())
		return
	}
	_ = d.Admin.Audit(ctx, telegramID, "alamat", uid, true, coin+"=(kosong)", coin+"="+row.Address)
	reply(ctx, b, chatID, fmt.Sprintf("Alamat %s (%s) tersimpan: %s\nBeli sekarang otomatis dikirim ke sini.", coin, net, row.Address))
}

// statusCmd: /status — payout terakhir user (txhash + status).
func statusCmd(ctx context.Context, b *bot.Bot, chatID, telegramID int64, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	rows, err := d.Q.RecentPayoutsByUser(ctx, db.RecentPayoutsByUserParams{UserID: uid, Limit: 5})
	if err != nil {
		reply(ctx, b, chatID, "Gagal baca status, coba lagi.")
		return
	}
	if len(rows) == 0 {
		reply(ctx, b, chatID, "Belum ada pengiriman. Beli dulu, koin otomatis dikirim.")
		return
	}
	var sb strings.Builder
	sb.WriteString("Pengiriman terakhir:\n")
	for _, r := range rows {
		tx := "(tanpa tx)"
		if r.TxHash.Valid {
			tx = r.TxHash.String
		}
		fmt.Fprintf(&sb, "%s %s → %s\ntx %s (%s)\n", r.Asset, r.SentUnits.String(), shortAddr(r.ToAddress), tx, r.Status)
	}
	reply(ctx, b, chatID, strings.TrimRight(sb.String(), "\n"))
}

func shortAddr(a string) string {
	if len(a) <= 14 {
		return a
	}
	return a[:8] + "..." + a[len(a)-4:]
}

// cekdepositCmd: /cekdeposit — deposit terakhir user.
func cekdepositCmd(ctx context.Context, b *bot.Bot, chatID, telegramID int64, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	rows, err := d.Q.RecentDepositsByUser(ctx, db.RecentDepositsByUserParams{UserID: uid, Limit: 5})
	if err != nil {
		reply(ctx, b, chatID, "Gagal baca deposit, coba lagi.")
		return
	}
	if len(rows) == 0 {
		reply(ctx, b, chatID, "Belum ada deposit. Tap Jual → pilih coin → kirim ke alamat deposit.")
		return
	}
	var sb strings.Builder
	sb.WriteString("Deposit terakhir:\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "%s %s → IDR +%s (%s)\ntx %s\n", r.Asset, r.Units.String(), formatIDR(decimal.NewFromInt(r.CreditedIdr)), r.Status, r.TxHash)
	}
	reply(ctx, b, chatID, strings.TrimRight(sb.String(), "\n"))
}

// quoteFlowBuy: buy transit Fase 8 — wajib alamat, fee mock, quote 3 baris,
// confirm → ExecuteTrade → payout mock → txhash.
func quoteFlowBuy(ctx context.Context, b *bot.Bot, chatID, telegramID, uid int64, asset string, amount int64, d *Deps) {
	// Wajib alamat tersimpan.
	addrRow, err := d.Q.GetWalletAddress(ctx, db.GetWalletAddressParams{UserID: uid, Asset: asset})
	if err != nil {
		reply(ctx, b, chatID, "Belum ada alamat "+asset+". Daftarin dulu: /alamat "+asset+" <wallet lu>")
		return
	}
	if err := d.Risk.CheckBeforeQuote(ctx, asset, "buy"); err != nil {
		_ = d.Risk.RecordEvent(ctx, uid, true, telegramID, "quote_rejected", "buy "+asset+": "+err.Error())
		reply(ctx, b, chatID, "Tidak bisa kasih quote: "+riskMsg(err))
		return
	}
	snap, err := d.Price.Snapshot(asset)
	if err != nil {
		reply(ctx, b, chatID, "Harga basi ("+errMsg(err)+"), coba lagi.")
		return
	}
	fee, err := d.Chain.EstimateFee(asset)
	if err != nil {
		reply(ctx, b, chatID, "Gagal estimasi fee: "+err.Error())
		return
	}
	qr, err := d.Ledger.CreateQuoteBuy(ctx, uid, asset, amount, fee, addrRow.Address, snap)
	if err != nil {
		reply(ctx, b, chatID, "Gagal bikin quote: "+err.Error())
		return
	}
	net, _ := chain.NetworkFor(asset)
	lockSecs := d.Set.Int64("quote_lock_seconds")
	text := fmt.Sprintf("Beli %s Rp%s\nDapat: %s %s (fee %s)\nTerkirim ke %s (%s)\nHarga lock %d detik. Confirm?",
		asset, formatIDR(decimal.NewFromInt(amount)), qr.SentUnits.String(), asset, qr.FeeUnits.String(),
		shortAddr(qr.ToAddress), net, lockSecs)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: QuoteKeyboard(qr.ID)})
}
