// Phase 7 D6 (revisi V.1): lifecycle copy + session expired unit tests.
// Pure unit — tanpa DB, tanpa network.
package bot

import (
	"testing"
	"time"
)

func TestPayoutStatusID(t *testing.T) {
	cases := []struct {
		status   string
		refunded bool
		want     string
	}{
		{"queued", false, "Menunggu diproses"},
		{"broadcasting", false, "Sedang dikirim"},
		{"confirmed", false, "Berhasil"},
		{"failed", true, "Gagal — dana dikembalikan"},
		{"failed", false, "Gagal — hubungi admin"},
		{"unknown", false, "Perlu pengecekan — jangan ulang transaksi"},
	}
	for _, c := range cases {
		if got := PayoutStatusID(c.status, c.refunded); got != c.want {
			t.Errorf("PayoutStatusID(%q,%v) = %q, mau %q", c.status, c.refunded, got, c.want)
		}
	}
}

func TestDepositStatusID(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"pending", "Menunggu konfirmasi"},
		{"credited", "Masuk"},
		{"manual", "Perlu verifikasi admin"},
	} {
		if got := DepositStatusID(c.in); got != c.want {
			t.Errorf("DepositStatusID(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestExpiredCopy(t *testing.T) {
	for _, c := range []struct {
		step Step
		want string
	}{
		{StepAwaitBuy, "Sesi beli sudah habis. Tekan Beli untuk mulai lagi."},
		{StepAwaitAddr, "Sesi alamat sudah habis. Buka Menu → Alamat."},
		{StepAdminBroadcast, "Sesi broadcast sudah habis. Buka Admin → Broadcast lagi."},
		{StepAdminSpread, "Sesi admin sudah habis. Buka Admin lagi."},
		{StepIdle, ""},
	} {
		if got := ExpiredCopy(Session{Step: c.step}); got != c.want {
			t.Errorf("ExpiredCopy(%q) = %q, mau %q", c.step, got, c.want)
		}
	}
}

func TestSessionExpiredTracking(t *testing.T) {
	s := NewSessions()
	// Fresh = tidak tercatat expired.
	s.Set(1, Session{Step: StepAwaitBuy, Coin: "SOL"})
	if got := s.Get(1); got.Step != StepAwaitBuy {
		t.Fatalf("fresh harus kembali, dapat %+v", got)
	}
	if got := s.ExpiredStep(1); got.Step != StepIdle {
		t.Fatalf("belum expired, ExpiredStep harus idle, dapat %+v", got)
	}
	// Kadaluarsa = Get idle + tercatat sekali.
	s.Set(2, Session{Step: StepAwaitAddr, Coin: "SOL"})
	s.mu.Lock()
	st := s.m[2]
	st.UpdatedAt = time.Now().Add(-SessionTimeout - time.Second)
	s.m[2] = st
	s.mu.Unlock()
	if got := s.Get(2); got.Step != StepIdle {
		t.Fatalf("expired harus idle, dapat %+v", got)
	}
	exp := s.ExpiredStep(2)
	if exp.Step != StepAwaitAddr {
		t.Fatalf("catatan expired harus StepAwaitAddr, dapat %+v", exp)
	}
	// Sekali baca = habis.
	if got := s.ExpiredStep(2); got.Step != StepIdle {
		t.Fatalf("ExpiredStep kedua harus idle, dapat %+v", got)
	}
	// Clear menghapus catatan juga.
	s.Set(3, Session{Step: StepAwaitBuy, Coin: "SOL"})
	s.mu.Lock()
	st3 := s.m[3]
	st3.UpdatedAt = time.Now().Add(-SessionTimeout - time.Second)
	s.m[3] = st3
	s.mu.Unlock()
	_ = s.Get(3)
	s.Clear(3)
	if got := s.ExpiredStep(3); got.Step != StepIdle {
		t.Fatalf("Clear harus hapus catatan, dapat %+v", got)
	}
}
