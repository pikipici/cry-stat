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
	"github.com/pikipici/crypto-station/internal/price"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
)

// handleFlowCallback: inline t:buy:SOL / t:sell:ETH / t:sellall:SOL / t:cancel.
func handleFlowCallback(ctx context.Context, b *bot.Bot, cb *models.CallbackQuery, d *Deps) {
	// Phase 2 (A12/A13): guard dulu — nil-safe + private + user.
	// Spinner sudah dijawab caller (handleCallback).
	g, deny, ok := GuardCallback(ctx, d, cb)
	if !ok {
		if deny != "" && cb != nil && cb.Message.Message != nil {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: cb.Message.Message.Chat.ID, MessageID: cb.Message.Message.ID, Text: deny})
		}
		return
	}
	uid := g.UID
	chatID := g.ChatID
	msgID := g.MsgID
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Tombol ini sudah kedaluwarsa. Mulai ulang proses."})
		return
	}
	// Phase 2 (A13): malformed tanpa mutasi tapi spinner + feedback jelas.
	stale := func() {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Tombol ini sudah kedaluwarsa. Mulai ulang proses."})
	}
	switch parts[1] {
	case "cancel":
		// Phase 2 (A8): navigation-cancel — clear session SAJA, quote tidak disentuh.
		if d.Session != nil {
			d.Session.Clear(cb.From.ID)
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Proses dibatalkan."})
		return
	case "addr":
		// Fase 8: daftar alamat — t:addr:SOL → session await addr → user ketik alamat.
		if len(parts) != 3 {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Tombol ini sudah kedaluwarsa. Mulai ulang proses."})
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: msgID, Text: "Tombol ini sudah kedaluwarsa. Mulai ulang proses."})
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
			stale()
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			stale()
			return
		}
		amount, ok := ParseNominal(parts[3])
		if !ok || amount <= 0 {
			stale()
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
			Text: fmt.Sprintf("Deposit masuk (SIMULASI): %s %s\ntx %s\nIDR +%s (neto dari senilai Rp%s). Cek /saldo.", dep.Units.String(), dep.Asset, txHash, formatIDR(decimal.NewFromInt(dep.AmountIDR)), formatIDR(decimal.NewFromInt(amount)))})
		return
	case "buy", "sell":
		if len(parts) != 3 {
			stale()
			return
		}
		coin := strings.ToUpper(parts[2])
		if coin != "BNB" && coin != "ETH" && coin != "SOL" {
			stale()
			return
		}
		if parts[1] == "sell" {
			// Phase 6: satu presenter deposit dipakai tombol + slash.
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID: chatID, MessageID: msgID,
				Text: sellDepositText(coin, d), ReplyMarkup: DepositSimPicker(coin),
			})
			return
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID: chatID, MessageID: msgID,
			Text: fmt.Sprintf("Mau beli %s berapa? Ketik angka IDR (min %s). Contoh: 50000", coin, d.Set.MustString("min_buy_idr")),
		})
		if d.Session != nil {
			d.Session.Set(cb.From.ID, Session{Step: StepAwaitBuy, Coin: coin})
		}
		return
	case "sellall":
		// Phase 6 (A10): sell-bandar mati total. Forged callback = stale.
		stale()
		return
	}
}

// sellDepositText: copy satu-satunya layar jual (Phase 6 A11).
func sellDepositText(coin string, d *Deps) string {
	depAddr, _ := d.Set.Get("deposit_address_" + strings.ToLower(coin))
	if depAddr == "" {
		depAddr = "MOCK-DEPOSIT-" + coin
	}
	return fmt.Sprintf("Jual %s (SIMULASI):\n1. Kirim %s dari wallet lu ke:\n%s\n2. Tap tombol di bawah buat simulasi kirim masuk (sandbox).", coin, coin, depAddr)
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
		// Phase 5 (A17): replace atomik + audit actual before/after di ledger.
		if _, _, err := d.Ledger.ReplaceWalletAddress(ctx, uid, telegramID, st.Coin, addr, net); err != nil {
			reply(ctx, b, chatID, "Gagal simpan alamat: "+err.Error())
			return
		}
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
	// Phase 6: satu-satunya quote user adalah buy. Session sell basi diarahkan
	// ke deposit dan return — tidak ada path sell ke quote bandar.
	if st.Step == StepAwaitSell {
		// Phase 6 (A10/A11): sell-bandar mati. Session sell basi (dibuat
		// sebelum Phase 6) tidak boleh jadi quote bandar — arahin deposit.
		if d.Session != nil {
			d.Session.Clear(telegramID)
		}
		if !price.ValidAsset(st.Coin) {
			reply(ctx, b, chatID, "Jual sekarang via deposit: tap Jual → pilih coin.")
			return
		}
		reply(ctx, b, chatID, fmt.Sprintf("Jual %s sekarang via deposit (SIMULASI): tap Jual → pilih %s → kirim ke alamat deposit.", st.Coin, st.Coin))
		return
	}
	if d.Session != nil {
		d.Session.Clear(telegramID)
	}
	// Fase 8 transit: buy via quoteFlowBuy (alamat + fee + kirim).
	// Phase 6: satu-satunya quote user adalah buy. Sell = deposit flow.
	quoteFlowBuy(ctx, b, chatID, telegramID, uid, st.Coin, amount, d)
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
	// Phase 5 (A17): replace atomik + audit actual before/after di ledger.
	if _, _, err := d.Ledger.ReplaceWalletAddress(ctx, uid, telegramID, coin, addr, net); err != nil {
		reply(ctx, b, chatID, "Gagal simpan alamat: "+err.Error())
		return
	}
	reply(ctx, b, chatID, fmt.Sprintf("Alamat %s (%s) tersimpan: %s\nBeli sekarang otomatis dikirim ke sini.", coin, net, addr))
}

