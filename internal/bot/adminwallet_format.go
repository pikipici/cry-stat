package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/pikipici/crypto-station/internal/adminwallet"
)

func formatAssetSummary(items []adminwallet.AssetSummary) string {
	var sb strings.Builder
	sb.WriteString("Ringkasan Aset (SIMULASI)\n\n")
	for _, item := range items {
		status := "AMAN ✅"
		if item.Status != "AMAN" {
			status = "TIPIS ⚠️"
		}
		hot := item.HotAddress
		if hot == "" {
			hot = "belum diatur"
		} else {
			hot = shortAddress(hot)
		}
		fmt.Fprintf(&sb, "%s (%s) — %s\nSaldo jurnal: %s\nFloat minimum: %s\nAlamat hot: %s\n",
			item.Asset, item.Network, status, item.Balance.String(), item.Minimum.String(), hot)
		if item.EstimateOK {
			fmt.Fprintf(&sb, "Estimasi IDR: Rp%d\n", item.EstimatedIDR)
		} else {
			sb.WriteString("Estimasi IDR: tidak tersedia (harga basi)\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Harga/estimasi: hanya saat harga masih fresh.\nCatatan: saldo di atas berasal dari jurnal sandbox, bukan saldo blockchain.")
	return sb.String()
}

func formatActivityList(items []adminwallet.ActivityItem, limit int) string {
	if len(items) == 0 {
		return "Transaksi terakhir (SIMULASI)\n\nBelum ada aktivitas."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Transaksi terakhir (SIMULASI, %d)\n\n", limit)
	for i, item := range items {
		emoji := map[string]string{adminwallet.KindStockIn: "➕", adminwallet.KindPayoutOut: "➖", adminwallet.KindUserDepositIn: "📥"}[item.Kind]
		fmt.Fprintf(&sb, "%d. %s %s #%d — %s %s\n   %s | Ref: %s | %s\n", i+1, emoji, item.Kind, item.ID,
			item.Units.String(), item.Asset, item.Status, shortTxRef(item.TxRef), formatWIB(item.CreatedAt))
	}
	return guardTelegramList(sb.String(), len(items))
}

func formatActivityDetail(detail adminwallet.ActivityDetail) string {
	i := detail.Item
	var sb strings.Builder
	fmt.Fprintf(&sb, "Detail Transaksi (SIMULASI)\n\nJenis: %s\nID: %d\nAset/network: %s / %s\nUnits: %s\nStatus: %s\nReference mock: %s\nWaktu: %s\n",
		i.Kind, i.ID, i.Asset, i.Network, i.Units.String(), i.Status, i.TxRef, formatWIB(i.CreatedAt))
	if detail.JournalID != nil {
		fmt.Fprintf(&sb, "Journal ID: %d\n", *detail.JournalID)
	}
	if detail.AdminTelegramID != nil {
		fmt.Fprintf(&sb, "Admin Telegram ID: %d\n", *detail.AdminTelegramID)
	}
	if i.UserID != nil {
		fmt.Fprintf(&sb, "User ID: %d\n", *i.UserID)
	}
	if detail.FromAddress != "" {
		fmt.Fprintf(&sb, "Alamat asal: %s\n", detail.FromAddress)
	}
	if detail.ToAddress != "" {
		fmt.Fprintf(&sb, "Alamat tujuan: %s\n", detail.ToAddress)
	}
	if detail.FeeUnits != nil {
		fmt.Fprintf(&sb, "Fee: %s\n", detail.FeeUnits.String())
	}
	if detail.SentUnits != nil {
		fmt.Fprintf(&sb, "Terkirim: %s\n", detail.SentUnits.String())
	}
	if detail.CreditedIDR != nil {
		fmt.Fprintf(&sb, "Kredit IDR: Rp%d\n", *detail.CreditedIDR)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func shortAddress(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:6] + "..." + value[len(value)-4:]
}

func shortTxRef(value string) string {
	if len(value) <= 16 {
		return value
	}
	return value[:12] + "..."
}

func formatWIB(value time.Time) string {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return value.UTC().Format("02-01-2006 15:04 UTC")
	}
	return value.In(loc).Format("02-01-2006 15:04 WIB")
}

// guardTelegramList memberi keluaran deterministik bila suatu hari detail list membesar.
func guardTelegramList(value string, itemCount int) string {
	if len(value) <= 4000 {
		return value
	}
	lines := strings.Split(value, "\n")
	keep := 2 + min(itemCount, 5)*2
	if keep > len(lines) {
		keep = len(lines)
	}
	return strings.Join(lines[:keep], "\n") + "\n\nDaftar dipotong agar muat di Telegram."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
