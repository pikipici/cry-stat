// Payout lifecycle user-facing copy (revisi V.1 A24).
// Aturan: refund hanya diklaim bila jurnal posted; unknown = jangan ulang.
package bot

// PayoutStatusID: status payout → copy Indonesia.
// failed tanpa refund posted = belum bisa klaim kembali (jujur, A24).
func PayoutStatusID(status string, refunded bool) string {
	switch status {
	case "queued":
		return "Menunggu diproses"
	case "broadcasting":
		return "Sedang dikirim"
	case "confirmed":
		return "Berhasil"
	case "failed":
		if refunded {
			return "Gagal — dana dikembalikan"
		}
		return "Gagal — hubungi admin"
	case "unknown":
		return "Perlu pengecekan — jangan ulang transaksi"
	default:
		return status
	}
}

// DepositStatusID: status deposit user → copy Indonesia.
func DepositStatusID(status string) string {
	switch status {
	case "pending":
		return "Menunggu konfirmasi"
	case "credited":
		return "Masuk"
	case "manual":
		return "Perlu verifikasi admin"
	default:
		return status
	}
}
