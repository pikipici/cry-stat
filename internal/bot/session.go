// Session: state multi-step T1-button (Beli/Jual via tombol).
// In-memory + timeout; restart bot = state hilang (fail-closed, suruh ulangi).
// Worker bot = 1, tapi tetap pakai mutex biar aman kalau worker dinaikkan.
package bot

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// Step sesi per user.
type Step string

const (
	StepIdle           Step = ""
	StepAwaitBuy       Step = "await_buy_amount"  // sudah pilih coin buy, nunggu nominal
	StepAwaitSell      Step = "await_sell_amount" // sudah pilih coin sell, nunggu nominal atau SEMUA
	StepAdminSpread    Step = "admin_spread"      // sudah pilih coin+side spread, nunggu angka 0-0.1
	StepAdminLimit     Step = "admin_limit"       // sudah pilih key limit, nunggu nilai
	StepAdminBroadcast Step = "admin_broadcast"   // nunggu teks broadcast
)

// SessionTimeout: state hangus setelah ini (suruh tap Beli/Jual lagi).
const SessionTimeout = 120 * time.Second

type Session struct {
	Step      Step
	Coin      string // BNB/ETH/SOL (T1 + admin spread/pause)
	Extra     string // T1: kosong. Admin: side (buy/sell) / key limit
	UpdatedAt time.Time
}

type Sessions struct {
	mu sync.Mutex
	m  map[int64]Session
}

func NewSessions() *Sessions { return &Sessions{m: map[int64]Session{}} }

// Get: ambil state kalau masih fresh; expired = hapus + idle.
func (s *Sessions) Get(id int64) Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.m[id]
	if !ok {
		return Session{Step: StepIdle}
	}
	if time.Since(st.UpdatedAt) > SessionTimeout {
		delete(s.m, id)
		return Session{Step: StepIdle}
	}
	return st
}

// Set: simpan/timpa state user.
func (s *Sessions) Set(id int64, st Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st.UpdatedAt = time.Now()
	s.m[id] = st
}

// Clear: hapus state (selesai / batal / pindah menu).
func (s *Sessions) Clear(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
}

// ParseNominal: "50000" / "50.000" / "Rp50.000" / "50,000" → 50000.
// Tolak huruf, <=0. Pengecekan minimum (min_buy_idr) di caller via settings.
func ParseNominal(raw string) (int64, bool) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	t = strings.ReplaceAll(t, "RP", "")
	t = strings.ReplaceAll(t, ".", "")
	t = strings.ReplaceAll(t, ",", "")
	t = strings.ReplaceAll(t, " ", "")
	if t == "" {
		return 0, false
	}
	for _, c := range t {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// MatchButton: teks pesan → nama tombol (case-insensitive, dengan/tanpa slash).
// Return "": bukan tombol menu.
func MatchButton(text string) string {
	t := strings.ToLower(strings.TrimSpace(text))
	switch t {
	case "harga", "/harga":
		return BtnHarga
	case "saldo", "/saldo":
		return BtnSaldo
	case "beli", "/beli":
		return BtnBeli
	case "jual", "/jual":
		return BtnJual
	case "riwayat", "/riwayat":
		return BtnRiwayat
	case "batal", "/batal":
		return BtnBatal
	case "admin", "/admin":
		return BtnAdmin
	}
	return ""
}