// statusCmd: /status — payout + deposit terakhir user (1 pantau semua, Fase 9).
func statusCmd(ctx context.Context, b *bot.Bot, chatID, telegramID int64, d *Deps) {
	uid, need := needUser(ctx, telegramID, d)
	if need != "" {
		reply(ctx, b, chatID, need)
		return
	}
	var sb strings.Builder
	sb.WriteString("Status terakhir:\n")
	// Phase 7 (A20/A24): bedakan error vs empty vs partial; status copy ID.
	prows, perr := d.Q.RecentPayoutsByUser(ctx, db.RecentPayoutsByUserParams{UserID: uid, Limit: 3})
	if perr == nil && len(prows) > 0 {
		sb.WriteString("Pengiriman terbaru:\n")
		for _, r := range prows {
			tx := "(tanpa tx)"
			if r.TxHash.Valid {
				tx = r.TxHash.String
			}
			fmt.Fprintf(&sb, "%s %s → %s\ntx %s (%s)\n", r.Asset, r.SentUnits.String(), shortAddr(r.ToAddress), tx, PayoutStatusID(r.Status, r.RefundJournalID.Valid))
		}
	}
	drows, derr := d.Q.RecentDepositsByUser(ctx, db.RecentDepositsByUserParams{UserID: pgtype.Int8{Int64: uid, Valid: true}, Limit: 3})
	if derr == nil && len(drows) > 0 {
		sb.WriteString("Deposit masuk:\n")
		for _, r := range drows {
			fmt.Fprintf(&sb, "%s %s → IDR +%s (%s)\ntx %s\n", r.Asset, r.Units.String(), formatIDR(decimal.NewFromInt(r.CreditedIdr)), DepositStatusID(r.Status), r.TxHash)
		}
	}
	if perr != nil && derr != nil {
		reply(ctx, b, chatID, "Status sedang tidak tersedia. Coba lagi.")
		return
	}
	if (perr != nil || len(prows) == 0) && (derr != nil || len(drows) == 0) {
		reply(ctx, b, chatID, "Belum ada pengiriman/deposit. Beli dulu (otomatis dikirim), jual via deposit.")
		return
	}
	if perr != nil || derr != nil {
		sb.WriteString("(sebagian data tidak tersedia)\n")
	}
	reply(ctx, b, chatID, strings.TrimRight(sb.String(), "\n"))
}

func shortAddr(a string) string {
	if len(a) <= 14 {
		return a
	}
	return a[:8] + "..." + a[len(a)-4:]
}

// shortTx: hash/reference pendek untuk list (detail penuh di layar detail).
func shortTx(tx string) string {
	if len(tx) <= 16 {
		return tx
	}
	return tx[:10] + "..." + tx[len(tx)-6:]
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
	// Phase 7 (A19): quote aman — bruto/fee/net, fee-IDR, alamat penuh,
	// warning irreversible. Golden copy per coin di test bot.
	feeIDR := qr.FeeUnits.Mul(qr.Price).Round(0)
	text := fmt.Sprintf("Konfirmasi Pembelian — SIMULASI\n\nBayar: Rp%s\nDapat sebelum biaya: %s %s\nBiaya jaringan: %s %s (±Rp%s)\nDiterima: %s %s\nNetwork: %s\nAlamat tujuan:\n%s\nHarga berlaku: %d detik\n\n⚠️ Pastikan alamat dan network benar. Pengiriman crypto tidak dapat dibatalkan.",
		formatIDR(decimal.NewFromInt(amount)), qr.Units.String(), asset,
		qr.FeeUnits.String(), asset, formatIDR(feeIDR),
		qr.SentUnits.String(), asset, net, qr.ToAddress, lockSecs)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: QuoteKeyboard(qr.ID)})
}
